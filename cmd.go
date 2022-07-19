package vproxy

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"syscall"
)

func runCommand(args []string) *exec.Cmd {
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	setProcAttr(cmd)

	fmt.Println("[*] running command:", cmd)
	err := cmd.Start()
	if err != nil {
		log.Fatal("error starting command: ", err)
	}
	return cmd
}

func stopCommand(cmd *exec.Cmd) {

	if cmd == nil {
		return
	}

	fmt.Println("[*] stopping process", cmd.Process.Pid)
	e := cmd.Process.Signal(syscall.SIGTERM)
	if e != nil {
		fmt.Println("error killing child process:", e)
	}
	cmd.Process.Wait()
}
