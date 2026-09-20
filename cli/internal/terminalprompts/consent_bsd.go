//go:build darwin || freebsd || openbsd || netbsd || dragonfly

package terminalprompts

import (
	"os"

	"golang.org/x/sys/unix"
)

func queuedInputBytes(f *os.File) (int, error) {
	// FIONREAD = _IOR('f', 127, int), not exported by x/sys on BSD systems.
	return unix.IoctlGetInt(int(f.Fd()), 0x4004667f)
}
