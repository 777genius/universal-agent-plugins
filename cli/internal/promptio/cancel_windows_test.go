//go:build windows

package promptio

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
	"time"
	"unicode/utf16"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/prompt"
)

func TestWindowsConsoleLineBoundary(t *testing.T) {
	for _, tt := range []struct {
		name, input, want string
		closed            bool
	}{
		{"unicode", "中文é😀\r\nnext\r\n", "中文é😀", false},
		{"empty", "\r\nnext\r\n", "", false},
		{"EOF", "\x1a\r\nnext\r\n", "", true},
		{"partial EOF", "yes\x1a\r\nnext\r\n", "", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			queued := utf16.Encode([]rune(tt.input))
			read := func(b []uint16) (uint32, error) {
				if len(queued) == 0 {
					return 0, io.EOF
				}
				n := copy(b, queued)
				queued = queued[n:]
				return uint32(n), nil
			}
			line, err := readConsoleAnswer(context.Background(), true, read)
			if line != tt.want || (tt.closed && !errors.Is(err, prompt.ErrPromptInputClosed)) || (!tt.closed && err != nil) {
				t.Fatalf("first answer: %q %v", line, err)
			}
			// Inspect the native queue, not just a second read through this helper:
			// no residual CR/LF and no next-owner data hidden in a private buffer.
			if got := string(utf16.Decode(queued)); got != "next\r\n" {
				t.Fatalf("remaining input: %q", got)
			}
			line, err = readConsoleAnswer(context.Background(), true, read)
			if line != "next" || err != nil {
				t.Fatalf("reuse: %q %v", line, err)
			}
		})
	}
}

func TestWindowsConsoleCanceledBeforeReadLeavesQueue(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := readConsoleAnswer(ctx, true, func([]uint16) (uint32, error) {
		t.Fatal("read after cancellation")
		return 0, nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestWindowsConsoleReadError(t *testing.T) {
	want := errors.New("console read failed")
	_, err := readConsoleAnswer(context.Background(), true, func([]uint16) (uint32, error) { return 0, want })
	if !errors.Is(err, want) {
		t.Fatal(err)
	}
}

// Real synchronous pipe I/O covers the non-console cancellation path. Console
// cancellation itself must run in TestQualificationConsoleCancellation's ConPTY.
func TestWindowsPipeCancellationEntryAndReuse(t *testing.T) {
	for _, delay := range []time.Duration{0, time.Microsecond, time.Millisecond, 30 * time.Millisecond} {
		t.Run(delay.String(), func(t *testing.T) {
			r, w, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			defer r.Close()
			defer w.Close()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			timer := time.AfterFunc(delay, cancel)
			defer timer.Stop()
			done := make(chan error, 1)
			go func() { _, err := ReadLine(ctx, r); done <- err }()
			select {
			case err := <-done:
				if !errors.Is(err, context.Canceled) {
					t.Fatal(err)
				}
			case <-time.After(time.Second):
				t.Fatal("cancellation did not join reader")
			}
			written := make(chan error, 1)
			go func() { _, err := io.WriteString(w, "next\n"); written <- err }()
			reuse, stop := context.WithTimeout(context.Background(), time.Second)
			defer stop()
			line, err := ReadLine(reuse, r)
			if line != "next" || err != nil {
				t.Fatalf("reuse: %q %v", line, err)
			}
			if err := <-written; err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestWindowsConsoleRawEOFDoesNotWaitForEnter(t *testing.T) {
	calls := 0
	_, err := readConsoleAnswer(context.Background(), false, func(b []uint16) (uint32, error) {
		calls++
		if calls > 1 {
			t.Fatal("raw EOF tried to drain a cooked terminator")
		}
		b[0] = 0x1a
		return 1, nil
	})
	if !errors.Is(err, prompt.ErrPromptInputClosed) {
		t.Fatal(err)
	}
}

func TestWindowsConsoleAnswerByteLimit(t *testing.T) {
	for _, input := range []string{strings.Repeat("a", 4096), strings.Repeat("中", 1366)} {
		queued := utf16.Encode([]rune(input + "\r\n"))
		line, err := readConsoleAnswer(context.Background(), true, func(b []uint16) (uint32, error) {
			if len(queued) == 0 {
				t.Fatal("read beyond submitted line")
			}
			b[0] = queued[0]
			queued = queued[1:]
			return 1, nil
		})
		if len(input) <= 4096 {
			if line != input || err != nil {
				t.Fatalf("boundary: length=%d err=%v", len(line), err)
			}
		} else if err == nil || line != "" {
			t.Fatalf("oversized UTF-8 answer accepted: %d %v", len(line), err)
		}
	}
}
