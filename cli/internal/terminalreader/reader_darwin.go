//go:build darwin

// Package terminalreader owns cancellable reads without owning caller handles.
package terminalreader

import (
	"fmt"
	"os"

	"github.com/muesli/cancelreader"
	"golang.org/x/sys/unix"
	"golang.org/x/term"
)

// New preserves the existing kqueue backend for physical terminals. Darwin's
// virtual /dev/tty needs select, even when inherited as /dev/stdin. Compare
// device identities without a readiness probe or a new borrowed-file wrapper.
func New(f *os.File) (cancelreader.CancelReader, error) {
	if !term.IsTerminal(int(f.Fd())) {
		return cancelreader.NewReader(f)
	}
	var input, virtual unix.Stat_t
	if err := unix.Fstat(int(f.Fd()), &input); err != nil {
		return nil, fmt.Errorf("inspect terminal device: %w", err)
	}
	if err := unix.Stat("/dev/tty", &virtual); err != nil {
		return nil, fmt.Errorf("inspect virtual terminal device: %w", err)
	}
	if input.Rdev != virtual.Rdev {
		return cancelreader.NewReader(f)
	}
	// Vendor select otherwise falls back to an uncancellable reader at this limit.
	if f.Fd() >= unix.FD_SETSIZE {
		return nil, fmt.Errorf("virtual terminal descriptor exceeds select limit")
	}
	return cancelreader.NewReader(virtualTTY{f})
}

// Only the backend hint changes. The original file, descriptor and finalizer
// remain caller-owned; cancelreader closes only its own wakeup descriptors.
type virtualTTY struct{ *os.File }

func (virtualTTY) Name() string { return "/dev/tty" }
