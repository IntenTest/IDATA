//go:build !windows

package executionservice

import (
	"os"
	"os/exec"
)

func configureHiddenProcess(_ *exec.Cmd) {}

func terminateProcess(process *os.Process) error {
	return process.Kill()
}
