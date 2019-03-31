package gomet

import (
	"bufio"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/pem"
	"github.com/ginuerzh/gosocks5"
	"github.com/pkg/errors"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

type Server struct {

	config Config

	tunnel *Tunnel

	sessionIndex int
	sessions map[int]*Session

	routes map[string]*Session

	pubKeyHash string

	wg *sync.WaitGroup

	osCommands map[string]map[string] string

	listener net.Listener
	socks net.Listener

	sessionListeners []SessionListener

	httpMagic string
}

type SessionListener interface {
	NewSession(session *Session)
	CloseSession(session *Session)
}


func NewServer(wg *sync.WaitGroup, config Config) *Server {

	return &Server{
		sessionIndex: 0,
		sessions: make(map[int]*Session),
		routes: make(map[string]*Session),
		wg: wg,
		config: config,
		tunnel: NewTunnel(config),
		httpMagic: randomString(15),
	}
}

func (s *Server) populateOsCommands() {

	s.osCommands = make(map[string]map[string] string)

	s.osCommands["windows"] = make(map[string] string)

	s.osCommands["windows"]["ls"]="dir"
	s.osCommands["windows"]["ps"]="tasklist /V"
	s.osCommands["windows"]["id"]="whoami"
	s.osCommands["windows"]["pwd"]="cd"
	s.osCommands["windows"]["netstat"]="netstat -a"

	for _, os := range strings.Split("android darwin dragonfly freebsd linux nacl netbsd openbsd plan9 solaris", " ") {

		s.osCommands[os] = make(map[string] string)

		s.osCommands[os]["ls"]="ls -la"
		s.osCommands[os]["ps"]="ps -axfe"
		s.osCommands[os]["id"]="id"
		s.osCommands[os]["pwd"]="pwd"
		s.osCommands[os]["netstat"]="netstat -a"
	}
}

func (s *Server) Start() {
	if s.config.Socks.Enable {
		go s.startSocks()
	}

	s.populateOsCommands()

	go s.startListener()
}

func (s *Server) Stop() {
	for _, session := range s.sessions {
		session.Close()
	}
	s.listener.Close()
	s.wg.Done()
}

func (s *Server) handleConnection(conn net.Conn) {

	log.Printf("Connection from %s", conn.RemoteAddr())

	reader := bufio.NewReader(conn)
	line, _, err := reader.ReadLine()
	if err != nil {
		log.Printf("ERROR %s", err)
		conn.Close()
		return
	}

	if stringMatch("CONNECT .* HTTP/1.1", line) {
		s.handleNewSession(conn)
	} else if stringMatch("GET /" + s.httpMagic + "/agent/[^/]*/[^ ]* .*", line) {
		s.handleNewAgent(conn, string(line), reader)
	} else if stringMatch("GET /" + s.httpMagic+ "/[^ ]* .*", line) {
		s.handleNewDownload(conn, string(line))
	} else if stringMatch("POST /" + s.httpMagic + "/[^ ]* .*", line) {
		s.handleNewUpload(conn, string(line), reader)
	} else {
		sendHttp404Error(conn)
	}
}

func (s *Server) handleNewSession(conn net.Conn) {
	s.sessionIndex++
	session := NewSession(s, conn, s.sessionIndex)
	if session != nil {
		s.sessions[session.Id] = session
		for _, listener := range s.sessionListeners {
			listener.NewSession(session)
		}
	}
}



func (s *Server) handleNewDownload(conn net.Conn, request string) {
	log.Println("Download file")

	defer conn.Close()

	expr := regexp.MustCompile("GET /" + s.httpMagic + "/([^ ]*) .*")
	values := expr.FindAllStringSubmatch(request, -1)[0]

	if len(values) == 2 {
		path := values[1]

		fileAbsPath, _ := filepath.Abs("share/" + path)
		downloadAbsPath, _ := filepath.Abs("share/")

		if !strings.Contains(fileAbsPath, downloadAbsPath) {
			log.Printf("ERROR Invalid path")
			sendHttp404Error(conn)
			return
		}

		log.Printf("File request %s", path)

		fileContent, err := ioutil.ReadFile(fileAbsPath)
		if err != nil {
			log.Printf("ERROR %s", err)
			sendHttp404Error(conn)
			return
		}

		conn.Write([]byte("HTTP/1.1 200 OK\r\nContent-Disposition: inline; filename=\"" + path +"\"\r\nContent-Length: " + strconv.Itoa(len(fileContent)) + "\r\n\r\n"))
		conn.Write(fileContent)
	}
}

func (s *Server) handleNewUpload(conn net.Conn, request string, reader *bufio.Reader) {
	log.Println("Upload file")

	defer conn.Close()

	expr := regexp.MustCompile("POST /" + s.httpMagic + "/([^ ]*) .*")
	values := expr.FindAllStringSubmatch(request, -1)[0]

	if len(values) == 2 {
		path := values[1]

		fileAbsPath, _ := filepath.Abs("share/" + path)
		downloadAbsPath, _ := filepath.Abs("share/")

		if !strings.Contains(fileAbsPath, downloadAbsPath) {
			log.Printf("ERROR Invalid path")
			sendHttp404Error(conn)
			return
		}

		log.Printf("File request %s", path)
		headers := readHttpHeaders(reader)

		log.Printf("%s", headers)

		fileSize, err := strconv.ParseInt(headers["content-length"], 10, 64)
		if err != nil {
			log.Printf("ERROR %s", err)
			sendHttp404Error(conn)
			return
		}

		file, err := os.OpenFile(fileAbsPath, os.O_RDWR|os.O_CREATE, 644)
		if err != nil {
			log.Printf("ERROR %s", err)
			sendHttp404Error(conn)
			return
		}

		defer file.Close()

		_, err = io.CopyN(file, reader, fileSize)
		if err != nil {
			log.Printf("ERROR %s", err)
			sendHttp404Error(conn)
			return
		}

		conn.Write([]byte("HTTP/1.1 201 Created\r\n\r\n"))
	}
}

func sendHttp404Error(conn net.Conn) {
	conn.Write([]byte("HTTP/1.1 404 Not Found\r\n\r\n"))
	conn.Close()
}

/* -----------------
   Sessions
  ------------------ */


func (s *Server) GetSession(sessionId int) (*Session, error) {
	if session, ok := s.sessions[sessionId]; ok {
		return session, nil
	} else {
		return nil, errors.New("Invalid session Id")
	}
}

func (s *Server) CloseSession(sessionId int) error {
	if session, ok := s.sessions[sessionId]; ok {
		delete(s.sessions, session.Id)
		session.Close()
		for _, listener := range s.sessionListeners {
			listener.CloseSession(session)
		}
	} else {
		return errors.New("Invalid session Id")
	}
	return nil
}

func (s *Server) RegisterSessionListener(listener SessionListener) {
	s.sessionListeners = append(s.sessionListeners, listener)
}

func (s *Server) UnregisterSessionListener(listener SessionListener) {
	//TODO
}


/* ------------------
   Routing
  ------------------- */


func (s *Server) AddRoute(cidr string, sessionId int) error {
	_, _, err := net.ParseCIDR(cidr)
	if err != nil {
		return errors.New("Invalid IP or range")
	}

	if _, ok := s.sessions[sessionId]; ok {
		s.routes[cidr] = s.sessions[sessionId]
	} else {
		return errors.New("Invalid session Id")
	}
	return nil
}

func (s *Server) DelRoute(cidr string) error {
	if _, ok := s.routes[cidr]; ok {
		delete(s.routes, cidr)
	} else {
		return errors.New("Invalid route")
	}
	return nil
}

func (s *Server) ClearRoutes() {
	for key, _ := range s.routes {
		delete(s.routes, key)
	}
}



/* -----------------------
   Agent
  ------------------------ */


func (s *Server) handleNewAgent(conn net.Conn, request string, reader *bufio.Reader) {
	log.Println("Download agent")

	defer conn.Close()

	expr := regexp.MustCompile("GET /" + s.httpMagic + "/agent/([^/]*)/([^ ]*) .*")
	values := expr.FindAllStringSubmatch(request, -1)[0]

	if len(values) == 3 {
		os := values[1]
		arch := values[2]

		headers := readHttpHeaders(reader)

		log.Printf("Agent request Os:%s Arch:%s", os, arch)
		log.Printf("HTTP headers %s", headers)

		agentContent, err := s.GenerateAgent(os,arch, headers["host"], "", "", "", "", s.pubKeyHash)
		if err != nil {
			log.Printf("ERROR %s", err)
			conn.Write([]byte("HTTP/1.1 500 Server Error\r\n\r\n"))
			conn.Close()
			return
		}

		conn.Write([]byte("HTTP/1.1 200 OK\r\nContent-Disposition: inline; filename=\"agent\"\r\nContent-Length: " + strconv.Itoa(len(agentContent)) + "\r\n\r\n"))
		conn.Write(agentContent)
	}
}


func (s *Server) GenerateAgent(goos string, goarch string, host string, httpProxyHost string, httpsProxyHost string, proxyUsername string, proxyPassword string, pubKeySum string) ([]byte, error) {

	tempDir, err := ioutil.TempDir("", "agent")
	if err != nil {
		return nil, err
	}

	defer os.RemoveAll(tempDir)

	log.Printf("New agent in %s\n", tempDir)

	ldflags := "-X main.connectHost=" + host
	ldflags += " -X main.httpProxyHost=" + httpProxyHost
	ldflags += " -X main.httpsProxyHost=" + httpsProxyHost
	ldflags += " -X main.proxyUsername=" + proxyUsername
	ldflags += " -X main.proxyPassword=" + proxyPassword
	ldflags += " -X main.pubKeySum=" + pubKeySum

	usr, err := user.Current()
	if err != nil {
		log.Printf("ERROR %s", err)
		return nil, err
	}

	cmd := exec.Command("go", "build", "-i", "-o", tempDir + "/agent", "-pkgdir", tempDir, "-ldflags", ldflags)
	cmd.Env = []string {"GOOS=" + goos,"GOARCH=" + goarch, "GOPATH=" + usr.HomeDir + "/go", "PATH=" + os.Getenv("PATH"), "GOCACHE=" + tempDir}
	cmd.Dir = "./agent"
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Start()

	log.Println("Building agent...")
	cmd.Wait()

	agentContent, err := ioutil.ReadFile(tempDir + "/agent")

	if err == nil {
		log.Println("Agent build success")
	} else {
		log.Println("ERROR Failed to build agent")
	}

	return agentContent, err
}

/* -------------------
   Socks
  -------------------- */

func (s *Server) getSessionRoute(ip string) *Session {

	ipAddr := net.ParseIP(ip)
	if ipAddr == nil {
		log.Println("Invalid ip %s", ip)
	}

	for cidr, session := range s.routes {
		_, ipnet, _ := net.ParseCIDR(cidr)

		if ipnet.Contains(ipAddr) {
			return session
		}
	}

	return nil
}

func (s *Server) startSocks() {

	var err error

	log.Println("Starting socks")

	s.socks, err = net.Listen("tcp", s.config.Socks.Addr)
	if err != nil {
		log.Printf("ERROR %s", err)
		return
	}

	for {
		log.Println("Waiting for connection...")
		conn, err := s.socks.Accept()
		if err != nil {
			log.Printf("ERROR %s", err)
			break
		}
		log.Println("New socks client")

		methods, err := gosocks5.ReadMethods(conn)
		if err != nil {
			log.Printf("ERROR %s", err)
			conn.Close()
			continue
		}
		log.Printf("Methods %s", methods)

		err = gosocks5.WriteMethod(gosocks5.MethodNoAuth, conn)
		if err != nil {
			log.Printf("ERROR %s", err)
			conn.Close()
			continue
		}

		log.Printf("Read socks request")

		req, err := gosocks5.ReadRequest(conn)
		if err != nil {
			log.Printf("ERROR %s", err)
			conn.Close()
			continue
		}

		log.Printf("Request %s", req)

		rep := gosocks5.NewReply(gosocks5.Succeeded, nil)
		if err := rep.Write(conn); err != nil {
			log.Printf("ERROR %s", err)
			conn.Close()
			continue
		}

		session := s.getSessionRoute(req.Addr.Host)
		if session == nil {
			log.Printf("No route to host %s, using tunnel", req.Addr.Host)

			err = s.tunnel.Connect(conn, req.Addr.String())
			if err != nil {
				log.Printf("ERROR %s", err)
				conn.Close()
			}

			continue
		}

		session.ConnectToRemote(conn, req.Addr.String())
	}
}

/* ----------------
   Listener
  ----------------- */

func (s *Server) startListener() {

	log.Println("Starting listener")

	cert, err := tls.LoadX509KeyPair("config/server.crt", "config/server.key")
	if err != nil {
		log.Printf("ERROR %s", err)
		return
	}

	pemBytes, err := ioutil.ReadFile("config/server.pub")
	if err != nil {
		log.Printf("ERROR %s", err)
		return
	}

	block, _ := pem.Decode(pemBytes)

	sha256 := sha256.New()
	sha256.Write(block.Bytes)
	s.pubKeyHash = hex.EncodeToString(sha256.Sum(nil))
	log.Printf("Public key sum %s", s.pubKeyHash)

	config := tls.Config{Certificates: []tls.Certificate{cert}}
	config.Rand = rand.Reader

	s.listener, err = tls.Listen("tcp", s.config.ListenAddr, &config)
	if err != nil {
		log.Printf("ERROR %s", err)
		return
	}

	for {
		log.Println("Waiting for connection...")
		conn, err := s.listener.Accept()
		if err != nil {
			log.Printf("ERROR %s", err)
			break
		}
		go s.handleConnection(conn)
	}
}