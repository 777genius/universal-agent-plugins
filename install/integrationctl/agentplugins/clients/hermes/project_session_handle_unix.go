//go:build linux || darwin

package hermes

import (
	"fmt"
	"os"
	"syscall"
)

func validateSessionHandle(_ *os.File, info os.FileInfo) error {
	metadata, ok := info.Sys().(*syscall.Stat_t)
	if !ok || metadata.Nlink != 1 || int64(metadata.Uid) != int64(os.Geteuid()) || info.Mode().Perm() != 0o600 {
		return fmt.Errorf("hermes session source must be private, single-link and owned")
	}
	return nil
}

func validateSessionDirectory(info os.FileInfo) error {
	metadata, ok := info.Sys().(*syscall.Stat_t)
	if !ok || !info.IsDir() || int64(metadata.Uid) != int64(os.Geteuid()) || info.Mode().Perm() != 0o700 {
		return fmt.Errorf("hermes session directory must be private and owned")
	}
	return nil
}

func validateSessionDirectoryHandle(_ *os.File, info os.FileInfo) error {
	return validateSessionDirectory(info)
}

func openSessionFile(root *os.Root, name string) (*os.File, error) {
	return root.OpenFile(name, os.O_RDONLY|syscall.O_NONBLOCK|syscall.O_NOFOLLOW, 0)
}
