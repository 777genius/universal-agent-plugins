package agentpluginscli

import (
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/prompt"
)

// Regression: the default saved add used to prompt for targets without a
// prompter and failed with "prompt unavailable" in a real terminal.
func TestSavedAddPromptsForTargetsInTerminal(t *testing.T) {
	j := newSavedJourney(t, t.TempDir())
	stop := errors.New("stop after target prompt")
	prompted := false
	j.cli.app.PromptFactory = func(io.Reader, io.Writer, io.Writer, bool, bool) (prompt.Prompter, io.Writer, error) {
		return fakePrompter{selectFn: func(prompt.TargetSelectionRequest) (prompt.TargetSelectionResult, error) {
			prompted = true
			return prompt.TargetSelectionResult{}, stop
		}}, io.Discard, nil
	}
	_, _, err := j.cli.execute(true, "add", "github:owner/repo@"+strings.Repeat("a", 40))
	if !prompted || !errors.Is(err, stop) {
		t.Fatalf("prompted=%v err=%v", prompted, err)
	}
}
