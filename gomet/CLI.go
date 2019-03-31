package gomet

import (
	"github.com/abiosoft/ishell"
	"io/ioutil"
	"log"
	"os"
	"strconv"
)

type CLI struct {
	shell *ishell.Shell
	server *Server
	currentSession *Session
}

func NewCLI(server *Server) *CLI {
	return &CLI{
		shell:ishell.New(),
		server: server,
	}
}

func (c *CLI) Start() {

	c.shell.Println("\n")
	c.shell.Println("	  ____       __  __      _   ")
	c.shell.Println("	 / ___| ___ |  \\/  | ___| |_")
	c.shell.Println("	| |  _ / _ \\| |\\/| |/ _ \\ __|")
	c.shell.Println("	| |_| | (_) | |  | |  __/ |_")
	c.shell.Println("	 \\____|\\___/|_|  |_|\\___|\\__|")
	c.shell.Println("                                      by Mimah\n\n")

	c.server.RegisterSessionListener(c)
	c.registerServerCommands()
}

func (t *CLI) registerServerCommands() {

	t.shell.Close()
	t.shell = ishell.New()
	t.shell.SetPrompt("server > ")

	// remove default exit command
	t.shell.DeleteCmd("exit")

	t.shell.AddCmd(&ishell.Cmd{
		Name: "exit",
		Help: "Exit",
		Func: t.exit,
	})

	// Sessions
	sessionsCmd := ishell.Cmd{
		Name: "sessions",
		Help: "List sessions",
		Func: t.listSessions,
	}

	t.shell.AddCmd(&sessionsCmd)

	sessionsCmd.AddCmd(&ishell.Cmd{
		Name: "open",
		Help: "Open session",
		Func: t.openSession,
	})

	sessionsCmd.AddCmd(&ishell.Cmd{
		Name: "close",
		Help: "Close session",
		Func: t.closeSession,
	})

	// Routes
	routesCmd := ishell.Cmd{
		Name: "routes",
		Help: "List routes",
		Func: t.listRoutes,
	}

	t.shell.AddCmd(&routesCmd)

	routesCmd.AddCmd(&ishell.Cmd{
		Name: "add",
		Help: "Add a route",
		Func: t.addRoute,
	})

	routesCmd.AddCmd(&ishell.Cmd{
		Name: "del",
		Help: "Delete a route",
		Func: t.delRoute,
	})

	routesCmd.AddCmd(&ishell.Cmd{
		Name: "clear",
		Help: "Clear routes",
		Func: t.clearRoutes,
	})

	// Agent
	t.shell.AddCmd(&ishell.Cmd{
		Name: "generate",
		Help: "Generate an agent",
		Func: t.generateAgent,
	})

	t.shell.AddCmd(&ishell.Cmd{
		Name: "info",
		Help: "Print server information",
		Func: t.printInfo,
	})

	t.shell.Start()
}


