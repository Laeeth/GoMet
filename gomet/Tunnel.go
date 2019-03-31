package gomet

import (
	"golang.org/x/crypto/ssh"
	"log"
	"net"
	"sync"
	"io"
)

type Tunnel struct {
	client *ssh.Client
}

func NewTunnel(config Config) *Tunnel {

	log.Printf("Opening tunnel")

	var tunnel = Tunnel {}

	if len(config.Tunnel.Nodes) > 0 {

		for _, node := range config.Tunnel.Nodes {
			log.Printf("Connect to node %s", node.Host)
			tunnel.client = connectSshNode(tunnel.client, node.Host, node.Username, node.Password)
		}

		log.Printf("Tunnel opened, remote listen on %s", config.Tunnel.ListenAddr)

		laddr, err := net.ResolveTCPAddr("tcp", config.Tunnel.ListenAddr)
		log.Printf("Addr %s", laddr)

		remote, err := tunnel.client.Listen("tcp", config.Tunnel.ListenAddr)
		if err != nil {
			log.Printf("ERROR %s", err)
		}

		go handleSshConnections(remote, config)
	}
	return &tunnel
}

func (t *Tunnel) Connect(conn net.Conn, addr string) error {
	var remoteConn net.Conn
	var err error

	if t.client != nil {
		remoteConn, err = t.client.Dial("tcp", addr)
	} else {
		remoteConn, err = net.Dial("tcp", addr)
	}

	if err != nil {
		return err
	}

	go handleSshConnection(remoteConn, conn)

	return nil
}


func makeSshConfig(user, password string) (*ssh.ClientConfig, error) {

	config := ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.Password(password),
		},
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			log.Printf("Ignored host %s key %s", hostname, key)
			return nil
		},
	}

	return &config, nil
}

func connectSshNode(client *ssh.Client, host, username, password string) *ssh.Client {

	cfg, err := makeSshConfig(username, password)
	if err != nil {
		log.Printf("ERROR %s", err)
		return nil
	}

	var conn net.Conn
	if client == nil {
		conn, err = net.Dial("tcp", host)
	} else {
		conn, err = client.Dial("tcp", host)
	}

	if err != nil {
		log.Printf("ERROR %s", err)
		return nil
	}

	c, chans, reqs, err := ssh.NewClientConn(conn, host, cfg)
	if err != nil {
		log.Printf("ERROR %s", err)
		return nil
	}
	return ssh.NewClient(c, chans, reqs)
}

func handleSshConnections(remote net.Listener, config Config) {
	for {
		log.Printf("Waiting for remote connection...")
		remoteConn, err := remote.Accept()
		if err != nil {
			log.Printf("ERROR %s", err)
			break
		}

		log.Printf("New connection from %s", remoteConn.RemoteAddr())

		localConn, err := net.Dial("tcp", config.ListenAddr)
		if err != nil {
			log.Printf("ERROR %s", err)
			break
		}

		go handleSshConnection(remoteConn, localConn)
	}
}

func handleSshConnection(remoteConn net.Conn, localConn net.Conn) {

	defer remoteConn.Close()
	defer localConn.Close()

	var wg sync.WaitGroup
	wg.Add(2)

	log.Printf("Handle connection %s to %s", remoteConn.RemoteAddr(), localConn.RemoteAddr())

	go func() {
		io.Copy(localConn, remoteConn)
		log.Printf("Close input")
		localConn.Close()
		wg.Done()
	}()

	go func() {
		io.Copy(remoteConn, localConn)
		log.Printf("Close output")
		remoteConn.Close()
		wg.Done()
	}()

	wg.Wait()

	log.Printf("Connection closed")
}