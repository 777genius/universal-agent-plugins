//go:build linux

package kiro

import "golang.org/x/sys/unix"

func queuedPipeBytes(fd int) (int, error) {
	return unix.IoctlGetInt(fd, unix.TIOCINQ)
}
