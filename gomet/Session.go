package gomet

import (
	"bufio"
	"github.com/xtaci/smux"
	"log"
	"net"
	"os"
	"strings"
	"time"
)


type Registry struct {
	streams map[uint32]*smux.Stream
}

func NewRegistry() Registry {
	return  Registry{
		streams: make(map[uint32]*smux.Stream),
	}
}

func (r *Registry) Register(stream *smux.Stream) {
	r.streams[stream.ID()] = stream
	log.Printf("Stream %d registered", stream.ID())
}

func (r *Registry) Unregister(stream *smux.Stream) {
	delete(r.streams, stream.ID())
	log.Printf("Stream %d unregistered", stream.ID())
}

func (r *Registry) Close() {
	for _, stream := range r.streams {
		log.Printf("Closing stream %d", stream.ID())
		stream.Close()
	}
}


type LogWriter struct {
	Logger *log.Logger
}

func (t *LogWriter) Write(p []byte) (n int, err error) {
	t.Logger.Print(string(p))
	return len(p), nil
}

func (t *LogWriter) WriteString(s string) (n int, err error) {
	t.Logger.Print(s)
	return len(s), nil
}


type Session struct {
	Id int `json:"id"`
	Os       string `json:"os"`
	Arch     string `json:"arch"`
	Hostname string `json:"hostname"`
	Address  string `json:"address"`

	jobIndex int
	jobs map[int]*Command

	registry Registry

	server *Server
	session *smux.Session
	commandStream *smux.Stream
	logWriter *LogWriter
}

func NewSession(server *Server, conn net.Conn, id int) *Session {

	log.Printf("Handle a new session")

	var err error

	var s = Session{
		Id:       id,
		server:   server,
		jobIndex: 0,
		jobs:     make(map[int]*Command),
		registry: NewRegistry(),
	}

	s.session, err = smux.Client(conn, nil)
	if err != nil {
		log.Printf("ERROR %s", err)
		return nil
	}

	s.commandStream, err = s.session.OpenStream()
	if err != nil {
		log.Printf("ERROR %s", err)
		return nil
	}

	log.Printf("Command stream opened")

	reader := bufio.NewReader(s.commandStream)
	systemInfo, _, err := reader.ReadLine()
	if err != nil {
		s.commandStream.Close()
		log.Printf("ERROR %s", err)
		return nil
	}

	array := strings.Split(string(systemInfo), "|")
	if len(array) != 3 {
		s.commandStream.Close()
		log.Printf("Invalid system info format")
		return nil
	}

	s.Os = array[0]
	s.Arch = array[1]
	s.Hostname = array[2]
	s.Address = conn.RemoteAddr().String()

	current_time := time.Now().Local()
	file, err := os.OpenFile("logs/" + current_time.Format("2006-01-02") + "_" + s.Hostname + ".log", os.O_RDWR | os.O_CREATE | os.O_APPEND, 0666)
	s.logWriter = &LogWriter{
		Logger: log.New(file, "", log.LstdFlags),
	}

	return &s
}


func (s *Session) RunCommand(command Command) {

	s.logWriter.WriteString(command.String())

	if command.GetRemoteCommand() != "" {
		s.commandStream.Write([]byte(command.GetRemoteCommand()))
	}
	if command.IsJob() {
		s.runBackgroundCommand(command)
	} else {
		s.runInteractiveCommand(command)
	}
}

func (s *Session) ConnectToRemote(conn net.Conn, remoteAddress string) {

	_, err := s.commandStream.Write([]byte("5\n" + remoteAddress + "\n"))
	if err != nil {
		log.Printf("ERROR %s", err)
		conn.Close()
		return
	}

	stream, err := s.session.AcceptStream()
	if err != nil {
		log.Printf("ERROR %s", err)
		conn.Close()
		return
	}

	go handleConnection(conn, stream, &s.registry)
}


func (s *Session) DownloadFile(remoteFilename string, localFilename string) {

	file, err := os.OpenFile(localFilename, os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		log.Printf("ERROR %s", err)
		return
	}

	defer file.Close()

	s.RunCommand(&Download{
		writer: file,
		remoteFilename: remoteFilename,
	})
}

func (s *Session) UploadFile(localFilename string, remoteFilename string) {

	file, err := os.OpenFile(localFilename, os.O_RDONLY, 0755)
	if err != nil {
		log.Printf("ERROR %s", err)
		return
	}

	defer file.Close()

	s.RunCommand(&Upload{
		reader: file,
		remoteFilename: remoteFilename,
	})
}

func (s *Session) Close() {

	for _, job := range s.jobs {
		(*job).Stop()
	}

	s.commandStream.Write([]byte("6\n"))
	s.commandStream.Close()
	s.session.Close()
}

func (s *Session) String() string {
	return s.Hostname + " - " + s.Address + " - " + s.Os + "/" + s.Arch
}


/* Private functions */

func (s *Session) runBackgroundCommand(command Command) {
	s.jobs[s.newJobId()] = &command
	go command.Start(s.session, &s.registry, s.logWriter)
}

func (s *Session) runInteractiveCommand(command Command) {
	command.Start(s.session, &s.registry, s.logWriter)
	command.Stop()
}

func (s *Session) newJobId() int {
	s.jobIndex++
	return s.jobIndex
}
