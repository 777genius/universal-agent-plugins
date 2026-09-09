package terminalprompts

import (
	"context"
	"errors"
	"io"
	"os"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/prompt"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestFormWindowSizeMessages(t *testing.T) {
	// Only the size-query command is consumed. In particular, messages carrying
	// slices must pass through without an interface-comparison panic.
	for _, msg := range []tea.Msg{nil, tea.QuitMsg{}, tea.WindowSizeMsg{Width: 80, Height: 24}, tea.BatchMsg{}} {
		got, err := formWindowSize(io.Discard, msg)
		if err != nil || !reflect.DeepEqual(got, msg) {
			t.Fatalf("forward %T: got %T, %v", msg, got, err)
		}
	}
	f, err := os.CreateTemp(t.TempDir(), "redirected")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	for _, output := range []io.Writer{io.Discard, f} {
		if got, err := formWindowSize(output, tea.RequestWindowSize()); err != nil || got != nil {
			t.Fatalf("nonterminal %T: got %v, %v", output, got, err)
		}
	}
}

// Both implementations must discard values when canceled, closed or unable to
// show their question. PTY tests separately exercise rich keyboard submission.
func TestAdapterErrorContract(t *testing.T) {
	req := prompt.TargetSelectionRequest{Choices: []prompt.TargetChoice{{ID: "cursor", Label: "Cursor"}}, DefaultIDs: []domain.ClientID{"cursor"}}
	for _, name := range []string{"plain", "huh"} {
		t.Run(name, func(t *testing.T) {
			for _, kind := range []string{"canceled", "writer", "eof"} {
				t.Run(kind, func(t *testing.T) {
					ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
					defer cancel()
					var input io.Reader = strings.NewReader("")
					var output io.Writer = io.Discard
					if kind == "canceled" {
						cancel()
					}
					if kind == "writer" {
						output = broken{}
					}
					var p prompt.Prompter = PlainPrompter{input, output}
					if name == "huh" {
						p = HuhPrompter{Input: input, Output: output}
					}
					result, err := p.SelectTargets(ctx, req)
					if err == nil || result.IDs != nil {
						t.Fatal(result, err)
					}
					accepted, err := p.Confirm(ctx, prompt.ConfirmationRequest{Title: "Apply?"})
					if err == nil || accepted.Accepted {
						t.Fatal(accepted, err)
					}
				})
			}
		})
	}
}
func TestHuhKeysRequireSubmitAndCancel(t *testing.T) {
	km := promptKeyMap()
	for _, code := range []rune{tea.KeyEscape, 'c', 'd'} {
		k := tea.Key{Code: code}
		if code != '\x1b' {
			k.Mod = tea.ModCtrl
		}
		if !key.Matches(tea.KeyPressMsg(k), km.Quit) {
			t.Fatalf("missing cancel %v", k)
		}
	}
	if km.Confirm.Accept.Enabled() || km.Confirm.Reject.Enabled() {
		t.Fatal("immediate consent shortcuts enabled")
	}
}
func TestSubmissionReaderPreservesQueuedAnswer(t *testing.T) {
	input := strings.NewReader("\rnext\n")
	r := newSubmissionReader(context.Background(), input)
	var b [8]byte
	if n, e := r.Read(b[:]); n != 1 || e != nil || b[0] != '\r' {
		t.Fatal(n, e)
	}
	done := make(chan error, 1)
	go func() { _, e := r.Read(b[:]); done <- e }()
	r.finish()
	select {
	case e := <-done:
		if e == nil {
			t.Fatal("read crossed submit")
		}
	case <-time.After(time.Second):
		t.Fatal("read hung")
	}
	rest, _ := io.ReadAll(input)
	if string(rest) != "next\n" {
		t.Fatal(string(rest))
	}
}
func TestSubmissionReaderRejectAndPaste(t *testing.T) {
	r := newSubmissionReader(context.Background(), strings.NewReader("\r\x1b[200~y\nn\n\x1b[201~\r"))
	var b [1]byte
	_, _ = r.Read(b[:])
	r.reject()
	var got strings.Builder
	for i := 0; i < len("\x1b[200~y\nn\n\x1b[201~\r"); i++ {
		_, e := r.Read(b[:])
		if e != nil {
			t.Fatal(e)
		}
		got.WriteByte(b[0])
	}
	if got.String() != "\x1b[200~y\nn\n\x1b[201~\r" {
		t.Fatal(got.String())
	}
	r.finish()
}
func TestHuhCancelBlockedFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("rich Windows mode is disabled pending native qualification")
	}
	r, w, e := os.Pipe()
	if e != nil {
		t.Fatal(e)
	}
	defer r.Close()
	defer w.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	p := HuhPrompter{Input: r, Output: io.Discard}
	result, e := p.Confirm(ctx, prompt.ConfirmationRequest{Title: "Apply?"})
	if result.Accepted || !errors.Is(e, context.DeadlineExceeded) {
		t.Fatal(result, e)
	}
}

