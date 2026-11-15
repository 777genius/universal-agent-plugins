//go:build linux || darwin

package cline

import (
	"os"
	"syscall"
)

// Descriptor-only inspection must neither follow a swapped final symlink nor
// wait on a foreign FIFO substituted between Lstat and open. Directory pins
// reject a substituted ordinary file in the kernel before a read is possible.
func openSessionPinnedPath(path string, directory bool) (*os.File, error) {
	flags := syscall.O_RDONLY | syscall.O_CLOEXEC | syscall.O_NOFOLLOW | syscall.O_NONBLOCK
	if directory {
		flags |= syscall.O_DIRECTORY
	}
	fd, err := syscall.Open(path, flags, 0)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(fd), path), nil
}
