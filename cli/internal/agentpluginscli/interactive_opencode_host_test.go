package agentpluginscli

import (
	"io"
	"slices"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/prompt"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// Regression: read-only detection never qualifies an OpenCode host, so the
// direct interactive add hid OpenCode for every package with Skills or MCP
// and blamed the publisher. Selection must offer it without running the
// client; qualification happens only after the user selected it.
func TestDirectInteractiveAddOffersUnqualifiedOpenCodeAndQualifiesAfterSelection(t *testing.T) {
	fixture, preparer, plugin, _ := openCodePreparationCLI(t, true)
	offered := false
	fixture.app.PromptFactory = func(io.Reader, io.Writer, io.Writer, bool, bool) (prompt.Prompter, io.Writer, error) {
		return fakePrompter{selectFn: func(request prompt.TargetSelectionRequest) (prompt.TargetSelectionResult, error) {
			if preparer.calls != 0 || preparer.probes != 0 {
				t.Fatal("native host was prepared before target selection")
			}
			offered = slices.ContainsFunc(request.Choices, func(choice prompt.TargetChoice) bool { return choice.ID == domain.ClientOpenCode })
			return prompt.TargetSelectionResult{IDs: []domain.ClientID{domain.ClientOpenCode}}, nil
		}, confirmFn: func() (prompt.ConfirmationResult, error) {
			return prompt.ConfirmationResult{Accepted: true}, nil
		}}, io.Discard, nil
	}
	if _, stderr, err := fixture.executeInput(true, "", "add", plugin); err != nil {
		t.Fatalf("interactive add failed: %v %s", err, stderr)
	}
	if !offered {
		t.Fatal("unqualified OpenCode was not offered")
	}
	if preparer.probes == 0 {
		t.Fatal("selected OpenCode was installed without host qualification")
	}
	state, err := fixture.store.Load()
	if err != nil || len(state.Installations) != 1 || !installationHasTarget(state.Installations[0], domain.ClientOpenCode, "user") {
		t.Fatalf("selected OpenCode was not installed: %+v %v", state, err)
	}
}
