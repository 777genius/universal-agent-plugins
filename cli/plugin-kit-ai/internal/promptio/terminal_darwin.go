//go:build darwin

package promptio

import "golang.org/x/sys/unix"

// SnapshotTerminal returns a restore callback for the current terminal state.
// Call it after this owner's input readers and cancellation callbacks have joined.
func SnapshotTerminal(fd int) (func() error, error) {
	attrs, err := unix.IoctlGetTermios(fd, unix.TIOCGETA)
	if err != nil {
		return nil, err
	}
	return func() error { return restoreTerminal(fd, attrs) }, nil
}

func restoreTerminal(fd int, attrs *unix.Termios) error {
	initial := *attrs
	if attrs.Lflag&(unix.ICANON|unix.PENDIN) == unix.ICANON {
		current, err := unix.IoctlGetTermios(fd, unix.TIOCGETA)
		if err != nil {
			return err
		}
		if current.Lflag&unix.ICANON == 0 {
			// TIOCSETA wakes kqueue readers before installing ICANON. Their
			// ttnread can clear the kernel's PENDIN under the old raw settings,
			// leaving even newline-terminated input in the raw queue. Include
			// PENDIN in the requested flags so canonical replay remains due
			// after the new settings are installed.
			initial.Lflag |= unix.PENDIN
		}
	}
	if err := unix.IoctlSetTermios(fd, unix.TIOCSETA, &initial); err != nil {
		return err
	}
	if attrs.Lflag&unix.PENDIN != 0 {
		return nil
	}
	// Darwin can retain PENDIN across TIOCSETA. FIONREAD lets the line
	// discipline reprocess pending input and clear that transient flag without
	// dequeuing any bytes into userspace. Do not do this when PENDIN was set in
	// the caller's snapshot: that pending work belongs to the next owner.
	// FIONREAD = _IOR('f', 127, int), not exported by x/sys on Darwin.
	if _, err := unix.IoctlGetInt(fd, 0x4004667f); err != nil {
		return err
	}
	// Replay can change other flags (ttyecho clears FLUSHO). Restore the
	// snapshot again now that PENDIN is clear. ICANON already matches, so this
	// TIOCSETA does not schedule another replay.
	return unix.IoctlSetTermios(fd, unix.TIOCSETA, attrs)
}
