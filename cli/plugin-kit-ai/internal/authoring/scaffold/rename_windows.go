//go:build windows

package scaffold

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"runtime"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// FILE_RENAME_INFORMATION with ReplaceIfExists FALSE fails if any destination
// exists. Both names are single components relative to held directory handles.
// This avoids MoveFileEx's pathname races and never enables REPLACE_IF_EXISTS or
// POSIX_SEMANTICS. Unsupported filesystems fail without a weaker fallback.
func renameExclusive(from *os.File, old string, to *os.File, new string) error {
	return renameResult(from, old, to, new, renameWindows(from, old, to, new, nil))
}

func renamePackage(ctx context.Context, from *os.File, old string, to *os.File, new string, check func() error) error {
	r := &renameRetry{ctx: ctx, check: check, budget: time.Second, attempts: 32, wait: waitRename}
	return renameResult(from, old, to, new, renameWindows(from, old, to, new, r))
}

func renameResult(from *os.File, old string, to *os.File, new string, err error) error {
	runtime.KeepAlive(from)
	runtime.KeepAlive(to)
	if err != nil {
		err = windowsRenameError(err, to, new)
		// Native NTSTATUS errors do not implement errors.Is. Convert to the
		// Win32 errno so callers can classify collisions with os.ErrExist.
		var status windows.NTStatus
		if errors.As(err, &status) {
			err = fmt.Errorf("%v: %w", err, status.Errno())
		}
		return &os.LinkError{Op: "rename-exclusive", Old: old, New: new, Err: err}
	}
	return nil
}

func windowsRenameError(err error, to *os.File, new string) error {
	// Both NT calls return NTStatus, which lacks Is/Unwrap in pinned x/sys, but
	// renameWindows already wraps failures with %w so errors.As still finds
	// them. Join rather than replace so any "commit exclusive directory
	// rename" context from renameWindows survives into the final error, while
	// the Win32 errno becomes discoverable for os.ErrExist/os.ErrNotExist
	// classification even for callers that never call renameResult.
	var status windows.NTStatus
	if errors.As(err, &status) {
		err = errors.Join(err, status.Errno())
	}
	if !errors.Is(err, windows.STATUS_SHARING_VIOLATION) {
		return err
	}
	// A winner's delete-denying data handle can make sharing failure precede
	// name collision. Observe existence only after failure; never retry rename.
	// Metadata access avoids data opens, and the single rooted component is
	// opened as a reparse point so even a dangling/outside link counts as existing.
	name, e := windows.NewNTUnicodeString(new)
	if e != nil {
		return err
	}
	oa := windows.OBJECT_ATTRIBUTES{RootDirectory: windows.Handle(to.Fd()), ObjectName: name, Attributes: windows.OBJ_CASE_INSENSITIVE}
	oa.Length = uint32(unsafe.Sizeof(oa))
	var handle windows.Handle
	e = windows.NtCreateFile(&handle, windows.FILE_READ_ATTRIBUTES|windows.SYNCHRONIZE, &oa, &windows.IO_STATUS_BLOCK{}, nil, 0,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, windows.FILE_OPEN,
		windows.FILE_OPEN_REPARSE_POINT|windows.FILE_SYNCHRONOUS_IO_NONALERT, 0, 0)
	runtime.KeepAlive(to)
	if e != nil {
		return err // Missing or unobservable destination: no new existence claim.
	}
	if e = windows.CloseHandle(handle); e != nil {
		return errors.Join(err, e)
	}
	return errors.Join(err, fmt.Errorf("destination already exists: %w", fs.ErrExist))
}

// Retry belongs to package publication only. A nil policy is the original
// single-attempt primitive used by ApplySkill and its source CAS.
type renameRetry struct {
	ctx      context.Context
	check    func() error
	budget   time.Duration
	attempts int
	wait     func(context.Context, time.Duration) error
}

func waitRename(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func renameWindows(from *os.File, old string, to *os.File, new string, retry *renameRetry) error {
	if retry != nil {
		if err := retry.check(); err != nil {
			return err
		}
	}
	name, err := windows.NewNTUnicodeString(old)
	if err != nil {
		return err
	}
	oa := windows.OBJECT_ATTRIBUTES{RootDirectory: windows.Handle(from.Fd()), ObjectName: name, Attributes: windows.OBJ_CASE_INSENSITIVE}
	oa.Length = uint32(unsafe.Sizeof(oa))
	var status windows.IO_STATUS_BLOCK
	var handle windows.Handle
	err = windows.NtCreateFile(&handle, windows.DELETE|windows.SYNCHRONIZE, &oa, &status, nil, 0,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, windows.FILE_OPEN,
		windows.FILE_DIRECTORY_FILE|windows.FILE_OPEN_REPARSE_POINT|windows.FILE_SYNCHRONOUS_IO_NONALERT, 0, 0)
	if err != nil {
		return fmt.Errorf("open source directory for exclusive rename: %w", err)
	}
	defer windows.CloseHandle(handle)
	target, err := windows.UTF16FromString(new)
	if err != nil {
		return err
	}
	// Native structure alignment differs between 32/64 bit; use Offsetof.
	type renameInfo struct {
		ReplaceIfExists byte
		RootDirectory   windows.Handle
		FileNameLength  uint32
		FileName        [1]uint16
	}
	var header renameInfo
	size := max(int(unsafe.Sizeof(header)), int(unsafe.Offsetof(header.FileName))+(len(target)-1)*2)
	buffer := make([]byte, size)
	info := (*renameInfo)(unsafe.Pointer(&buffer[0]))
	info.RootDirectory = windows.Handle(to.Fd())
	info.FileNameLength = uint32((len(target) - 1) * 2)
	copy(unsafe.Slice(&info.FileName[0], len(target)-1), target[:len(target)-1])
	var started time.Time
	var lastFailure error
	delay := 10 * time.Millisecond
	for attempt := 1; ; attempt++ {
		if retry != nil {
			if err := retry.check(); err != nil {
				return err
			}
			if err := retry.ctx.Err(); err != nil {
				return err
			}
		}
		if retry != nil && !started.IsZero() && time.Since(started) >= retry.budget {
			return lastFailure
		}
		err := windows.NtSetInformationFile(handle, &status, &buffer[0], uint32(size), windows.FileRenameInformation)
		if err == nil {
			return nil
		}
		failure := fmt.Errorf("commit exclusive directory rename: %w", err)
		lastFailure = failure
		// Only this failed native operation is eligible. Keep the same source
		// handle, anchored parents and no-replace request throughout the loop.
		if retry == nil || err != windows.STATUS_SHARING_VIOLATION {
			return failure
		}
		if started.IsZero() {
			started = time.Now()
		}
		remaining := retry.budget - time.Since(started)
		if attempt >= retry.attempts || remaining <= 0 {
			return failure
		}
		if err := retry.wait(retry.ctx, min(delay, remaining)); err != nil {
			return err
		}
		if err := retry.ctx.Err(); err != nil {
			return err
		}
		if time.Since(started) >= retry.budget {
			return failure
		}
		delay = min(delay*2, 50*time.Millisecond)
	}
}
