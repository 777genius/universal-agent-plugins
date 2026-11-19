//go:build windows

package agentpluginscli

import (
	"errors"
	"syscall"
)

// Windows reports these conditions as native error numbers that the portable
// syscall constants do not match: ERROR_WRITE_PROTECT (19),
// ERROR_HANDLE_DISK_FULL (39) and ERROR_DISK_FULL (112).
func isDiskFull(err error) bool {
	return errors.Is(err, syscall.ENOSPC) || errors.Is(err, syscall.Errno(39)) || errors.Is(err, syscall.Errno(112))
}

func isReadOnly(err error) bool {
	return errors.Is(err, syscall.EROFS) || errors.Is(err, syscall.Errno(19))
}
