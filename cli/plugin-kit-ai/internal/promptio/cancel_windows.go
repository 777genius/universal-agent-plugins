//go:build windows

package promptio

import (
	"context"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"time"
	"unicode/utf16"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/prompt"

	"golang.org/x/sys/windows"
)

var cancelSynchronousIO = windows.NewLazySystemDLL("kernel32.dll").NewProc("CancelSynchronousIo")

// Private, per-call diagnostic seam. Nil in normal operation; qualification
// records into preallocated memory and formats only after the reader has joined.
// The context keeps concurrent callers isolated without a process-global hook.
type cancelWindowsKey struct{}
type cancelWindowsEvent struct {
	operation string
	handle    windows.Handle
	threadID  uint32
	units     uint32
	err       error
}
type cancelWindowsOps struct {
	observe   func(cancelWindowsEvent)
	duplicate func(windows.Handle, windows.Handle, windows.Handle, *windows.Handle, uint32, bool, uint32) error
	close     func(windows.Handle) error
	open      func() (windows.Handle, error)
	mode      func(windows.Handle, *uint32) error
	read      func(context.Context, windows.Handle, uint32) (string, error)
}

// Console requests own a separately opened input file object. A duplicate
// would retain the inherited file object's lifetime when our handle closes.
// Open only for validated console input; keep pipe cancellation unchanged.
// The console host still owns editing and echo, and no input is flushed.
func readCancelable(ctx context.Context, r io.Reader) (string, error) {
	ops, _ := ctx.Value(cancelWindowsKey{}).(*cancelWindowsOps)
	emit := func(op string, h windows.Handle, err error) {
		if ops != nil && ops.observe != nil {
			ops.observe(cancelWindowsEvent{operation: op, handle: h, threadID: windows.GetCurrentThreadId(), err: err})
		}
	}
	duplicate, closeHandle := windows.DuplicateHandle, windows.CloseHandle
	if ops != nil {
		if ops.duplicate != nil {
			duplicate = ops.duplicate
		}
		if ops.close != nil {
			closeHandle = ops.close
		}
	}
	closeOwned := func(kind string, h windows.Handle) { emit(kind, h, closeHandle(h)) }
	emit("request-start", 0, nil)
	defer emit("request-end", 0, nil)
	f, ok := r.(*os.File)
	if !ok {
		emit("branch-reader", 0, nil)
		emit("read-enter", 0, nil)
		line, err := readLine(ctx, r)
		emit("read-complete", 0, err)
		return line, err
	}
	if ops != nil && ops.observe != nil {
		emit("branch-file", windows.Handle(f.Fd()), nil)
	}
	if err := ctx.Err(); err != nil {
		emit("pre-cancel", 0, err)
		return "", err
	}
	read := func() (string, error) { return readLine(ctx, r) }
	getMode := windows.GetConsoleMode
	consoleRead := readConsoleLine
	open := func() (windows.Handle, error) {
		return windows.CreateFile(windows.StringToUTF16Ptr("CONIN$"), windows.GENERIC_READ|windows.GENERIC_WRITE, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING, 0, 0)
	}
	if ops != nil {
		if ops.mode != nil {
			getMode = ops.mode
		}
		if ops.open != nil {
			open = ops.open
		}
		if ops.read != nil {
			consoleRead = ops.read
		}
	}
	var console windows.Handle
	var mode uint32
	if getMode(windows.Handle(f.Fd()), &mode) == nil {
		emit("branch-console", windows.Handle(f.Fd()), nil)
		// Output handles also have console modes, potentially identical to input.
		// Classify without consuming input before opening the shared input queue.
		var events uint32
		validationErr := windows.GetNumberOfConsoleInputEvents(windows.Handle(f.Fd()), &events)
		emit("input-validate", windows.Handle(f.Fd()), validationErr)
		if err := validationErr; err != nil {
			// Defer system message formatting until the caller requests Error().
			return "", os.NewSyscallError("validate console input", err)
		}
		var err error
		console, err = open()
		emit("console-acquire", console, err)
		if err != nil {
			return "", fmt.Errorf("prepare console cancellation: %w", err)
		}
		// Registered before the join defer: never close a handle with a live read.
		defer closeOwned("console-close", console)
		var openedMode uint32
		validationErr = getMode(console, &openedMode)
		if validationErr != nil {
			emit("console-validate", console, validationErr)
			return "", os.NewSyscallError("validate console input", validationErr)
		}
		if openedMode != mode {
			err := fmt.Errorf("console input mode changed: inherited=%#x opened=%#x", mode, openedMode)
			emit("console-validate", console, err)
			return "", err
		}
		emit("console-validate", console, nil)
		read = func() (string, error) { return consoleRead(ctx, console, mode) }
	} else {
		emit("branch-stream", windows.Handle(f.Fd()), nil)
	}

	defer runtime.KeepAlive(f)
	type result struct {
		line string
		err  error
	}
	done := make(chan result, 1)
	ready := make(chan windows.Handle, 1)
	release := make(chan struct{})
	finished := make(chan struct{})
	defer func() { close(release); <-finished; emit("join", 0, nil) }()
	go func() {
		defer close(finished)
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		emit("worker-start", 0, nil)
		var thread windows.Handle
		process := windows.CurrentProcess()
		err := duplicate(process, windows.CurrentThread(), process, &thread, 0, false, windows.DUPLICATE_SAME_ACCESS)
		emit("thread-acquire", thread, err)
		if err != nil {
			ready <- 0
			done <- result{err: fmt.Errorf("prepare prompt cancellation: %w", err)}
			return
		}
		// Keep the thread handle alive until cancellation has stopped and the
		// reader is released, before allowing the OS thread to be reused.
		defer closeOwned("thread-close", thread)
		ready <- thread
		emit("read-enter", console, nil)
		line, err := read()
		emit("read-complete", console, err)
		done <- result{line, err}
		<-release
	}()
	thread := <-ready
	select {
	case v := <-done:
		return v.line, v.err
	case <-ctx.Done():
		// Cancellation can race the entry into ReadFile/ReadConsole. Repeat on the
		// owned request (or dedicated non-console thread) until it exits, then join.
		emit("cancel-start", 0, nil)
		defer emit("cancel-complete", 0, nil)
		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()
		for {
			if console != 0 {
				// ERROR_NOT_FOUND is expected when cancellation wins the entry race.
				// Repeat until the read returns; completion, not cancellation success,
				// is the handoff boundary. Input ownership remains exclusive until then.
				emit("console-cancel", console, windows.CancelIoEx(console, nil))
			} else if thread != 0 {
				ok, _, err := cancelSynchronousIO.Call(uintptr(thread))
				if ok != 0 {
					err = nil
				}
				emit("thread-cancel", thread, err)
			}
			select {
			case <-done:
				return "", ctx.Err()
			case <-ticker.C:
			}
		}
	}
}

