package agentpluginscli

import (
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestSharedGroupHumanReviewIdentifiesLogicalTargets(t *testing.T) {
	for _, order := range [][]domain.ClientID{{domain.ClientCopilot, domain.ClientVSCode}, {domain.ClientVSCode, domain.ClientCopilot}} {
		t.Run(string(order[0]), func(t *testing.T) {
			first, second := fixtureClient(t, order[0]), fixtureClient(t, order[1])
			if first.ClientID == domain.ClientCopilot {
				first.ExecutablePath = "/test/bin/copilot"
			} else {
				second.ExecutablePath = "/test/bin/copilot"
			}
			f := newCLIFixture(t, []domain.DetectedClient{first, second})
			out, _, err := f.executeInput(true, "\nn\n", "add", writeCLIPlugin(t))
			if err != nil {
				t.Fatal(err)
			}
			for _, target := range []string{"copilot", "vscode"} {
				if strings.Count(out, "Target: "+target+"\n") != 1 {
					t.Fatalf("selected target missing or repeated: %s", out)
				}
			}
			if !strings.Contains(out, "Uses shared physical binding owned by ") ||
				strings.Count(out, "Authentication: not_checked") != 2 || !strings.Contains(out, "Installation not applied") {
				t.Fatal(out)
			}
			state, err := f.store.Load()
			if err != nil || len(state.Installations) != 0 {
				t.Fatalf("declined review mutated state: %+v, %v", state, err)
			}
		})
	}
}

func TestGroupCollisionErrorIncludesSelectedTargetContext(t *testing.T) {
	f := newCLIFixture(t, []domain.DetectedClient{fixtureClient(t, domain.ClientCursor), fixtureClient(t, domain.ClientKiro)})
	f.app.Lifecycle.NativeObserver = selectiveNativeObserver{foreign: domain.ClientCursor}
	_, _, err := f.execute(false, "add", writeCLIPlugin(t), "--target", "cursor,kiro")
	if err == nil || !strings.Contains(err.Error(), "selected targets: [cursor kiro]") ||
		!strings.Contains(err.Error(), "unmanaged") || !strings.Contains(err.Error(), "no target was changed") {
		t.Fatalf("collision error: %v", err)
	}
	state, loadErr := f.store.Load()
	if loadErr != nil || len(state.Installations) != 0 {
		t.Fatalf("collision mutated state: %+v, %v", state, loadErr)
	}
}
