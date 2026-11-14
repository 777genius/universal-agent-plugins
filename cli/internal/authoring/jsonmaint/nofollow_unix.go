//go:build linux || darwin

package jsonmaint

import (
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

func openNoFollow(root *os.Root, name string) (*os.File, error) {
	parent, err := root.Open(".")
	if err != nil {
		return nil, err
	}
	fd, openErr := unix.Openat(int(parent.Fd()), name, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	err = errors.Join(openErr, parent.Close())
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(fd), name), nil
}