func TestHuhOutputFailureCancelsBlockedInput(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("rich Windows mode is disabled pending native qualification")
	}
	r, w, e := os.Pipe()
	if e != nil {
		t.Fatal(e)
	}
	defer r.Close()
	defer w.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	p := HuhPrompter{Input: r, Output: broken{}}
	req := prompt.TargetSelectionRequest{Choices: []prompt.TargetChoice{{ID: "cursor", Label: "Cursor"}}, DefaultIDs: []domain.ClientID{"cursor"}}
	result, e := p.SelectTargets(ctx, req)
	if result.IDs != nil || !errors.Is(e, io.ErrClosedPipe) {
		t.Fatal(result, e)
	}
}

// The preprinted question succeeds; a later renderer write fails with input
// still held open. The adapter must surface that failure and join its reader.
type lateBrokenWriter struct{ writes int }

func (w *lateBrokenWriter) Write(p []byte) (int, error) {
	w.writes++
	if w.writes > 1 {
		return 0, io.ErrClosedPipe
	}
	return len(p), nil
}
func TestHuhRendererFailureAfterQuestion(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("rich Windows disabled")
	}
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	out := &lateBrokenWriter{}
	result, err := (HuhPrompter{Input: r, Output: out}).Confirm(ctx, prompt.ConfirmationRequest{Title: "Apply?"})
	if result.Accepted || !errors.Is(err, io.ErrClosedPipe) || out.writes < 2 {
		t.Fatal(result, err, out.writes)
	}
	if _, err := w.Write([]byte("next\n")); err != nil {
		t.Fatal("inherited input closed:", err)
	}
}
func TestHuhClosedInputInitializationFails(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("rich Windows disabled")
	}
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	r.Close()
	defer w.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	result, err := (HuhPrompter{Input: r, Output: io.Discard}).Confirm(ctx, prompt.ConfirmationRequest{Title: "Apply?"})
	if err == nil || result.Accepted || errors.Is(err, context.DeadlineExceeded) {
		t.Fatal(result, err)
	}
}

// A renderer may call Write on its own goroutine. Recovery must happen there,
// and a one-shot failure must not prevent the later terminal cleanup writes.
type panicOnceWriter struct {
	calls int
}

func (w *panicOnceWriter) Write(p []byte) (int, error) {
	w.calls++
	if w.calls == 1 {
		panic("synthetic-secret-token\x1b[31m")
	}
	return len(p), nil
}

func TestFormWriterPanicCancelsAndAllowsCleanup(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	output := &panicOnceWriter{}
	writer := &formWriter{Writer: output, cancel: cancel}
	done := make(chan struct{})
	go func() {
		defer close(done)
		n, err := writer.Write([]byte("render"))
		if n != 0 || err == nil || err.Error() != "terminal output writer panicked" {
			t.Errorf("panic write = %d, %v", n, err)
		}
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("writer panic did not return")
	}
	if ctx.Err() != context.Canceled {
		t.Fatal("writer panic did not cancel form")
	}
	first := writer.Err()
	if first == nil || strings.Contains(first.Error(), "synthetic-secret") {
		t.Fatalf("unsafe stored error: %v", first)
	}
	if n, err := writer.Write([]byte("restore cursor")); n != len("restore cursor") || err != nil {
		t.Fatalf("cleanup write = %d, %v", n, err)
	}
	if writer.Err() != first {
		t.Fatal("cleanup lost original error")
	}
}

func TestFormWriterKeepsFirstWriteErrorAfterPanic(t *testing.T) {
	_, cancel := context.WithCancel(context.Background())
	defer cancel()
	writer := &formWriter{Writer: broken{}, cancel: cancel}
	if _, err := writer.Write([]byte("render")); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatal(err)
	}
	writer.Writer = &panicOnceWriter{}
	if _, err := writer.Write([]byte("cleanup")); err == nil {
		t.Fatal("panic did not return an error")
	}
	if !errors.Is(writer.Err(), io.ErrClosedPipe) {
		t.Fatalf("original write error replaced: %v", writer.Err())
	}
}
