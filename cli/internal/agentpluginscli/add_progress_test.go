package agentpluginscli

import (
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestInteractiveAddReportsWorkImmediatelyAfterTargetSelection(t *testing.T) {
	t.Parallel()
	fixture := newCLIFixture(t, []domain.DetectedClient{
		fixtureClient(t, domain.ClientCursor),
		fixtureClient(t, domain.ClientCodex),
	})

	_, stderr, err := fixture.executeInput(true, "\n", "add", writeCLIPlugin(t), "--dry-run")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stderr, "Checking selected agents and preparing the install plan...") {
		t.Fatalf("post-selection progress missing from stderr: %q", stderr)
	}
}

func TestInteractiveGroupAddReportsThreeStepsPerSelectedClient(t *testing.T) {
	fixture := newCLIFixture(t, []domain.DetectedClient{
		fixtureClient(t, domain.ClientCursor),
		fixtureClient(t, domain.ClientCodex),
	})

	stdout, stderr, err := fixture.executeInput(true, "\n\n", "add", writeCLIPlugin(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"Cursor", "OpenAI Codex"} {
		previous := -1
		for _, step := range []string{"1/3  preparing package", "2/3  configuration ready", "3/3  "} {
			position := strings.Index(stderr, name+"  "+step)
			if position <= previous {
				t.Fatalf("missing %s %s in progress:\n%s", name, step, stderr)
			}
			previous = position
		}
	}
	if strings.Contains(stdout, "doctor (read-only)") || !strings.Contains(stdout, "installation prepared") {
		t.Fatalf("install summary mixed with unrelated status:\n%s", stdout)
	}
}
