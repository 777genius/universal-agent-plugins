package agentpluginscli

import (
	"io"
	"strings"
	"unicode"

	"github.com/777genius/plugin-kit-ai/cli/internal/terminaltheme"
	"github.com/charmbracelet/x/ansi"
)

// ErrorText uses the same flag parser without executing a command or touching state.
// Malformed flags and unknown commands deliberately keep their diagnostic unstyled.
func ErrorText(args []string, output io.Writer, err error) string {
	text := safeDiagnosticText(err.Error())
	root := NewRoot(App{ErrorOutput: output})
	cmd, remaining, findErr := root.Find(args)
	if findErr != nil {
		return text
	}
	if cmd.ParseFlags(remaining) != nil {
		return text
	}
	return terminaltheme.For(root.ErrOrStderr()).Text(terminaltheme.Error, text)
}

// Diagnostics can contain multiline examples and trailing recovery guidance.
// Strip escapes per line so an unterminated sequence cannot consume later lines.
func safeDiagnosticText(text string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = strings.Map(func(r rune) rune {
			if unicode.IsControl(r) || unicode.In(r, unicode.Cf) {
				return -1
			}
			return r
		}, ansi.Strip(line))
	}
	return strings.Join(lines, "\n")
}
