package agentpluginscli

import (
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// A host with only cursor-agent has no Cursor user-scope target, although
// doctor reports the agent. The error names the scope the agent does select.
func TestCursorAgentOnlyUserTargetSuggestsProjectScope(t *testing.T) {
	t.Parallel()
	agentOnly := fixtureClient(t, domain.ClientCursor)
	agentOnly.Status = domain.DetectionNotDetected
	agentOnly.Surfaces = []domain.ClientSurface{{ID: domain.CursorAgentSurfaceID, Detected: true}}
	fixture := newCLIFixture(t, []domain.DetectedClient{agentOnly})
	_, _, err := fixture.execute(false, "add", writeCLIPlugin(t), "--target", "cursor")
	if err == nil || !strings.Contains(err.Error(), `target "cursor" was not detected`) || !strings.Contains(err.Error(), "--scope project") {
		t.Fatalf("agent-only Cursor error = %v", err)
	}
}