// ReadConsole's cooked line includes CR/LF. os.File's byte reader translates
// Ctrl+Z to EOF immediately, leaving that terminator for the next console owner.
// Read the complete submitted line ourselves before reporting prompt EOF. The
// console host still owns editing and echo; no console modes are changed.
func readConsoleLine(ctx context.Context, h windows.Handle, mode uint32) (string, error) {
	return readConsoleAnswer(ctx, mode&windows.ENABLE_LINE_INPUT != 0, func(b []uint16) (uint32, error) {
		var n uint32
		err := windows.ReadConsole(h, &b[0], uint32(len(b)), &n, nil)
		if ops, _ := ctx.Value(cancelWindowsKey{}).(*cancelWindowsOps); ops != nil && ops.observe != nil {
			ops.observe(cancelWindowsEvent{operation: "native-read-complete", handle: h, threadID: windows.GetCurrentThreadId(), units: n, err: err})
		}
		return n, err
	})
}

func readConsoleAnswer(ctx context.Context, cooked bool, read func([]uint16) (uint32, error)) (string, error) {
	var units []uint16
	var b [1]uint16 // Never prefetch any character from the next submitted line.
	closed := false
	for {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		n, err := read(b[:])
		if e := ctx.Err(); e != nil {
			return "", e
		}
		if err != nil {
			return "", fmt.Errorf("read prompt: %w", err)
		}
		if n == 0 {
			return "", prompt.ErrPromptInputClosed
		}
		switch b[0] {
		case 0x1a:
			if !cooked {
				return "", prompt.ErrPromptInputClosed
			}
			closed = true
		case '\n':
			if closed {
				return "", prompt.ErrPromptInputClosed
			}
			line := strings.TrimSuffix(string(utf16.Decode(units)), "\r")
			if len(line) > 4096 {
				return "", fmt.Errorf("prompt answer exceeds 4096 bytes")
			}
			return line, nil
		}
		if !closed {
			if len(units) >= 4097 {
				return "", fmt.Errorf("prompt answer exceeds 4096 bytes")
			}
			units = append(units, b[0])
		}
	}
}
