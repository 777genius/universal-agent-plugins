package terminalprompts

// These helpers run only under scripts/terminal-ui/qualification_faults.py.
// Inherited terminal descriptors remain owned by the runner throughout.
import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"charm.land/huh/v2"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/prompt"
	"github.com/777genius/plugin-kit-ai/cli/internal/promptio"
)

type qualificationWriter struct {
	armed atomic.Bool
	kind  string
}

func (w *qualificationWriter) Write(p []byte) (int, error) {
	if w.armed.CompareAndSwap(true, false) {
		switch w.kind {
		case "huh-write-error":
			return 0, io.ErrClosedPipe
		case "huh-write-short":
			return 0, nil
		case "huh-render-panic":
			panic("qualification active renderer panic")
		}
	}
	return os.Stdout.Write(p)
}

func TestQualificationTerminalFault(t *testing.T) {
	kind := os.Getenv("UAP_QUALIFICATION_CASE")
	if kind == "" {
		t.Skip("requires isolated native terminal runner")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	writer := &qualificationWriter{kind: kind}
	stop := make(chan struct{})
	joined := make(chan struct{})
	go func() {
		defer close(joined)
		tick := time.NewTicker(time.Millisecond)
		defer tick.Stop()
		for {
			select {
			case <-stop:
				return
			case <-tick.C:
				if _, e := os.Stat(os.Getenv("UAP_QUALIFICATION_TRIGGER")); e == nil {
					if strings.HasSuffix(kind, "cancel") {
						cancel()
					} else {
						writer.armed.Store(true)
					}
					fmt.Fprintln(os.Stderr, "QUALIFICATION_TRIGGERED")
					return
				}
			}
		}
	}()
	defer func() { close(stop); <-joined }()
	var p prompt.Prompter = HuhPrompter{Input: os.Stdin, Output: writer, NoColor: true}
	if kind == "huh-cancel" {
		p = HuhPrompter{Input: os.Stdin, Output: os.Stdout, NoColor: true}
	}
	if kind == "plain-cancel" {
		p = PlainPrompter{Input: os.Stdin, Output: os.Stdout}
	}
	var result prompt.ConfirmationResult
	var err error
	if kind == "huh-init-error" {
		// Call the production form runner directly so the failed output descriptor
		// reaches Tea initialization, rather than failing the preprinted question.
		closed, e := os.Open(os.DevNull)
		if e != nil {
			t.Fatal(e)
		}
		closed.Close()
		field := huh.NewConfirm().Title("Apply qualification?").Value(&result.Accepted)
		err = (HuhPrompter{Input: os.Stdin, Output: closed, NoColor: true}).run(ctx, huh.NewForm(huh.NewGroup(field)))
	} else {
		result, err = p.Confirm(ctx, prompt.ConfirmationRequest{Title: "Apply qualification?"})
	}
	if result.Accepted || err == nil {
		t.Fatalf("consent on fault: %+v, %v", result, err)
	}
	switch kind {
	case "huh-init-error":
		if !errors.Is(err, os.ErrClosed) {
			t.Fatalf("initialization error lost: %v", err)
		}
	case "plain-cancel", "huh-cancel":
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("external cancel: %v", err)
		}
	case "huh-write-error":
		if !errors.Is(err, io.ErrClosedPipe) {
			t.Fatalf("writer error lost: %v", err)
		}
	case "huh-write-short":
		if !errors.Is(err, io.ErrShortWrite) {
			t.Fatalf("short write lost: %v", err)
		}
	case "huh-render-panic":
		if !strings.Contains(err.Error(), "panic") && !strings.Contains(err.Error(), "initialization") {
			t.Fatalf("panic error lost: %v", err)
		}
	}
	fmt.Fprintf(os.Stdout, "\nQUALIFICATION_RETURNED %s %v\n", kind, err)
	// An independent fresh reader on the very same inherited descriptor proves
	// that cleanup joined the old read and left subsequent input available.
	fmt.Fprintln(os.Stdout, "QUALIFICATION_REUSE_READY")
	reuseCtx, reuseCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer reuseCancel()
	line, err := promptio.ReadLine(reuseCtx, os.Stdin)
	if err != nil || line != "qualification-reuse" {
		t.Fatalf("inherited input reuse: %q %v", line, err)
	}
	fmt.Fprintln(os.Stdout, "QUALIFICATION_REUSE_OK")
}
