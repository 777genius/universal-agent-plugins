//go:build !linux && !darwin && !freebsd && !openbsd && !netbsd && !dragonfly

package terminalprompts

import (
	"os"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/prompt"
)

func queuedInputBytes(*os.File) (int, error) {
	return 0, prompt.ErrPromptUnavailable
}