func (t *CLI) registerSessionCommands(sessionId int) {

	t.shell.Close()
	t.shell = ishell.New()
	t.shell.SetPrompt("session " + strconv.Itoa(sessionId) + " > ")

	t.shell.DeleteCmd("exit")

	t.shell.AddCmd(&ishell.Cmd{
		Name: "close",
		Help: "Close session",
		Func: t.closeCurrentSession,
	})

	t.shell.AddCmd(&ishell.Cmd{
		Name: "exit",
		Help: "Back to server",
		Func: t.suspendCurrentSession,
	})

	jobCmd := ishell.Cmd{
		Name: "jobs",
		Help: "List jobs",
		Func: t.listJobs,
	}

	t.shell.AddCmd(&jobCmd)

	jobCmd.AddCmd(&ishell.Cmd{
		Name: "kill",
		Help: "Kill a job",
		Func: t.killJob,
	})

	streamCmd := ishell.Cmd{
		Name: "streams",
		Help: "List streams",
		Func: t.listStreams,
	}
	t.shell.AddCmd(&streamCmd)

	streamCmd.AddCmd(&ishell.Cmd{
		Name: "kill",
		Help: "Kill a stream",
		Func: t.killStream,
	})

	t.shell.AddCmd(&ishell.Cmd{
		Name: "execute",
		Help: "Execute a command",
		Func: func(c *ishell.Context) {
			t.runCommand(&Execute{
				writer: os.Stdout,
				command: readParameter(c, "Command: "),
			})
		},
	})

	t.shell.AddCmd(&ishell.Cmd{
		Name: "upload",
		Help: "Upload a file",
		Func: func(c *ishell.Context) {
			t.currentSession.UploadFile(readParameter(c, "Local file: "), readParameter(c, "Remote file: "))
		},
	})

	t.shell.AddCmd(&ishell.Cmd{
		Name: "download",
		Help: "Download a file",
		Func: func(c *ishell.Context) {
			t.currentSession.DownloadFile(readParameter(c, "Remote file: "), readParameter(c, "Local file: "))
		},
	})

	t.shell.AddCmd(&ishell.Cmd{
		Name: "shell",
		Help: "Interactive remote shell",
		Func: func(c *ishell.Context) {
			t.runCommand(&Shell{
				writer: os.Stdout,
				reader: os.Stdin,
			})
		},
	})

	t.shell.AddCmd(&ishell.Cmd{
		Name: "ls",
		Help: "List files",
		Func: func(c *ishell.Context) {
			t.runCommand(&Execute{
				writer: os.Stdout,
				command: t.server.osCommands[t.currentSession.Os]["ls"],
			})
		},
	})

	t.shell.AddCmd(&ishell.Cmd{
		Name: "getuid",
		Help: "Get user Id",
		Func: func(c *ishell.Context) {
			t.runCommand(&Execute{
				writer: os.Stdout,
				command: t.server.osCommands[t.currentSession.Os]["id"],
			})
		},
	})

	t.shell.AddCmd(&ishell.Cmd{
		Name: "pwd",
		Help: "Get current directory",
		Func: func(c *ishell.Context) {
			t.runCommand(&Execute{
				writer: os.Stdout,
				command: t.server.osCommands[t.currentSession.Os]["pwd"],
			})
		},
	})

	t.shell.AddCmd(&ishell.Cmd{
		Name: "ps",
		Help: "List processes",
		Func: func(c *ishell.Context) {
			t.runCommand(&Execute{
				writer: os.Stdout,
				command: t.server.osCommands[t.currentSession.Os]["ps"],
			})
		},
	})

	t.shell.AddCmd(&ishell.Cmd{
		Name: "netstat",
		Help: "List connections",
		Func: func(c *ishell.Context) {
			t.runCommand(&Execute{
				writer: os.Stdout,
				command: t.server.osCommands[t.currentSession.Os]["netstat"],
			})
		},
	})

	t.shell.AddCmd(&ishell.Cmd{
		Name: "cat",
		Help: "Print a file",
		Func: func(c *ishell.Context) {
			t.runCommand(&Download{
				writer: os.Stdout,
				remoteFilename: readParameter(c, "Remote file: "),
			})
		},
	})

	t.shell.AddCmd(&ishell.Cmd{
		Name: "listen",
		Help: "Connect a remote port to a local Address",
		Func: func(c *ishell.Context) {
			t.runCommand(&Listen{
				localAddress: readParameter(c,"Local Address: "),
				remoteAddress: readParameter(c, "Remote Address: "),
			})
		},
	})

	t.shell.AddCmd(&ishell.Cmd{
		Name: "connect",
		Help: "Connect a local port to a remote Address",
		Func: func(c *ishell.Context) {
			t.runCommand(&Connect{
				localAddress:  readParameter(c,"Local Address: "),
				remoteAddress: readParameter(c, "Remote Address: "),
				session:       t.currentSession,
			})
		},
	})

	t.shell.AddCmd(&ishell.Cmd{
		Name: "relay",
		Help: "Relay listen",
		Func: func(c *ishell.Context) {
			t.runCommand(&Listen{
				localAddress:  t.server.config.ListenAddr,
				remoteAddress: readParameter(c, "Remote Address: "),
			})
		},
	})

	t.shell.Start()
}

func (t* CLI) exit(c *ishell.Context) {
	t.server.Stop()
	c.Stop()
}

func (t *CLI) listSessions(c *ishell.Context) {

	if len(t.server.sessions) == 0 {
		c.Println("No sessions")
		return
	}

	c.Println("Sessions:")
	for key, session := range t.server.sessions {
		c.Printf("%5d - %s\n", key, session.String())
	}
}

func (t *CLI) openSession(c *ishell.Context) {
	id, err := strconv.Atoi(c.Args[0])
	if err != nil {
		c.Print("Invalid session Id")
		return
	}

	session, err := t.server.GetSession(id)
	if err != nil {
		c.Print(err)
	} else {
		t.currentSession = session
		t.registerSessionCommands(session.Id)
	}
}

func (t *CLI) closeSession(c *ishell.Context) {
	id, err := strconv.Atoi(c.Args[0])
	if err != nil {
		c.Print("Invalid session Id")
		return
	}

	err = t.server.CloseSession(id)
	if err != nil {
		c.Print(err)
	}
}



