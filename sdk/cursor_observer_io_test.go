package pluginkitai_test

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	pluginkitai "github.com/777genius/plugin-kit-ai/sdk"
	"github.com/777genius/plugin-kit-ai/sdk/cursor"
)

// This public injected IO boundary deliberately has no closable process handles.
// IO owns its bytes; these tests assert observable writes and supplied contexts.
type cursorCallerIO struct {
	input       []byte
	out         bytes.Buffer
	writes      int
	stderr      string
	readPanic   bool
	outputError bool
	outputPanic bool
	deadline    time.Time
}

func (b *cursorCallerIO) ReadStdin(ctx context.Context) ([]byte, error) {
	if b.readPanic {
		panic("private decoder/transport content")
	}
	b.deadline, _ = ctx.Deadline()
	return b.input, ctx.Err()
}
func (b *cursorCallerIO) WriteStdout(p []byte) error {
	b.writes++
	if b.outputPanic {
		panic("private output content")
	}
	if b.outputError {
		return errors.New("output unavailable")
	}
	_, err := b.out.Write(p)
	return err
}
func (b *cursorCallerIO) WriteStderr(s string) error { b.stderr += s; return nil }

// Red: Config.IO is ignored, dispatch bytes are merged into the neutral response,
// a transport/decode/middleware panic escapes, or a caller-owned handle is closed.
func TestCursorObserverInjectedIO(t *testing.T) {
	for _, mode := range []string{"normal", "read panic", "middleware panic", "output error", "output panic", "canceled"} {
		t.Run(mode, func(t *testing.T) {
			boundary := &cursorCallerIO{input: []byte(cursorInput), readPanic: mode == "read panic", outputError: mode == "output error"}
			boundary.outputPanic = mode == "output panic"
			app := pluginkitai.New(pluginkitai.Config{Args: []string{"TEST", "CursorStop"}, IO: boundary})
			calls := 0
			app.Cursor().OnStopContext(func(ctx context.Context, e *cursor.StopEvent) (*cursor.StopResponse, error) {
				calls++
				if ctx.Err() != nil || e.Status != cursor.StopStatusCompleted {
					t.Fatal("bad callback context/fact")
				}
				return &cursor.StopResponse{}, nil
			})
			if mode == "middleware panic" {
				app.Use(func(pluginkitai.Next) pluginkitai.Next { panic("private middleware content") })
			}
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			if mode == "canceled" {
				cancel()
			}
			code := app.RunCursorObserver(ctx)
			wantCode, wantOutput := 0, "{}\n"
			if mode == "output error" || mode == "output panic" {
				wantCode, wantOutput = 1, ""
			}
			if code != wantCode || boundary.out.String() != wantOutput || boundary.writes != 1 || boundary.stderr != "" {
				t.Fatalf("injected IO: code=%d out=%q writes=%d stderr=%q", code, boundary.out.String(), boundary.writes, boundary.stderr)
			}
			wantCalls := 0
			if mode == "normal" || mode == "output error" || mode == "output panic" {
				wantCalls = 1
			}
			if calls != wantCalls {
				t.Fatalf("callback count = %d", calls)
			}
			if mode == "normal" {
				d, _ := ctx.Deadline()
				if boundary.deadline.After(d.Add(-90 * time.Millisecond)) {
					t.Fatal("no output reserve in callback/input budget")
				}
			}
		})
	}
}

type cursorContextIO struct {
	cursorCallerIO
	outputDeadline time.Time
}

func (b *cursorContextIO) WriteStdoutContext(ctx context.Context, p []byte) error {
	b.outputDeadline, _ = ctx.Deadline()
	return b.WriteStdout(p)
}

// Red: an injected cancellable writer receives the canceled work context and
// cannot emit the fixed response, or caller's earlier deadline is extended.
func TestCursorObserverInjectedOutputContext(t *testing.T) {
	boundary := &cursorContextIO{cursorCallerIO: cursorCallerIO{input: []byte(cursorInput)}}
	app := pluginkitai.New(pluginkitai.Config{Args: []string{"TEST", "CursorStop"}, IO: boundary})
	app.Cursor().OnStopContext(func(ctx context.Context, _ *cursor.StopEvent) (*cursor.StopResponse, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	})
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	d, _ := ctx.Deadline()
	if code := app.RunCursorObserver(ctx); code != 0 || boundary.out.String() != "{}\n" || boundary.outputDeadline.After(d) || time.Now().After(d) {
		t.Fatal("output cancellation/reservation contract changed")
	}
}
