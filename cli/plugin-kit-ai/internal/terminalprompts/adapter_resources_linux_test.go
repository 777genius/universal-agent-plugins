package terminalprompts

import (
	"context"
	"errors"
	"io"
	"os"
	"runtime"
	"runtime/debug"
	"runtime/pprof"
	"strings"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/prompt"
	"github.com/777genius/plugin-kit-ai/cli/internal/promptio"
)

// A Linux pipe exercises owned cancel-reader descriptors and goroutine joining.
// This is a resource census, not a substitute for native PTY mode restoration
// or a Windows console HANDLE census. No fixture touches a real user profile.
func TestAdapterRepeatedSessionResources(t *testing.T) {
	oldGC := debug.SetGCPercent(-1) // Finalizers must not hide leaked descriptors.
	defer debug.SetGCPercent(oldGC)
	for _, adapter := range []string{"plain", "huh"} {
		t.Run(adapter, func(t *testing.T) {
			cycle := func() {
				for _, kind := range []string{"success", "cancel", "writer", "eof"} {
					resourceSession(t, adapter, kind)
				}
			}
			cycle() // Warm runtime poller / renderer initialization before the census.
			time.Sleep(50 * time.Millisecond)
			fds, goroutines := resourceCounts(t)
			t.Logf("baseline: fd=%d goroutines=%d", fds, goroutines)
			for batch := 1; batch <= 3; batch++ {
				for i := 0; i < 10; i++ {
					cycle()
				}
				deadline := time.Now().Add(3 * time.Second)
				for {
					gotFD, gotGo := resourceCounts(t)
					if gotFD <= fds && gotGo <= goroutines {
						t.Logf("after %d sessions: fd=%d goroutines=%d", batch*40, gotFD, gotGo)
						break
					}
					if time.Now().After(deadline) {
						var stacks strings.Builder
						_ = pprof.Lookup("goroutine").WriteTo(&stacks, 2)
						t.Fatalf("resource growth: baseline fd=%d goroutines=%d; now fd=%d goroutines=%d\n%s", fds, goroutines, gotFD, gotGo, stacks.String())
					}
					time.Sleep(10 * time.Millisecond)
				}
			}
		})
	}
}

func resourceCounts(t *testing.T) (int, int) {
	t.Helper()
	entries, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		t.Fatal(err)
	}
	return len(entries), runtime.NumGoroutine()
}

type resourceFaultWriter struct {
	calls  int
	kind   string
	cancel context.CancelFunc
}

func (w *resourceFaultWriter) Write(p []byte) (int, error) {
	w.calls++
	// Huh's first write is the question, the second reaches its form renderer.
	if w.calls == 2 {
		if w.kind == "cancel" {
			w.cancel()
		}
		if w.kind == "writer" {
			return 0, io.ErrClosedPipe
		}
	}
	return len(p), nil
}

func resourceSession(t *testing.T, adapter, kind string) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	var output io.Writer = io.Discard
	if kind == "success" {
		keys := "\n"
		if adapter == "huh" {
			keys = "\r"
		}
		if _, err := io.WriteString(w, keys); err != nil {
			t.Fatal(err)
		}
	}
	if kind == "eof" {
		if err := w.Close(); err != nil {
			t.Fatal(err)
		}
	}
	if kind == "cancel" || kind == "writer" {
		fault := &resourceFaultWriter{kind: kind, cancel: cancel}
		if adapter == "plain" {
			fault.calls = 1
		}
		output = fault
	}
	var p prompt.Prompter = PlainPrompter{Input: r, Output: output}
	if adapter == "huh" {
		p = HuhPrompter{Input: r, Output: output, NoColor: true}
	}
	got, err := p.Confirm(ctx, prompt.ConfirmationRequest{Title: "Resource fixture?"})
	var want error
	switch kind {
	case "cancel":
		want = context.Canceled
	case "writer":
		want = io.ErrClosedPipe
	case "eof":
		want = prompt.ErrPromptInputClosed
	}
	if got.Accepted || !errors.Is(err, want) {
		t.Fatalf("%s/%s: result=%+v err=%v want=%v", adapter, kind, got, err, want)
	}
	if _, err := r.Stat(); err != nil {
		t.Fatal("inherited input closed:", err)
	}
	if kind != "eof" {
		if _, err := io.WriteString(w, "reuse\n"); err != nil {
			t.Fatal(err)
		}
		reuseCtx, reuseCancel := context.WithTimeout(context.Background(), time.Second)
		defer reuseCancel()
		line, err := promptio.ReadLine(reuseCtx, r)
		if err != nil || line != "reuse" {
			t.Fatalf("%s/%s reuse: %q %v", adapter, kind, line, err)
		}
	}
}
