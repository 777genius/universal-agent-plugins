//go:build linux

package grok

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"golang.org/x/sys/unix"
)

const grokACPTailLimit = 1 << 20

// The duplicate is unread until RunDuplexWithPlannedShutdown has proved the
// entire owned family closed. Keeping only its fd avoids touching the runtime
// poller or shared file-status flags while the exchange reader is active.
type grokACPTail struct{ fd int }

func captureGrokACPTail(stdout io.Reader) (*grokACPTail, error) {
	return duplicateGrokACPPipe(stdout, func(fd int) (int, error) {
		return unix.FcntlInt(uintptr(fd), unix.F_DUPFD_CLOEXEC, 0)
	})
}

func duplicateGrokACPPipe(stdout io.Reader, duplicate func(int) (int, error)) (*grokACPTail, error) {
	file, ok := stdout.(*os.File)
	if !ok {
		return nil, fmt.Errorf("grok ACP requires its owned stdout pipe")
	}
	raw, err := file.SyscallConn()
	if err != nil {
		return nil, err
	}
	fd := -1
	var captureErr error
	err = raw.Control(func(original uintptr) {
		var stat unix.Stat_t
		if captureErr = unix.Fstat(int(original), &stat); captureErr != nil {
			return
		}
		flags, flagErr := unix.FcntlInt(original, unix.F_GETFL, 0)
		if flagErr != nil || stat.Mode&unix.S_IFMT != unix.S_IFIFO || flags&unix.O_NONBLOCK == 0 {
			captureErr = errors.Join(flagErr, fmt.Errorf("grok ACP stdout is not an owned pollable pipe"))
			return
		}
		fd, captureErr = duplicate(int(original))
	})
	if err != nil || captureErr != nil || fd < 0 {
		if fd >= 0 {
			_ = unix.Close(fd)
		}
		return nil, errors.Join(err, captureErr, fmt.Errorf("retain grok ACP stdout"))
	}
	return &grokACPTail{fd: fd}, nil
}

func (tail *grokACPTail) close() error {
	if tail == nil || tail.fd < 0 {
		return nil
	}
	fd := tail.fd
	tail.fd = -1
	return unix.Close(fd)
}

// drain is called only after the runner returns successful family cleanup.
// The original buffered reader's unread bytes are part of this same stream.
func (tail *grokACPTail) drain(ctx context.Context, prefetched []byte) (resultErr error) {
	if tail == nil || tail.fd < 0 {
		return fmt.Errorf("grok ACP retained stdout is unavailable")
	}
	fd := tail.fd
	tail.fd = -1
	file := os.NewFile(uintptr(fd), "grok-acp-tail")
	if file == nil {
		return errors.Join(unix.Close(fd), fmt.Errorf("open retained grok ACP stdout"))
	}
	defer func() { resultErr = errors.Join(resultErr, file.Close()) }()
	if err := ctx.Err(); err != nil {
		return err
	}
	deadline := time.Now().Add(250 * time.Millisecond)
	if end, ok := ctx.Deadline(); ok && end.Before(deadline) {
		deadline = end
	}
	if err := file.SetReadDeadline(deadline); err != nil {
		return fmt.Errorf("grok ACP stdout cannot bound final EOF: %w", err)
	}
	if len(prefetched) > grokACPTailLimit {
		return fmt.Errorf("grok ACP trailing output exceeds its bound")
	}
	body, err := io.ReadAll(io.LimitReader(file, int64(grokACPTailLimit+1-len(prefetched))))
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err != nil {
		return fmt.Errorf("grok ACP stdout did not finish with bounded EOF: %w", err)
	}
	if len(prefetched)+len(body) != 0 {
		return fmt.Errorf("grok ACP emitted trailing output after its exact response")
	}
	return nil
}
