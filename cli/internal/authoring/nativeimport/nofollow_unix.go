//go:build linux || darwin

package nativeimport

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"golang.org/x/sys/unix"
)

func openNoFollow(path string) (*os.File, error) {
	clean := filepath.Clean(path)
	// macOS exposes these sealed system directories through root-level aliases.
	// Traverse their canonical /private locations while retaining no-follow
	// checks for every user-controlled ancestor below them.
	if runtime.GOOS == "darwin" {
		for _, alias := range []string{"var", "tmp", "etc"} {
			prefix := string(filepath.Separator) + alias
			if clean == prefix || strings.HasPrefix(clean, prefix+string(filepath.Separator)) {
				clean = filepath.Join(string(filepath.Separator), "private", strings.TrimPrefix(clean, string(filepath.Separator)))
				break
			}
		}
	}
	root := "."
	components := strings.Split(clean, string(filepath.Separator))
	if filepath.IsAbs(clean) {
		root = string(filepath.Separator)
		components = components[1:]
	}
	if len(components) == 0 {
		return nil, os.ErrInvalid
	}

	fd, err := unix.Open(root, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_DIRECTORY, 0)
	if err != nil {
		return nil, err
	}
	for _, component := range components[:len(components)-1] {
		next, openErr := unix.Openat(fd, component, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW|unix.O_DIRECTORY, 0)
		closeErr := unix.Close(fd)
		if err = errors.Join(openErr, closeErr); err != nil {
			if openErr == nil {
				_ = unix.Close(next)
			}
			return nil, err
		}
		fd = next
	}
	fileFD, openErr := unix.Openat(fd, components[len(components)-1], unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	err = errors.Join(openErr, unix.Close(fd))
	if err != nil {
		if openErr == nil {
			_ = unix.Close(fileFD)
		}
		return nil, err
	}
	return os.NewFile(uintptr(fileFD), filepath.Base(path)), nil
}
