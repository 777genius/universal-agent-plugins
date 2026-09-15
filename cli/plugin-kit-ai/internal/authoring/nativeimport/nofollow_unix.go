//go:build linux || darwin

package nativeimport

import (
	"errors"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

func openNoFollow(path string) (*os.File, error) {
	parent, err := os.Open(filepath.Dir(path))
	if err != nil {
		return nil, err
	}
	fd, openErr := unix.Openat(int(parent.Fd()), filepath.Base(path), unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	err = errors.Join(openErr, parent.Close())
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(fd), filepath.Base(path)), nil
}
