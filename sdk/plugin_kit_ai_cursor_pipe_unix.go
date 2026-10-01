//go:build linux || darwin

package pluginkitai

import (
	"fmt"
	"os"
	"syscall"
	"time"
)

// Inherited standard streams are often blocking NewFile wrappers that cannot
// accept deadlines. Transfer them to nonblocking pollable duplicates. Duplicate
// stdout also keeps a broken pipe from terminating Go via fd 1 SIGPIPE handling.
func prepareCursorPipe(source *os.File) (*os.File, error) {
	if source == nil {
		return nil, fmt.Errorf("Cursor observer requires owned pipe IO")
	}
	defer source.Close()
	stat, err := source.Stat()
	if err != nil {
		return nil, err
	}
	if stat.Mode()&os.ModeNamedPipe == 0 {
		return nil, fmt.Errorf("Cursor observer requires pipe IO")
	}
	raw, err := source.SyscallConn()
	if err != nil {
		return nil, err
	}
	fd := -1
	var setupErr error
	err = raw.Control(func(value uintptr) {
		syscall.ForkLock.RLock()
		fd, setupErr = syscall.Dup(int(value))
		if setupErr == nil {
			syscall.CloseOnExec(fd)
		}
		syscall.ForkLock.RUnlock()
		if setupErr == nil {
			setupErr = syscall.SetNonblock(fd, true)
		}
	})
	if err != nil || setupErr != nil {
		if fd >= 0 {
			_ = syscall.Close(fd)
		}
		if err != nil {
			return nil, err
		}
		return nil, setupErr
	}
	file := os.NewFile(uintptr(fd), "cursor-observer-pipe")
	if err := file.SetDeadline(time.Time{}); err != nil {
		_ = file.Close()
		return nil, err
	}
	return file, nil
}
