//go:build linux

package grok

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

// The native file must already have been created by the qualified native
// startup. UAP never replaces this permanent advisory-lock identity.
func lockNativeEnablement(root string) (func() error, error) {
	path := filepath.Join(root, ".config-init.lock")
	file, info, err := openProjectPath(path, false, nil)
	if err != nil {
		return nil, err
	}
	actual, ok := info.Sys().(*unix.Stat_t)
	if !ok {
		var stat unix.Stat_t
		if err := unix.Fstat(int(file.Fd()), &stat); err != nil {
			_ = file.Close()
			return nil, err
		}
		actual = &stat
	}
	if info.Size() != 0 || actual.Nlink != 1 || int64(actual.Uid) != int64(os.Getuid()) ||
		(info.Mode().Perm() != 0600 && info.Mode().Perm() != 0644) {
		_ = file.Close()
		return nil, fmt.Errorf("grok native configuration lock authority differs")
	}
	if err := unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("grok native configuration is busy: %w", err)
	}
	if err := stableProjectPath(path, file, info); err != nil {
		_ = file.Close()
		return nil, err
	}
	return func() error {
		stableErr := stableProjectPath(path, file, info)
		return errors.Join(stableErr, unix.Flock(int(file.Fd()), unix.LOCK_UN), file.Close())
	}, nil
}
