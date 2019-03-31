package gomet

import (
	"bufio"
	"fmt"
	"github.com/xtaci/smux"
	"io"
	"log"
	"net"
	"sync"
)

type Command interface {
	IsJob() bool
	GetRemoteCommand() string
	Start(session *smux.Session, registry *Registry, logger *LogWriter)
	Stop()
	String() string
}


// Execute command
// ---------------

type Execute struct {
	writer io.Writer
	command string
	stream *smux.Stream
}

func (e *Execute) GetRemoteCommand() string {
	return "0\n" + e.command + "\n"
}

func (e *Execute) Start(session *smux.Session, registry *Registry, logger *LogWriter) {

	e.stream = acceptNewStream(session)

	if e.stream == nil {
		return
	}

	defer e.stream.Close()

	log.Printf("Execute command %s", e.command)

	io.Copy(io.MultiWriter(e.writer, logger), e.stream)

	log.Println("Done")
}

func (e *Execute) Stop() {
	if e.stream != nil {
		e.stream.Close()
	}
}

func (e *Execute) IsJob() bool {
	return false
}

func (e *Execute) String() string {
	return "Execute " + e.command
}


// Download command
// ----------------

type Download struct {
	writer io.Writer
	remoteFilename string
	stream *smux.Stream
}

func (d *Download) GetRemoteCommand() string {
	return "1\n" + d.remoteFilename + "\n"
}

func (d *Download) Start(session *smux.Session, registry *Registry, logger *LogWriter) {

	d.stream = acceptNewStream(session)

	if d.stream == nil {
		return
	}

	defer d.stream.Close()

	log.Printf("Download file %s", d.remoteFilename)

	io.Copy(io.MultiWriter(d.writer, logger), d.stream)

	log.Println("Done")
}

func (d *Download) Stop() {
	if d.stream != nil {
		d.stream.Close()
	}
}

func (e *Download) IsJob() bool {
	return false
}

func (e *Download) String() string {
	return "Downloading " + e.remoteFilename
}


// Upload command
// --------------

type Upload struct {
	reader io.Reader
	remoteFilename string
	stream *smux.Stream
}

func (u *Upload) GetRemoteCommand() string {
	return "2\n" + u.remoteFilename + "\n"
}

func (u *Upload) Start(session *smux.Session, registry *Registry, logger *LogWriter) {

	u.stream = acceptNewStream(session)

	if u.stream == nil {
		return
	}

	defer u.stream.Close()

	log.Printf("Upload file %s", u.remoteFilename)

	io.Copy(io.MultiWriter(u.stream, logger), u.reader)

	log.Println("Done")
}

func (u *Upload) Stop() {
	if u.stream != nil {
		u.stream.Close()
	}
}

func (e *Upload) IsJob() bool {
	return false
}

func (e *Upload) String() string {
	return "Uploading " + e.remoteFilename
}


// Shell command
// -------------

type Shell struct {
	writer io.Writer
	reader io.Reader
	stream *smux.Stream
}

func (s *Shell) GetRemoteCommand() string {
	return "3\n"
}

func (s *Shell) Start(session *smux.Session, registry *Registry, logger *LogWriter) {

	s.stream = acceptNewStream(session)
	if s.stream == nil {
		return
	}

	defer s.stream.Close()

	log.Printf("New shell")

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		io.Copy(io.MultiWriter(s.stream, logger), s.reader)
		wg.Done()
	}()

	go func() {
		io.Copy(io.MultiWriter(s.writer, logger), s.stream)
		wg.Done()
		fmt.Printf("Press \"Enter\" to close")
	}()

	s.stream.Write([]byte("\n"))

	wg.Wait()

	log.Println("Done")
}

func (s *Shell) Stop() {
	if s.stream != nil {
		s.stream.Close()
	}
}

func (e *Shell) IsJob() bool {
	return false
}

func (e *Shell) String() string {
	return "Interactive shell"
}


// Listen command
// --------------

type Listen struct {
	localAddress string
	remoteAddress string
	stream *smux.Stream
}

func (l *Listen) GetRemoteCommand() string {
	return "4\n" + l.remoteAddress + "\n"
}

func (l *Listen) Start(session *smux.Session, registry *Registry, logger *LogWriter) {

	l.stream = acceptNewStream(session)
	if l.stream == nil {
		return
	}

	go func() {

		defer l.stream.Close()

		reader := bufio.NewReader(l.stream)

		for {
			log.Println("Wait for remote connection...")
			_, _, err := reader.ReadLine()
			if err != nil {
				log.Printf("ERROR %s", err)
				break
			}

			_, err = l.stream.Write([]byte("OK\n"))
			if err != nil {
				log.Printf("ERROR %s", err)
				break
			}

			log.Println("Open stream")
			listenStream, err := session.OpenStream()
			if err != nil {
				log.Printf("ERROR %s", err)
				break
			}

			log.Println("Connect to local Address")
			conn, err := net.Dial("tcp", l.localAddress)
			if err != nil {
				log.Printf("ERROR %s", err)
				listenStream.Close()
				continue
			}

			go handleConnection(conn, listenStream, registry)
		}
	}()
}

func (l *Listen) Stop() {
	if l.stream != nil {
		log.Println("Closing command stream")
		l.stream.Close()
	}
}

func (l *Listen) IsJob() bool {
	return true
}

func (l *Listen) String() string {
	return "Remote " + l.remoteAddress + " to local " + l.localAddress
}


// Connect command
// --------------

type Connect struct {
	localAddress string
	remoteAddress string
	listen net.Listener
	session *Session
}

func (l *Connect) GetRemoteCommand() string {
	return ""
}

func (l *Connect) Start(session *smux.Session, registry *Registry, logger *LogWriter) {

	go func() {
		var err error
		l.listen, err = net.Listen("tcp", l.localAddress)
		if err != nil {
			log.Printf("ERROR %s", err)
			return
		}

		defer l.listen.Close()

		for {
			log.Printf("Waiting for connection on %s...\n", l.localAddress)
			conn, err := l.listen.Accept()
			if err != nil {
				log.Printf("ERROR %s", err)
				break
			}

			l.session.ConnectToRemote(conn, l.remoteAddress)
		}
	}()
}

func (l *Connect) Stop() {
	if l.listen != nil {
		l.listen.Close()
	}
}

func (l *Connect) IsJob() bool {
	return true
}

func (l *Connect) String() string {
	return "Local " + l.localAddress + " to remote " + l.remoteAddress
}
