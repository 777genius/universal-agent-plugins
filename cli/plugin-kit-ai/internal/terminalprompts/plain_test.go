package terminalprompts

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/prompt"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

type stalled struct{}

func (stalled) Read([]byte) (int, error) { return 0, nil }

type broken struct{}

func (broken) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }
func TestPlainConfirmation(t *testing.T) {
	for _, tc := range []struct {
		input     string
		yes, fail bool
	}{{"\n", false, false}, {"y\r\n", true, false}, {"yes\n", true, false}, {"n\n", false, false}, {"y", false, true}, {"", false, true}, {strings.Repeat("x", 4097) + "\n", false, true}} {
		p := PlainPrompter{strings.NewReader(tc.input), io.Discard}
		r, e := p.Confirm(context.Background(), prompt.ConfirmationRequest{Title: "Apply?"})
		if r.Accepted != tc.yes || (e != nil) != tc.fail {
			t.Fatalf("%q: %+v %v", tc.input, r, e)
		}
	}
	for _, p := range []PlainPrompter{{stalled{}, io.Discard}, {strings.NewReader("y\n"), broken{}}} {
		r, e := p.Confirm(context.Background(), prompt.ConfirmationRequest{})
		if e == nil || r.Accepted {
			t.Fatal(r, e)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	p := PlainPrompter{strings.NewReader("y\n"), io.Discard}
	if _, e := p.Confirm(ctx, prompt.ConfirmationRequest{}); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}
func TestPlainSelectionAndHandoff(t *testing.T) {
	req := prompt.TargetSelectionRequest{Choices: []prompt.TargetChoice{{ID: "cursor", Label: "Cursor"}, {ID: "codex", Label: "Codex"}}, DefaultIDs: []domain.ClientID{"cursor", "codex"}}
	for _, input := range []string{"\n", "2,1\r\n", "1,1\n", "3\n", ""} {
		var out bytes.Buffer
		p := PlainPrompter{strings.NewReader(input + "n\n"), &out}
		r, e := p.SelectTargets(context.Background(), req)
		valid := input == "\n" || input == "2,1\r\n"
		if (e == nil) != valid {
			t.Fatal(input, r, e)
		}
		if valid {
			if len(r.IDs) != 2 || r.IDs[0] != "cursor" {
				t.Fatal(r)
			}
			a, e := p.Confirm(context.Background(), prompt.ConfirmationRequest{})
			if e != nil || a.Accepted {
				t.Fatal(a, e)
			}
		}
	}
}
func TestModePolicy(t *testing.T) {
	base := Capabilities{true, true, true, true, "xterm-256color", "linux"}
	for _, tc := range []struct {
		name   string
		change func(*Capabilities)
		plain  bool
		want   Mode
	}{
		{"rich", func(*Capabilities) {}, false, Rich}, {"plain", func(*Capabilities) {}, true, Plain}, {"pipe", func(c *Capabilities) { c.InputTTY = false }, false, Unavailable}, {"both redirected", func(c *Capabilities) { c.OutputTTY = false; c.ErrorTTY = false }, false, Unavailable}, {"stdout redirected", func(c *Capabilities) { c.OutputTTY = false }, false, Plain}, {"stderr redirected", func(c *Capabilities) { c.ErrorTTY = false }, false, Plain}, {"dumb", func(c *Capabilities) { c.TERM = "dumb" }, false, Plain}, {"unset", func(c *Capabilities) { c.TERM = "" }, false, Plain}, {"tiny", func(c *Capabilities) { c.SizeOK = false }, false, Plain}, {"windows unqualified", func(c *Capabilities) { c.GOOS = "windows" }, false, Plain},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := base
			tc.change(&c)
			if got := ResolveMode(c, tc.plain); got != tc.want {
				t.Fatal(got)
			}
		})
	}
}

type shortWriter struct{}

func (shortWriter) Write([]byte) (int, error) { return 0, nil }

type forbiddenReader struct{ t *testing.T }

func (r forbiddenReader) Read([]byte) (int, error) {
	r.t.Fatal("read after failed prompt output")
	return 0, io.EOF
}
func TestShortPromptOutputCannotAuthorize(t *testing.T) {
	for _, p := range []prompt.Prompter{PlainPrompter{Input: forbiddenReader{t}, Output: shortWriter{}}, HuhPrompter{Input: forbiddenReader{t}, Output: shortWriter{}}} {
		result, err := p.Confirm(context.Background(), prompt.ConfirmationRequest{Title: "Apply?"})
		if result.Accepted || !errors.Is(err, io.ErrShortWrite) {
			t.Fatal(result, err)
		}
	}
}
