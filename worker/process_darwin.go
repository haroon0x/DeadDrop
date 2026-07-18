package main

import (
	"os/exec"
	"syscall"
)

func runningAsRoot() bool {
	return syscall.Geteuid() == 0
}

func configureProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func terminateProcess(cmd *exec.Cmd) error {
	return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
}

func commandShell(command string) (string, []string) {
	return "sh", []string{"-c", command}
}
