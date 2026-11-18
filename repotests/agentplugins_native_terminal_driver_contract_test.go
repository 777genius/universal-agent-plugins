package pluginkitairepo_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestNativeTerminalDriverOutputRemainsBoundedAndDrains(t *testing.T) {
	for _, size := range []int{0, nativeTerminalDriverLogLimit, 3*nativeTerminalDriverLogLimit + 17} {
		for _, chunked := range []bool{false, true} {
			payload := bytes.Repeat([]byte("x"), size)
			var output nativeTerminalDriverOutput
			var written int64
			var err error
			if chunked {
				// Hide bytes.Reader.WriteTo so Copy exercises repeated pipe-sized writes.
				written, err = io.Copy(&output, struct{ io.Reader }{bytes.NewReader(payload)})
			} else {
				var n int
				n, err = output.Write(payload)
				written = int64(n)
			}
			want := min(size, nativeTerminalDriverLogLimit)
			if err != nil || written != int64(size) || len(output.Bytes()) != want ||
				!bytes.Equal(output.Bytes(), payload[:want]) || output.overflow != (size > want) {
				t.Fatalf("diagnostic did not retain/drain size=%d chunked=%v: written=%d retained=%d overflow=%v err=%v",
					size, chunked, written, len(output.Bytes()), output.overflow, err)
			}
			n, err := output.Write([]byte("tail"))
			if n != 4 || err != nil || len(output.Bytes()) > nativeTerminalDriverLogLimit ||
				output.overflow != (size+4 > nativeTerminalDriverLogLimit) {
				t.Fatal("later diagnostic writes stopped draining or reset overflow")
			}
		}
	}
}

func nativeTerminalInertDriver(t *testing.T, mode, ready string) {
	t.Helper()
	interrupts := make(chan os.Signal, 1)
	switch mode {
	case "handled":
		signal.Notify(interrupts, os.Interrupt)
		defer signal.Stop(interrupts)
	case "ignored":
		signal.Ignore(os.Interrupt)
	default:
		t.Fatal("unknown inert terminal driver mode")
	}
	if err := os.WriteFile(ready, []byte("ready"), 0600); err != nil {
		t.Fatal(err)
	}
	if mode == "handled" {
		<-interrupts
		_, _ = os.Stdout.WriteString("terminal driver cleanup completed\n")
		return
	}
	time.Sleep(30 * time.Second)
	t.Fatal("inert driver survived bounded cancellation")
}

func TestNativeTerminalDriverInterruptAndEscalation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the native keyboard driver and its signal contract use Unix")
	}
	const modeKey = "UAP_INERT_TERMINAL_DRIVER_MODE"
	const readyKey = "UAP_INERT_TERMINAL_DRIVER_READY"
	if mode := os.Getenv(modeKey); mode != "" {
		nativeTerminalInertDriver(t, mode, os.Getenv(readyKey))
		return
	}
	// This is only an inert copy of the Go test binary; no installer or agent runs.
	for _, mode := range []string{"handled", "ignored"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			ready := filepath.Join(t.TempDir(), "ready")
			command := nativeTerminalDriverCommand(ctx, os.Args[0],
				"-test.run=^TestNativeTerminalDriverInterruptAndEscalation$", "-test.count=1")
			command.Env = append(os.Environ(), modeKey+"="+mode, readyKey+"="+ready)
			if command.WaitDelay != 5*time.Second {
				t.Fatal("terminal driver hard-escalation bound changed")
			}
			var output nativeTerminalDriverOutput
			command.Stdout, command.Stderr = &output, &output
			if err := command.Start(); err != nil {
				t.Fatal(err)
			}
			done := make(chan error, 1)
			go func() { done <- command.Wait() }()
			waited := false
			defer func() {
				cancel()
				if !waited {
					<-done
				}
			}()
			ticker := time.NewTicker(10 * time.Millisecond)
			defer ticker.Stop()
			for {
				_, err := os.Stat(ready)
				if err == nil {
					break
				}
				if !errors.Is(err, os.ErrNotExist) {
					t.Fatal(err)
				}
				select {
				case err := <-done:
					waited = true
					t.Fatalf("inert driver exited before readiness: %v\n%s", err, output.Bytes())
				case <-ctx.Done():
					t.Fatal("inert driver readiness exceeded its bound")
				case <-ticker.C:
				}
			}
			cancel()
			err := <-done
			waited = true
			if err == nil || ctx.Err() != context.Canceled || command.ProcessState == nil || output.overflow {
				t.Fatalf("cancellation was lost: %v; context=%v", err, ctx.Err())
			}
			cleaned := bytes.Contains(output.Bytes(), []byte("terminal driver cleanup completed\n"))
			if cleaned != (mode == "handled") {
				t.Fatalf("interrupt did not permit exactly the handled cleanup: %v\n%s", err, output.Bytes())
			}
			if mode == "handled" && !errors.Is(err, context.Canceled) {
				t.Fatalf("successful child cleanup masked the canceled command: %v", err)
			}
			if mode == "ignored" {
				var exited *exec.ExitError
				if !errors.As(err, &exited) || exited.ExitCode() != -1 {
					t.Fatalf("ignored interrupt did not reach bounded hard escalation: %v", err)
				}
			}
		})
	}
}
