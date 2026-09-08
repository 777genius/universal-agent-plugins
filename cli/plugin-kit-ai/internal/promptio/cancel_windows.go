//go:build windows

package promptio

import (
	"context"
	"fmt"
	"io"
	"os"
	"runtime"
	"time"

	"golang.org/x/sys/windows"
)

var cancelSynchronousIO = windows.NewLazySystemDLL("kernel32.dll").NewProc("CancelSynchronousIo")

// CancelSynchronousIo targets our locked reader thread, not the inherited
// console handle. No CONIN$ is opened and queued console input is not flushed.
func readCancelable(ctx context.Context, r io.Reader) (string, error) {
	if _, ok := r.(*os.File); !ok {
		return readLine(ctx, r)
	}
	type result struct {
		line string
		err  error
	}
	done := make(chan result, 1)
	ready := make(chan windows.Handle, 1)
	release := make(chan struct{})
	finished := make(chan struct{})
	defer func() { close(release); <-finished }()
	go func() {
		defer close(finished)
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		var thread windows.Handle
		process := windows.CurrentProcess()
		if err := windows.DuplicateHandle(process, windows.CurrentThread(), process, &thread, 0, false, windows.DUPLICATE_SAME_ACCESS); err != nil {
			ready <- 0
			done <- result{err: fmt.Errorf("prepare prompt cancellation: %w", err)}
			return
		}
		ready <- thread
		line, err := readLine(ctx, r)
		done <- result{line, err}
		<-release
	}()
	thread := <-ready
	if thread != 0 {
		defer windows.CloseHandle(thread)
	}
	select {
	case v := <-done:
		return v.line, v.err
	case <-ctx.Done():
		// Cancellation can race the entry into ReadFile/ReadConsole. Repeat on the
		// same dedicated thread until that read has exited, then join it.
		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()
		for {
			if thread != 0 {
				_, _, _ = cancelSynchronousIO.Call(uintptr(thread))
			}
			select {
			case <-done:
				return "", ctx.Err()
			case <-ticker.C:
			}
		}
	}
}
