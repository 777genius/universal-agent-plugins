//go:build darwin

package process

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

// Darwin can safely supervise a trusted one-shot command's process group while
// its unreaped leader keeps the numeric PGID from reuse. It cannot safely
// signal arbitrary start-time-checked descendant PIDs, so duplex Kiro
// verification (which must contain possible setsid descendants) fails closed.
type commandContainment struct {
	cmd              *exec.Cmd
	kqueue           int
	exitObservation  latchedExitObservation
	kevent           func(int, []unix.Kevent_t, []unix.Kevent_t, *unix.Timespec) (int, error)
	liveGroupMembers func(int) (int, error)
	killGroup        func(int, syscall.Signal) error
}

func explicitContainmentSupervision() bool { return true }

func duplexContainmentPreflight() error {
	return fmt.Errorf("safe duplex process containment unavailable on Darwin; manual activation required")
}

func naturalExitNeedsContainmentCleanup() bool { return true }

func plannedTerminationExitExpected(err error) bool {
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		return false
	}
	status, ok := exitErr.Sys().(syscall.WaitStatus)
	return ok && status.Signaled() && status.Signal() == syscall.SIGKILL
}

func newCommandContainment(cmd *exec.Cmd) (*commandContainment, error) {
	kqueue, err := unix.Kqueue()
	if err != nil {
		return nil, err
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	return &commandContainment{
		cmd:              cmd,
		kqueue:           kqueue,
		liveGroupMembers: darwinLiveGroupMembers,
		killGroup:        syscall.Kill,
	}, nil
}

func newOrdinaryCommandContainment(cmd *exec.Cmd, _ time.Duration) (*commandContainment, error) {
	return newCommandContainment(cmd)
}

func newDuplexCommandContainment(cmd *exec.Cmd) (*commandContainment, error) {
	return nil, duplexContainmentPreflight()
}

func (containment *commandContainment) attach(cmd *exec.Cmd) error {
	change := unix.Kevent_t{}
	unix.SetKevent(&change, cmd.Process.Pid, unix.EVFILT_PROC, unix.EV_ADD|unix.EV_ENABLE|unix.EV_CLEAR)
	change.Fflags = unix.NOTE_EXIT
	var err error
	for {
		_, err = containment.callKevent([]unix.Kevent_t{change}, nil, nil)
		if !errors.Is(err, syscall.EINTR) {
			break
		}
	}
	if errors.Is(err, syscall.ESRCH) {
		containment.exitObservation.latch()
		return nil
	}
	return err
}

func (*commandContainment) limitToProcessGroup() {}

func (containment *commandContainment) terminate() terminationResult {
	if containment.cmd.Process == nil {
		return terminationResult{err: os.ErrProcessDone}
	}
	members, inspectErr := containment.groupMembers(containment.cmd.Process.Pid)
	stopped, observeErr := containment.exited()
	if inspectErr == nil && observeErr == nil && stopped && members == 0 {
		return terminationResult{leaderStopped: true}
	}
	killErr := containment.signalGroup(-containment.cmd.Process.Pid, syscall.SIGKILL)
	if errors.Is(killErr, syscall.ESRCH) {
		killErr = nil
	}
	deadline := time.Now().Add(time.Second)
	for {
		stopped, observeErr = containment.exited()
		remaining, groupErr := containment.groupMembers(containment.cmd.Process.Pid)
		if stopped && remaining == 0 {
			return terminationResult{leaderStopped: true, forcedMembers: members > 0, err: errors.Join(inspectErr, killErr, observeErr, groupErr)}
		}
		if observeErr != nil || groupErr != nil {
			return terminationResult{leaderStopped: stopped, forcedMembers: members > 0, err: errors.Join(inspectErr, killErr, observeErr, groupErr)}
		}
		if time.Now().After(deadline) {
			return terminationResult{leaderStopped: stopped, forcedMembers: members > 0, err: errors.Join(inspectErr, killErr, fmt.Errorf("Darwin process group remained active after cleanup deadline"))}
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func darwinLiveGroupMembers(pgid int) (int, error) {
	processes, err := unix.SysctlKinfoProcSlice("kern.proc.all")
	if err != nil {
		return 0, fmt.Errorf("inspect Darwin process group: %w", err)
	}
	const zombieState = 5
	members := 0
	for _, process := range processes {
		if int(process.Proc.P_pid) != pgid && int(process.Eproc.Pgid) == pgid && process.Proc.P_stat != zombieState {
			members++
		}
	}
	return members, nil
}

func (containment *commandContainment) exited() (bool, error) {
	return containment.exitObservation.observe(func() (bool, error) {
		events := make([]unix.Kevent_t, 1)
		timeout := unix.Timespec{}
		n, err := containment.callKevent(nil, events, &timeout)
		if errors.Is(err, syscall.EINTR) {
			// A signal can interrupt this non-blocking observation after Git or
			// another short-lived child exits. The NOTE_EXIT event remains queued,
			// so let the supervised poll retry instead of failing a clean command.
			return false, nil
		}
		return err == nil && n > 0 && events[0].Fflags&unix.NOTE_EXIT != 0, err
	})
}

func (containment *commandContainment) callKevent(changes, events []unix.Kevent_t, timeout *unix.Timespec) (int, error) {
	if containment.kevent != nil {
		return containment.kevent(containment.kqueue, changes, events, timeout)
	}
	return unix.Kevent(containment.kqueue, changes, events, timeout)
}

func (containment *commandContainment) groupMembers(pgid int) (int, error) {
	if containment.liveGroupMembers != nil {
		return containment.liveGroupMembers(pgid)
	}
	return darwinLiveGroupMembers(pgid)
}

func (containment *commandContainment) signalGroup(pid int, signal syscall.Signal) error {
	if containment.killGroup != nil {
		return containment.killGroup(pid, signal)
	}
	return syscall.Kill(pid, signal)
}

func (containment *commandContainment) settleNaturalExit(ctx context.Context, timeout time.Duration) (bool, error) {
	deadline := time.Now().Add(timeout)
	for {
		members, err := containment.groupMembers(containment.cmd.Process.Pid)
		if err != nil || members == 0 {
			return members == 0, err
		}
		if time.Now().After(deadline) {
			return false, nil
		}
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		case <-time.After(5 * time.Millisecond):
		}
	}
}

func (containment *commandContainment) close() error {
	if containment.kqueue < 0 {
		return nil
	}
	err := unix.Close(containment.kqueue)
	containment.kqueue = -1
	return err
}
