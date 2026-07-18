package main

import (
	"os/exec"
	"strconv"
	"syscall"
)

func runningAsRoot() bool {
	return false
}

func configureProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP}
}

func terminateProcess(cmd *exec.Cmd) error {
	return exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(cmd.Process.Pid)).Run()
}

func commandShell(command string) (string, []string) {
	return "cmd.exe", []string{"/C", command}
}
