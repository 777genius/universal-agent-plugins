//go:build !windows

package qoder

import (
	"os"

	"golang.org/x/sys/unix"
)

func openProjectManifest(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
}
