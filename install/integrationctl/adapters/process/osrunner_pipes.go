package process

import (
	"io"
	"os"
	"os/exec"
	"time"
)

type duplexPipeFactory struct {
	output      func() (*os.File, *os.File, error)
	input       func(*exec.Cmd) (io.WriteCloser, error)
	containment func(*exec.Cmd) (duplexCommandContainment, error)
	// Finite commands may have an ordinary supervisor awaiting the natural
	// exit of native helpers. Strict duplex protocols retain their short grace.
	exitGrace time.Duration
	closure   *duplexClosureProof
}

func (pipes duplexPipeFactory) exitTimeout() time.Duration {
	if pipes.exitGrace > 0 {
		return pipes.exitGrace
	}
	return duplexExitGrace
}

var defaultDuplexPipeFactory = duplexPipeFactory{
	output: os.Pipe,
	input:  func(cmd *exec.Cmd) (io.WriteCloser, error) { return cmd.StdinPipe() },
	containment: func(cmd *exec.Cmd) (duplexCommandContainment, error) {
		return newDuplexCommandContainment(cmd)
	},
}
