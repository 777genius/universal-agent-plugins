package agentpluginscli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestOpenCodeNamespaceCLIGroupRefusesBeforeMutation(t *testing.T) {
	openCode := fixtureClient(t, domain.ClientOpenCode)
	cursor := fixtureClient(t, domain.ClientCursor)
	fixture := newCLIFixture(t, []domain.DetectedClient{cursor, openCode})
	if err := os.MkdirAll(openCode.ConfigRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(openCode.ConfigRoot, "opencode.json")
	foreign := []byte(`{"mcp":{"demo_extra":{"type":"remote","url":"https://foreign.test"}}}`)
	if err := os.WriteFile(config, foreign, 0o600); err != nil {
		t.Fatal(err)
	}
	plugin := writeCLIPlugin(t)
	writeCLIMCP(t, plugin)
	stdout, stderr, err := fixture.execute(false, "add", plugin, "--target", "cursor,opencode", "--format", "json")
	if err == nil || !strings.Contains(err.Error()+stderr, `"demo"`) || !strings.Contains(err.Error()+stderr, `"demo_extra"`) {
		t.Fatalf("missing actionable conflict: %v; stderr: %s", err, stderr)
	}
	if strings.Contains(stdout, openCodeNamespaceNotice) {
		t.Fatalf("failed preflight claimed namespace success: %s", stdout)
	}
	state, loadErr := fixture.store.Load()
	if loadErr != nil || len(state.Installations) != 0 {
		t.Fatalf("failed CLI preflight changed state: %+v, %v", state, loadErr)
	}
	if body, readErr := os.ReadFile(config); readErr != nil || string(body) != string(foreign) {
		t.Fatalf("failed CLI preflight changed OpenCode config: %s, %v", body, readErr)
	}
}

func TestOpenCodeNamespaceCLIDryRunShowsInformationalEvidence(t *testing.T) {
	openCode := fixtureClient(t, domain.ClientOpenCode)
	fixture := newCLIFixture(t, []domain.DetectedClient{openCode})
	plugin := writeCLIPlugin(t)
	writeCLIMCP(t, plugin)
	stdout, stderr, err := fixture.execute(false, "add", plugin, "--target", "opencode", "--dry-run", "--format", "json")
	if err != nil {
		t.Fatalf("safe OpenCode dry-run failed: %v; stderr: %s", err, stderr)
	}
	if !strings.Contains(stdout, openCodeNamespaceNotice) || !strings.Contains(stdout, `"severity":"info"`) {
		t.Fatalf("safe preflight did not publish informational evidence: %s", stdout)
	}
}
