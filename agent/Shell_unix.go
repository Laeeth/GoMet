// +build linux darwin freebsd netbsd openbsd

package main

import (
	"github.com/kr/pty"
	"io"
	"os/exec"
	"syscall"
)


func (a *Agent) execute(command string) {

	stream := openNewStream(a.session)

	if  stream == nil {
		return
	}

	defer stream.Close()

	cmd := exec.Command("sh", "-c", command)
	cmd.Stdout = stream
	cmd.Stderr = stream

	cmd.Run()
}


func (a *Agent) shell() {

	stream := openNewStream(a.session)

	if stream == nil {
		return
	}

	defer stream.Close()

	file, tty, err := pty.Open()
	if err != nil {
		return
	}

	defer file.Close()
	defer tty.Close()

	command := exec.Command("bash")
	command.Env = []string{"TERM=xterm"}
	command.Stdout = tty
	command.Stdin = tty
	command.Stderr = tty
	command.SysProcAttr = &syscall.SysProcAttr{
		Setctty: true,
		Setsid:  true,
	}

	err = command.Start()
	if err != nil {
		return
	}

	go func() {
		io.Copy(stream, file)
	}()

	go func() {
		io.Copy(file, stream)
	}()

	command.Wait()
}