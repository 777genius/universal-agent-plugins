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
	if err := unix.IoctlSetTermios(fd, unix.TIOCSETA, attrs); err != nil {
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
	_, err := unix.IoctlGetInt(fd, 0x4004667f)
	return err
}
