//go:build aix || dragonfly || freebsd || netbsd || openbsd || solaris

package process

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"time"
)

type commandContainment struct {
	cmd *exec.Cmd
}

func explicitContainmentSupervision() bool { return true }

func duplexContainmentPreflight() error {
	return fmt.Errorf("safe duplex process containment unavailable on this platform; manual activation required")
}

func naturalExitNeedsContainmentCleanup() bool { return false }

func plannedTerminationExitExpected(error) bool { return false }

func newCommandContainment(cmd *exec.Cmd) (*commandContainment, error) {
	return nil, fmt.Errorf("process execution is unsupported on %s", runtime.GOOS)
}

func newOrdinaryCommandContainment(cmd *exec.Cmd, _ time.Duration) (*commandContainment, error) {
	return newCommandContainment(cmd)
}

func newDuplexCommandContainment(cmd *exec.Cmd) (*commandContainment, error) {
	return nil, duplexContainmentPreflight()
}

func (*commandContainment) attach(*exec.Cmd) error { return nil }

func (*commandContainment) limitToProcessGroup() {}

func (*commandContainment) terminate() terminationResult { return terminationResult{} }

func (*commandContainment) exited() (bool, error) { return false, nil }

func (*commandContainment) settleNaturalExit(context.Context, time.Duration) (bool, error) {
	return false, nil
}

func (*commandContainment) close() error { return nil }
