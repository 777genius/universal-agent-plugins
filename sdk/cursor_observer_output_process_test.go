//go:build linux || darwin

package pluginkitai_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"testing"
	"time"
)

// Red: the fixed response blocks indefinitely on a full inherited stdout pipe,
// SIGPIPE terminates a broken-output consumer, or returning abandons IO workers.
// Completion is observed AFTER RunCursorObserver in the independent process.
func TestCursorObserverOutputPipes(t *testing.T) {
	binary := buildCursorConsumer(t)
	for _, full := range []bool{false, true} {
		name := "broken"
		if full {
			name = "full"
		}
		t.Run(name, func(t *testing.T) {
			r, w, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = r.Close() }()
			defer func() { _ = w.Close() }()
			if full {
				if err := w.SetWriteDeadline(time.Now().Add(20 * time.Millisecond)); err != nil {
					t.Fatal(err)
				}
				// A write much larger than an anonymous pipe fills its entire capacity.
				// The parent keeps the reader open without draining until child return.
				n, err := w.Write(make([]byte, 2<<20))
				if n == 0 || !os.IsTimeout(err) {
					t.Fatalf("could not fill output pipe: n=%d err=%v", n, err)
				}
				if err := w.SetWriteDeadline(time.Time{}); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := r.Close(); err != nil {
					t.Fatal(err)
				}
			}
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, binary, "CursorStop", "normal")
			cmd.Env = append(os.Environ(), "GORACE=atexit_sleep_ms=0")
			cmd.Stdin = bytes.NewBufferString(cursorInput)
			cmd.Stdout = w
			var diag bytes.Buffer
			cmd.Stderr = &diag
			start := time.Now()
			err = cmd.Run()
			var exit *exec.ExitError
			if !errors.As(err, &exit) || exit.ExitCode() != 1 || ctx.Err() != nil || time.Since(start) > time.Second {
				t.Fatalf("output failure process result: %v time=%v stderr=%q", err, time.Since(start), diag.String())
			}
			records := cursorRecords(t, diag.String())
			if len(records) != 2 || records[0].Kind != "cursor" || !records[1].Clean || records[1].Code != 1 {
				t.Fatalf("output failure misreported or leaked: %+v", records)
			}
		})
	}
}

// Red: distinct owned pipes with valid Stop input and EOF cause SIGPIPE when
// unrelated fd 0/1 are closed, losing the callback/return record and cleanup.
func TestCursorObserverOwnedBrokenPipes(t *testing.T) {
	binary := buildCursorConsumer(t)
	for _, mode := range []string{"owned-broken", "low-fd-broken"} {
		t.Run(mode, func(t *testing.T) {
			code, out, records := runCursorConsumer(t, binary, "CursorStop", mode, cursorInput)
			if code != 1 || out != "" || len(records) != 2 || records[0].Kind != "cursor" || records[0].Event.ConversationID != "TEST_CONVERSATION" || !records[1].Clean || records[1].Code != 1 {
				t.Fatalf("owned broken output: exit=%d stdout=%q records=%+v", code, out, records)
			}
		})
	}
}
