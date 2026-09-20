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
