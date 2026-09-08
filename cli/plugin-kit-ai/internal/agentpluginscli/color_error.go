package agentpluginscli

import (
	"io"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/prompt"
	"github.com/777genius/plugin-kit-ai/cli/internal/terminaltheme"
)

// ErrorText uses the same flag parser without executing a command or touching state.
// Malformed flags and unknown commands deliberately keep their diagnostic unstyled.
func ErrorText(args []string, output io.Writer, err error) string {
	text := prompt.SafeText(err.Error())
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
