//go:build !windows

package nativeconfig

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

func lockNativeConfig(path string) (func() error, error) {
	fd, err := syscall.Open(path, syscall.O_CREAT|syscall.O_RDWR|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0o600)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(fd), path)
	closeOnError := func(err error) (func() error, error) {
		_ = file.Close()
		return nil, err
	}
	info, err := file.Stat()
	if err != nil {
		return closeOnError(err)
	}
	if !info.Mode().IsRegular() {
		return closeOnError(fmt.Errorf("native config lock must be a regular file"))
	}
	if err := syscall.Flock(fd, syscall.LOCK_EX); err != nil {
		return closeOnError(err)
	}
	return func() error {
		unlockErr := syscall.Flock(fd, syscall.LOCK_UN)
		closeErr := file.Close()
		return errors.Join(unlockErr, closeErr)
	}, nil
}
