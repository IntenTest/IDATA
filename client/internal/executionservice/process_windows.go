//go:build windows

package executionservice

import (
	"os"
	"os/exec"
	"strconv"
	"syscall"
)

func configureHiddenProcess(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
}

func terminateProcess(process *os.Process) error {
	command := exec.Command("taskkill.exe", "/PID", strconv.Itoa(process.Pid), "/T", "/F")
	command.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	return command.Run()
}
