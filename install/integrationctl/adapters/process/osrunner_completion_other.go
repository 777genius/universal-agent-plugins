//go:build !linux

package process

import (
	"os/exec"
	"time"
)

func newCompletionCommandContainment(cmd *exec.Cmd, grace time.Duration) (*commandContainment, error) {
	return newOrdinaryCommandContainment(cmd, grace)
}
