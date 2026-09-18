//go:build darwin || linux

package kiro

import (
	"errors"
	"fmt"
	"io"
	"os"

	"golang.org/x/sys/unix"
)

// queuedACPRealPipeEvidence establishes the settlement boundary in the kernel:
// poll reports every byte and EOF ordered before this call, independent of when
// the goroutine next gets scheduled to issue Read.
func queuedACPRealPipeEvidence(reader io.Reader) (queued, eof bool, resultErr error) {
	file, ok := reader.(*os.File)
	if !ok {
		return false, false, nil
	}
	raw, err := file.SyscallConn()
	if err != nil {
		return false, false, err
	}
	err = raw.Control(func(fd uintptr) {
		if fd > uintptr(^uint32(0)>>1) {
			resultErr = fmt.Errorf("ACP pipe descriptor overflows poll fd")
			return
		}
		poll := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN | unix.POLLHUP}}
		for range 3 {
			_, pollErr := unix.Poll(poll, 0)
			if errors.Is(pollErr, unix.EINTR) {
				resultErr = pollErr
				continue
			}
			resultErr = pollErr
			break
		}
		if resultErr != nil {
			return
		}
		events := poll[0].Revents
		if events&unix.POLLNVAL != 0 {
			resultErr = fmt.Errorf("ACP pipe descriptor is invalid")
			return
		}
		if events&unix.POLLIN != 0 {
			available, ioctlErr := queuedPipeBytes(int(fd))
			if ioctlErr != nil {
				resultErr = fmt.Errorf("inspect queued ACP pipe bytes: %w", ioctlErr)
				return
			}
			queued = available > 0
		}
		eof = events&unix.POLLHUP != 0 && !queued
	})
	return queued, eof, errors.Join(resultErr, err)
}
