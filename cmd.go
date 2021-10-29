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
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

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

	err := syscall.Kill(-cmd.Process.Pid, syscall.Signal(0))
	if err != nil && err.Error() == "operation not permitted" {
		// process no longer running, nothing to do here
		return
	}

	fmt.Println("[*] stopping process", cmd.Process.Pid)
	e := syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
	if e != nil {
		fmt.Println("error killing child process:", e)
	}
	cmd.Process.Wait()
}