func (t *CLI) listRoutes(c *ishell.Context) {
	if len(t.server.routes) == 0 {
		c.Println("No routes")
		return
	}

	c.Println("Routes:")
	for key, session := range t.server.routes {
		c.Printf("%s - %s\n", key, session.String())
	}
}

func (t *CLI) addRoute(c *ishell.Context) {
	if len(c.Args) != 2 {
		c.Println("Usage: routes add <range> <sessionId>")
		return
	}

	id, err := strconv.Atoi(c.Args[1])
	if err != nil {
		c.Println("Invalid session Id")
		return
	}

	err = t.server.AddRoute(c.Args[0], id)

	if err!= nil {
		c.Println(err)
	}
}

func (t *CLI) delRoute(c *ishell.Context) {
	if len(c.Args) != 1 {
		c.Println("Usage: routes del <range>")
		return
	}

	err := t.server.DelRoute(c.Args[0])

	if err != nil {
		c.Println(err)
	}
}

func (t *CLI) clearRoutes(c *ishell.Context) {
	t.server.ClearRoutes()
}

func (t *CLI) suspendCurrentSession(c *ishell.Context) {
	c.Printf("Session %d suspended\n", t.currentSession.Id)
	t.currentSession = nil
	t.registerServerCommands()
}

func (t *CLI) closeCurrentSession(c *ishell.Context) {
	err := t.server.CloseSession(t.currentSession.Id)
	if err != nil {
		c.Print(err)
	}
}

func (t *CLI) listJobs(c *ishell.Context) {
	for key, job := range t.currentSession.jobs {
		c.Printf("%5d - %s\n", key, *job)
	}
}

func (t *CLI) killJob(c *ishell.Context) {
	id, err := strconv.Atoi(c.Args[0])
	if err != nil {
		c.Printf("Invalid job Id")
		return
	}
	if job, ok := t.currentSession.jobs[id]; ok {
		(*job).Stop()
		delete(t.currentSession.jobs, id)
		c.Printf("Job %d killed\n", id)
	} else {
		c.Println("Invalid job Id")
	}
}

func (t *CLI) listStreams(c *ishell.Context) {
	for key, stream := range t.currentSession.registry.streams {
		c.Printf("%5d - %s <-> %s\n", key, stream.LocalAddr(), stream.RemoteAddr())
	}
}

func (t *CLI) killStream(c *ishell.Context) {
	id, err := strconv.Atoi(c.Args[0])
	if err != nil {
		c.Printf("Invalid stream Id")
		return
	}
	if stream, ok := t.currentSession.registry.streams[uint32(id)]; ok {
		stream.Close()
		delete(t.currentSession.jobs, id)
		c.Printf("Stream %d killed\n", id)
	} else {
		c.Println("Invalid stream Id")
	}
}

func (t *CLI) runCommand(command Command) {
	t.currentSession.RunCommand(command)
}

func (t *CLI) generateAgent(c *ishell.Context) {

	os := readParameter(c, "OS: ")
	arch := readParameter(c, "Arch: ")
	host := readParameter(c, "Host: ")

	agentContent, err := t.server.GenerateAgent(os, arch, host, readParameter(c, "HTTP proxy: "), readParameter(c, "HTTPS proxy: "), readParameter(c, "Proxy username: "), readParameter(c, "Proxy password: "), t.server.pubKeyHash)
	if err != nil {
		log.Printf("ERROR %s", err)
		return
	}

	filename := randomString(15)
	err = ioutil.WriteFile("./share/" + filename, agentContent, 0644)
	if err != nil {
		log.Printf("ERROR %s", err)
		return
	}
	c.Printf("Generated agent URL: https://" + host + "/" + t.server.httpMagic + "/" + filename)
}

func (t *CLI) printInfo(c *ishell.Context) {
	c.Printf("Local listener: %s\n", t.server.config.ListenAddr)
	if len(t.server.config.Tunnel.Nodes) > 0 {
		c.Printf("Tunnel listener %s\n", t.server.config.Tunnel.ListenAddr)
	}
	if t.server.config.Socks.Enable {
		c.Printf("Socks listener: %s\n", t.server.config.Socks.Addr)
	}
	if t.server.config.Api.Enable {
		c.Printf("API listener: %s\n", t.server.config.Api.Addr)
	}
	c.Printf("HTTP magic: %s\n", t.server.httpMagic)
}

/* Session listener */

func (t *CLI) NewSession(session *Session) {
	t.shell.Printf("New session %d - %s\n", session.Id, session.String())
}

func (t *CLI) CloseSession(session *Session) {
	if session == t.currentSession {
		t.currentSession = nil
		t.registerServerCommands()
	}
	t.shell.Printf("Session %d - %s closed\n", session.Id, session.String())
}