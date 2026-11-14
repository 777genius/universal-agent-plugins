//go:build !windows

package processlock

import (
	"os"

	"golang.org/x/sys/unix"
)

func acquireFile(file *os.File) error {
	return unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB)
}

func releaseFile(file *os.File) error {
	return unix.Flock(int(file.Fd()), unix.LOCK_UN)
}
