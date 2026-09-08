package promptio

import (
	"io"
	"os"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/prompt"
	"golang.org/x/term"
)

// VisibleOutput keeps inherited file-backed questions out of redirected logs.
// Injected non-file writers remain usable by embedders and contract tests.
func VisibleOutput(primary, alternate io.Writer) (io.Writer, error) {
	if f, ok := primary.(*os.File); ok {
		if term.IsTerminal(int(f.Fd())) {
			return primary, nil
		}
		if f, ok := alternate.(*os.File); ok && term.IsTerminal(int(f.Fd())) {
			return alternate, nil
		}
		return nil, prompt.ErrPromptUnavailable
	}
	return primary, nil
}

// WriteText treats a short write as an error before a caller reads consent.
func WriteText(w io.Writer, text string) error {
	n, err := io.WriteString(w, text)
	if err == nil && n != len(text) {
		return io.ErrShortWrite
	}
	return err
}
