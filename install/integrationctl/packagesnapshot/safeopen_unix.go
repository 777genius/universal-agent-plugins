//go:build !windows

package packagesnapshot

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/sys/unix"
)

// openSnapshotRegular resolves every component relative to an already-opened
// directory descriptor. O_NOFOLLOW prevents a concurrently substituted
// symlink from escaping the source tree.
func openSnapshotRegular(root, rel string) (*os.File, error) {
	current, err := unix.Open(root, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_DIRECTORY|unix.O_NOFOLLOW, 0)
	if err != nil {
		return nil, fmt.Errorf("open snapshot root without symlinks: %w", err)
	}
	parts := strings.Split(rel, "/")
	for index, part := range parts {
		flags := unix.O_RDONLY | unix.O_CLOEXEC | unix.O_NOFOLLOW
		if index < len(parts)-1 {
			flags |= unix.O_DIRECTORY
		}
		next, openErr := unix.Openat(current, part, flags, 0)
		_ = unix.Close(current)
		if openErr != nil {
			return nil, fmt.Errorf("open snapshot component %q without symlinks: %w", part, openErr)
		}
		current = next
	}
	file := os.NewFile(uintptr(current), rel)
	if file == nil {
		_ = unix.Close(current)
		return nil, fmt.Errorf("adopt snapshot file descriptor")
	}
	return file, nil
}

func requireSingleLink(file *os.File) error {
	var info unix.Stat_t
	if err := unix.Fstat(int(file.Fd()), &info); err != nil {
		return fmt.Errorf("inspect hardlink count: %w", err)
	}
	if info.Nlink != 1 {
		return fmt.Errorf("hardlinks are not allowed")
	}
	return nil
}
