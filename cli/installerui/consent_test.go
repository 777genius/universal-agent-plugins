package installerui_test

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/installerui"
)

type shortWriter struct{}

func (shortWriter) Write([]byte) (int, error) { return 0, nil }

type forbiddenReader struct{ t *testing.T }

func (r forbiddenReader) Read([]byte) (int, error) {
	r.t.Fatal("input consumed after failed output")
	return 0, io.EOF
}

type stalled struct{}

func (stalled) Read([]byte) (int, error) { return 0, nil }

func TestConsentRequiresCompleteBoundedAnswer(t *testing.T) {
	for _, input := range []io.Reader{strings.NewReader("yes"), strings.NewReader(strings.Repeat("x", 4097) + "\n"), stalled{}} {
		u, _ := installerui.New(installerui.Config{Input: input, Output: io.Discard})
		got, err := u.Confirm(context.Background(), installerui.ConfirmRequest{Title: "Apply?"})
		if err == nil || got.Accepted {
			t.Fatalf("got=%+v err=%v", got, err)
		}
	}
}

func TestShortOutputDoesNotReadConsent(t *testing.T) {
	u, _ := installerui.New(installerui.Config{Input: forbiddenReader{t}, Output: shortWriter{}})
	got, err := u.Confirm(context.Background(), installerui.ConfirmRequest{Title: "Apply?"})
	if !errors.Is(err, io.ErrShortWrite) || got.Accepted {
		t.Fatalf("got=%+v err=%v", got, err)
	}
}

func TestSequentialInstancesDoNotPrefetchAndHonorDefault(t *testing.T) {
	in := strings.NewReader("\nno\n")
	var out strings.Builder
	for i, want := range []bool{true, false} {
		u, _ := installerui.New(installerui.Config{Input: in, Output: &out})
		got, err := u.Confirm(context.Background(), installerui.ConfirmRequest{Title: "Apply?", Default: true})
		if err != nil || got.Accepted != want {
			t.Fatalf("step %d: got=%+v err=%v", i, got, err)
		}
	}
	if !strings.Contains(out.String(), "[Y/n]") {
		t.Fatal(out.String())
	}
}

func TestCancelledContextDoesNotRender(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var out strings.Builder
	u, _ := installerui.New(installerui.Config{Input: forbiddenReader{t}, Output: &out})
	_, err := u.Confirm(ctx, installerui.ConfirmRequest{Title: "Apply?"})
	if !errors.Is(err, context.Canceled) || out.Len() != 0 {
		t.Fatalf("err=%v output=%q", err, out.String())
	}
}
