//go:build windows && terminaldiagnostic

package promptio

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

// TestDiagnosticConsoleReads requires the disposable ConPTY diagnostic driver.
// Temporary investigation: remove or convert after the root cause is proven.
// Each control gets a fresh console. It deliberately does not change modes,
// flush input, inject input, retry a failed read, or relax qualification bounds.
func TestDiagnosticConsoleReads(t *testing.T) {
	kind := os.Getenv("UAP_CONSOLE_DIAGNOSTIC")
	if kind == "" {
		t.Skip("requires isolated native ConPTY diagnostic driver")
	}
	// The driver supplies a fresh HOME and test working directory per control.
	// Keep XDG consumers in that disposable tree as well.
	for _, key := range []string{"XDG_CONFIG_HOME", "XDG_CACHE_HOME", "XDG_DATA_HOME", "XDG_STATE_HOME", "XDG_RUNTIME_DIR"} {
		dir := filepath.Join(os.Getenv("HOME"), key)
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		t.Setenv(key, dir)
	}
	switch kind {
	case "consecutive", "pre-canceled", "timed-cancel", "native-inherited", "native-duplicate":
	default:
		t.Fatalf("unknown diagnostic control %q", kind)
	}
	h := windows.Handle(os.Stdin.Fd())
	var before uint32
	if err := windows.GetConsoleMode(h, &before); err != nil {
		t.Fatal(err)
	}
	trace, err := os.Create(os.Getenv("UAP_CONSOLE_TRACE"))
	if err != nil {
		t.Fatal(err)
	}
	defer trace.Close()
	log := func(format string, args ...any) {
		fmt.Fprintf(trace, format+"\n", args...)
	}
	queue := func(i int, phase string) {
		// This does not consume input. Event counts do not include every
		// character held in the console host's cooked-line/private buffers.
		var n uint32
		err := windows.GetNumberOfConsoleInputEvents(h, &n)
		log("iteration=%d phase=%s input_events=%d err=%v", i, phase, n, err)
	}
	log("control=%s inherited=%d mode=%#x", kind, h, before)
	for i := 0; i < 30; i++ {
		if kind == "pre-canceled" || kind == "timed-cancel" {
			ctx, cancel := context.WithCancel(context.Background())
			delay := []time.Duration{0, time.Microsecond, 100 * time.Microsecond, time.Millisecond, 10 * time.Millisecond, 80 * time.Millisecond}[i%6]
			var timer *time.Timer
			if kind == "pre-canceled" {
				cancel()
			} else {
				timer = time.AfterFunc(delay, cancel)
			}
			started := time.Now()
			line, err := ReadLine(ctx, os.Stdin)
			elapsed := time.Since(started)
			if timer != nil {
				timer.Stop()
			}
			cancel()
			log("iteration=%d cancel delay=%s elapsed=%s line=%q err=%v", i, delay, elapsed, line, err)
			queue(i, "after-cancel")
			if line != "" || !errors.Is(err, context.Canceled) || elapsed > time.Second {
				t.Fatalf("iteration %d cancellation: %q %v elapsed=%s", i, line, err, elapsed)
			}
		}
		fmt.Fprintf(os.Stdout, "DIAGNOSTIC_REUSE_READY %d\n", i)
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		started := time.Now()
		var line string
		if kind == "native-inherited" || kind == "native-duplicate" {
			readHandle := h
			if kind == "native-duplicate" {
				if err := windows.DuplicateHandle(windows.CurrentProcess(), h, windows.CurrentProcess(), &readHandle, 0, false, windows.DUPLICATE_SAME_ACCESS); err != nil {
					cancel()
					t.Fatal(err)
				}
			}
			// Native controls have no cancellation before these reads. The
			// watchdog cancels only on failure, and is joined before close/reuse.
			completed := make(chan struct{})
			joined := make(chan struct{})
			go func() {
				defer close(joined)
				<-ctx.Done()
				for {
					select {
					case <-completed:
						return
					default:
					}
					_ = windows.CancelIoEx(readHandle, nil)
					time.Sleep(10 * time.Millisecond)
				}
			}()
			line, err = readConsoleAnswer(ctx, before&windows.ENABLE_LINE_INPUT != 0, func(b []uint16) (uint32, error) {
				log("iteration=%d enter handle=%d capacity=%d", i, readHandle, len(b))
				var n uint32
				err := windows.ReadConsole(readHandle, &b[0], uint32(len(b)), &n, nil)
				log("iteration=%d return n=%d units=%04x err=%v", i, n, b[:n], err)
				return n, err
			})
			close(completed)
			cancel()
			<-joined
			if kind == "native-duplicate" {
				if closeErr := windows.CloseHandle(readHandle); closeErr != nil {
					t.Fatal(closeErr)
				}
			}
		} else {
			line, err = ReadLine(ctx, os.Stdin)
		}
		cancel()
		log("iteration=%d reuse elapsed=%s line=%q err=%v", i, time.Since(started), line, err)
		queue(i, "after-reuse")
		if err != nil || line != fmt.Sprintf("diagnostic-reuse-%d", i) {
			t.Fatalf("iteration %d reuse: %q %v", i, line, err)
		}
		var after uint32
		if err := windows.GetConsoleMode(h, &after); err != nil || before != after {
			t.Fatalf("console mode/handle changed: %d %d %v", before, after, err)
		}
	}
	fmt.Fprintln(os.Stdout, "DIAGNOSTIC_CONSOLE_OK")
}
