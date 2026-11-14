//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package process

import (
	"os/exec"
	"syscall"
)

func isStdoutLimitPipeExit(waitErr error) bool {
	exitErr, ok := waitErr.(*exec.ExitError)
	if !ok {
		return false
	}
	status, ok := exitErr.Sys().(syscall.WaitStatus)
	return ok && status.Signaled() && status.Signal() == syscall.SIGPIPE
}
