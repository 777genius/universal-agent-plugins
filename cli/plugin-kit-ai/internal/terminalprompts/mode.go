package terminalprompts

import (
	"io"
	"os"
	"runtime"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/prompt"
	"github.com/777genius/plugin-kit-ai/cli/internal/terminaltheme"
	"golang.org/x/term"
)

type Capabilities struct {
	InputTTY, OutputTTY, ErrorTTY, SizeOK bool
	TERM, GOOS                            string
}
type Mode int

const (
	Unavailable Mode = iota
	Plain
	Rich
)

func ResolveMode(c Capabilities, plain bool) Mode {
	if !c.InputTTY || (!c.OutputTTY && !c.ErrorTTY) {
		return Unavailable
	}
	if plain || !c.OutputTTY || !c.ErrorTTY || !c.SizeOK || c.TERM == "" || c.TERM == "dumb" || c.GOOS == "windows" {
		return Plain
	}
	return Rich
}
func terminal(v any) bool {
	if w, ok := v.(io.Writer); ok {
		v = terminaltheme.Unwrap(w)
	}
	f, ok := v.(*os.File)
	return ok && term.IsTerminal(int(f.Fd()))
}

// New receives the CLI-resolved noColor decision for the visible stream.
// It must not reapply NO_COLOR after explicit flags have overridden it.
func New(input io.Reader, output, errorOutput io.Writer, plain, noColor bool) (prompt.Prompter, io.Writer, error) {
	c := Capabilities{InputTTY: terminal(input), OutputTTY: terminal(output), ErrorTTY: terminal(errorOutput), TERM: os.Getenv("TERM"), GOOS: runtime.GOOS}
	if f, ok := terminaltheme.Unwrap(output).(*os.File); ok {
		w, h, e := term.GetSize(int(f.Fd()))
		c.SizeOK = e == nil && w >= 40 && h >= 10
	}
	mode := ResolveMode(c, plain)
	if mode == Unavailable {
		return nil, nil, prompt.ErrPromptUnavailable
	}
	visible := output
	if !c.OutputTTY {
		visible = errorOutput
	}
	if mode == Rich {
		return HuhPrompter{Input: input, Output: terminaltheme.Unwrap(visible), NoColor: noColor}, visible, nil
	}
	policy := &terminaltheme.Policy{Mode: "always", Explicit: true}
	if noColor {
		policy.Mode = "never"
	}
	format := "human"
	return PlainPrompter{Input: input, Output: terminaltheme.Wrap(terminaltheme.Unwrap(visible), policy, &format)}, visible, nil
}
