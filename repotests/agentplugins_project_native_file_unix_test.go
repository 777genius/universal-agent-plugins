//go:build !windows

package pluginkitairepo_test

import (
	"golang.org/x/sys/unix"
	"os"
)

func openProjectNativeSnapshot(path string) (*os.File, error) {
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(fd), path), nil
}
