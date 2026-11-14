//go:build darwin

package process

import (
	"os"
	"os/exec"
	"syscall"
	"testing"

	"golang.org/x/sys/unix"
)

func TestDarwinExitObservationRetriesAfterInterruptedSystemCall(t *testing.T) {
	calls := 0
	containment := &commandContainment{kevent: func(_ int, _ []unix.Kevent_t, events []unix.Kevent_t, _ *unix.Timespec) (int, error) {
		calls++
		if calls == 1 {
			return 0, syscall.EINTR
		}
		events[0].Fflags = unix.NOTE_EXIT
		return 1, nil
	}}

	if exited, err := containment.exited(); err != nil || exited {
		t.Fatalf("interrupted observation = %v, %v, want pending without error", exited, err)
	}
	if exited, err := containment.exited(); err != nil || !exited {
		t.Fatalf("retried observation = %v, %v, want exit", exited, err)
	}
}

func TestDarwinAttachRetriesInterruptedRegistration(t *testing.T) {
	calls := 0
	containment := &commandContainment{kevent: func(_ int, changes []unix.Kevent_t, _ []unix.Kevent_t, _ *unix.Timespec) (int, error) {
		calls++
		if len(changes) != 1 {
			t.Fatalf("registration changes = %d, want 1", len(changes))
		}
		if calls == 1 {
			return 0, syscall.EINTR
		}
		return 0, nil
	}}
	cmd := exec.Command("true")
	cmd.Process = &os.Process{Pid: 1234}
	if err := containment.attach(cmd); err != nil {
		t.Fatalf("attach: %v", err)
	}
	if calls != 2 {
		t.Fatalf("registration calls = %d, want 2", calls)
	}
}
