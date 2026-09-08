//go:build windows

package promptio

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"
	"runtime/debug"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Diagnostic only: run AFTER the strict qualification, in a fresh process for
// each first formatter. Never include this probe in the strict census process:
// formatting here could hide the very first-call acquisition under investigation.
func TestQualificationConsoleErrorResourceProbe(t *testing.T) {
	if os.Getenv("UAP_QUALIFICATION_CASE") != "windows-console-error-resource" {
		t.Skip("requires separate native console diagnostic process")
	}
	first := os.Getenv("UAP_QUALIFICATION_ERROR_FIRST")
	if first != "errno" && first != "format-message" && first != "wrap" {
		t.Fatal("UAP_QUALIFICATION_ERROR_FIRST must be errno, format-message, or wrap")
	}
	oldGC := debug.SetGCPercent(-1)
	defer debug.SetGCPercent(oldGC)
	// Buffer every census until the measured work is complete. No t.Log, error
	// formatting, clock reads, sleeps, or settling polls inside a census sample.
	type sample struct {
		stage      string
		handles    uint32
		goroutines int
	}
	var samples [32]sample
	n := 0
	take := func(stage string) {
		s := &samples[n]
		s.stage = stage
		ok, _, err := qualificationGetProcessHandleCount.Call(uintptr(windows.CurrentProcess()), uintptr(unsafe.Pointer(&s.handles)))
		if ok == 0 {
			t.Fatalf("diagnostic census failed: %v", err)
		}
		s.goroutines = runtime.NumGoroutine()
		n++
	}
	take("entry")
	take("census-only-control")
	output, err := windows.CreateFile(windows.StringToUTF16Ptr("CONOUT$"),
		windows.GENERIC_READ|windows.GENERIC_WRITE,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE,
		nil, windows.OPEN_EXISTING, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if output != 0 {
			if err := windows.CloseHandle(output); err != nil {
				t.Errorf("cleanup output: %v", err)
			}
		}
	}()
	take("CONOUT-open")
	var mode uint32
	if err := windows.GetConsoleMode(output, &mode); err != nil {
		t.Fatal(err)
	}
	take("GetConsoleMode")
	started := time.Now()
	take("time.Now")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	take("context.WithTimeout")
	var events uint32
	raw := windows.GetNumberOfConsoleInputEvents(output, &events)
	take("raw-GetNumberOfConsoleInputEvents")
	// The type assertion and numeric comparison must not call Error().
	errno, isErrno := raw.(syscall.Errno)
	if !isErrno || errno != windows.ERROR_INVALID_HANDLE {
		t.Fatalf("unexpected raw rejection: %T %v", raw, raw)
	}
	take("raw-identity-check")
	var message string
	var wrapped error
	var messageBuffer [300]uint16
	format := func(which string) {
		switch which {
		case "errno":
			message = errno.Error()
			take("errno.Error")
		case "format-message":
			// Match Go 1.25.13 Errno.error exactly: same syscall entry point,
			// flags, 300 UTF-16 units, US English, then language-0 fallback.
			flags := uint32(syscall.FORMAT_MESSAGE_FROM_SYSTEM | syscall.FORMAT_MESSAGE_ARGUMENT_ARRAY | syscall.FORMAT_MESSAGE_IGNORE_INSERTS)
			_, formatErr := syscall.FormatMessage(flags, 0, uint32(errno), 0x0409, messageBuffer[:], nil)
			take("FormatMessageW-en-US")
			if formatErr != nil {
				_, formatErr = syscall.FormatMessage(flags, 0, uint32(errno), 0, messageBuffer[:], nil)
				take("FormatMessageW-language-0")
			}
			if formatErr != nil {
				t.Fatalf("FormatMessageW failed: %v", formatErr)
			}
		case "wrap":
			wrapped = fmt.Errorf("validate console input: %w", raw)
			take("fmt.Errorf-errno")
		}
	}
	format(first)
	for _, which := range []string{"errno", "format-message", "wrap"} {
		if which != first {
			format(which)
		}
	}
	cancel()
	take("context.cancel")
	elapsed := time.Since(started)
	take("time.Since")
	identityOK := errors.Is(wrapped, windows.ERROR_INVALID_HANDLE)
	take("errors.Is-wrapped")
	if err := windows.CloseHandle(output); err != nil {
		t.Fatal(err)
	}
	closed := output
	output = 0
	take("CONOUT-close")
	var handleFlags uint32
	ok, _, closeErr := qualificationGetHandleInformation.Call(uintptr(closed), uintptr(unsafe.Pointer(&handleFlags)))
	take("closed-handle-verification")
	// The sole intentional logging boundary is measured separately; stage rows
	// are emitted only after its trailing census has already been captured.
	t.Log("error-resource diagnostic logging boundary")
	take("t.Log")
	for i, s := range samples[:n] {
		var delta int64
		if i > 0 {
			delta = int64(s.handles) - int64(samples[i-1].handles)
		}
		t.Logf("error-resource first=%s stage=%s handles=%d delta=%+d goroutines=%d", first, s.stage, s.handles, delta, s.goroutines)
	}
	runtime.KeepAlive(message)
	runtime.KeepAlive(messageBuffer)
	if !identityOK || ctx.Err() != context.Canceled {
		t.Errorf("wrapped identity=%t context=%v elapsed=%s", identityOK, ctx.Err(), elapsed)
	}
	if ok != 0 || !errors.Is(closeErr, windows.ERROR_INVALID_HANDLE) {
		t.Errorf("output closure not verified: ok=%d error=%v", ok, closeErr)
	}
	// Observations are not a release ceiling or proof of native handle ownership.
}
