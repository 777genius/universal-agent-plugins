//go:build linux || darwin

package pluginkitai

import (
	"fmt"
	"os"
	"runtime"
	"syscall"
	"time"
)

// Inherited pipes and anonymous UNIX stream socketpairs (Node/libuv stdio) can
// be blocking NewFile wrappers. Transfer them to nonblocking pollable duplicates.
// Both duplicates must be above standard IO even when unrelated stdio is closed;
// os.NewFile on fd 1/2 enables Go's standard-output SIGPIPE termination behavior.
func prepareCursorPipe(source *os.File) (*os.File, error) {
	if source == nil {
		return nil, fmt.Errorf("Cursor observer requires owned pipe IO")
	}
	defer func() { _ = source.Close() }()
	stat, err := source.Stat()
	if err != nil {
		return nil, err
	}
	if stat.Mode()&(os.ModeNamedPipe|os.ModeSocket) == 0 {
		return nil, fmt.Errorf("Cursor observer requires pipe or anonymous UNIX stream IO")
	}
	raw, err := source.SyscallConn()
	if err != nil {
		return nil, err
	}
	fd := -1
	var setupErr error
	err = raw.Control(func(value uintptr) {
		if stat.Mode()&os.ModeSocket != 0 {
			if setupErr = validateCursorSocket(int(value)); setupErr != nil {
				return
			}
		}
		syscall.ForkLock.RLock()
		duplicate, _, errno := syscall.Syscall(syscall.SYS_FCNTL, value, syscall.F_DUPFD, 3)
		if errno != 0 {
			setupErr = errno
		} else {
			fd = int(duplicate)
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
	file := os.NewFile(uintptr(fd), "cursor-observer-ipc")
	if err := file.SetDeadline(time.Time{}); err != nil {
		_ = file.Close()
		return nil, err
	}
	return file, nil
}

// Validate under RawConn.Control before duplicating. Both native name calls
// must succeed, excluding listeners/unconnected sockets, and report AF_UNIX
// with no address. This is anonymous IPC, not a named/network socket transport.
func validateCursorSocket(fd int) error {
	kind, err := syscall.GetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_TYPE)
	if err != nil {
		return err
	}
	if kind != syscall.SOCK_STREAM {
		return fmt.Errorf("Cursor observer requires anonymous UNIX stream IO")
	}
	if err := cursorSocketAnonymous(fd, false); err != nil {
		return err
	}
	return cursorSocketAnonymous(fd, true)
}

// The Linux syscall SockaddrUnix wrapper loses the returned length and renders
// even unnamed socketpair endpoints as "@". Checking that string would also
// admit named abstract addresses containing NULs. Preserve the native length:
// Linux anonymous addresses contain only the two-byte family. Darwin has no
// abstract namespace and may return a padded sockaddr with an empty path.
func cursorSocketAnonymous(fd int, peer bool) error {
	var address syscall.RawSockaddrUnix
	size := uint32(syscall.SizeofSockaddrUnix)
	errno := cursorSocketName(fd, peer, &address, &size)
	if errno != 0 {
		return errno
	}
	if address.Family != syscall.AF_UNIX || size < 2 || size > syscall.SizeofSockaddrUnix || address.Path[0] != 0 || (runtime.GOOS == "linux" && size != 2) {
		return fmt.Errorf("Cursor observer requires anonymous UNIX stream IO")
	}
	return nil
}
