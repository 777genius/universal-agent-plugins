//go:build !windows

package grok

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
)

func openProjectHandle(path string, directory bool) (*os.File, error) {
	flags := unix.O_RDONLY | unix.O_CLOEXEC | unix.O_NOFOLLOW | unix.O_NONBLOCK
	fd, err := unix.Open(string(filepath.Separator), flags|unix.O_DIRECTORY, 0)
	if err != nil {
		return nil, err
	}
	parts := strings.Split(strings.TrimPrefix(path, string(filepath.Separator)), string(filepath.Separator))
	for i, part := range parts {
		if part == "" {
			_ = unix.Close(fd)
			return nil, fmt.Errorf("grok discovery path has an empty component")
		}
		selected := flags
		if i < len(parts)-1 || directory {
			selected |= unix.O_DIRECTORY
		}
		next, openErr := unix.Openat(fd, part, selected, 0)
		_ = unix.Close(fd)
		if openErr != nil {
			return nil, openErr
		}
		fd = next
	}
	return os.NewFile(uintptr(fd), path), nil
}
