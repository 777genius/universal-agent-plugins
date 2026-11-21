package agentpluginscli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providerstest"
)

// This drives the public CLI command, official plugin.json extension, loader,
// planner, native staging, ownership receipts, and Kiro's real config mutation.
// The Kiro runtime and account are deliberately not mocked as "verified".
func TestKiroPublisherMCPApprovalsFullCLI(t *testing.T) {
	client := fixtureClient(t, domain.ClientKiro)
	client.ExecutablePath = "/fixture/kiro-cli"
	fixture := newCLIFixture(t, []domain.DetectedClient{client})
	fixture.app.Lifecycle.Activator = providerstest.NewActivator(providers.Activator{})

	source := writeCLIPlugin(t)
	writeCLIMCP(t, source)
	manifestPath := filepath.Join(source, "plugin.json")
	body, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	var manifest map[string]any
	if err := json.Unmarshal(body, &manifest); err != nil {
		t.Fatal(err)
	}
	manifest["extensions"] = map[string]any{
		"com.universal-agent-plugins.kiro-mcp-approval": map[string]any{
			"version":          1,
			"mcp_auto_approve": map[string]any{"demo": []string{"read_docs"}},
		},
	}
	modified, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifestPath, modified, 0600); err != nil {
		t.Fatal(err)
	}

	nativePath := filepath.Join(client.ConfigRoot, "settings", "mcp.json")
	assertNative := func() {
		t.Helper()
		body, err := os.ReadFile(nativePath)
		if err != nil {
			t.Fatal(err)
		}
		var config map[string]any
		if err := json.Unmarshal(body, &config); err != nil {
			t.Fatal(err)
		}
		servers, ok := config["mcpServers"].(map[string]any)
		if !ok {
			t.Fatalf("missing native Kiro MCP servers: %s", body)
		}
		owned, ok := servers["demo"].(map[string]any)
		if !ok || !reflect.DeepEqual(owned["autoApprove"], []any{"read_docs"}) {
			t.Fatalf("Kiro exact approved tool was not installed: %s", body)
		}
		if _, err := os.Stat(filepath.Join(client.ConfigRoot, "settings", "permissions.yaml")); !os.IsNotExist(err) {
			t.Fatalf("unexpected global Kiro trust: %v", err)
		}
	}
	for _, action := range []string{"add", "add", "repair"} {
		args := []string{action, source, "--target", "kiro", "--format", "json"}
		if action == "repair" {
			args[1] = "demo"
		}
		out, stderr, err := fixture.execute(false, args...)
		if err != nil {
			t.Fatalf("%s failed: %s %s (%v)", action, out, stderr, err)
		}
		if !strings.Contains(out, "prepared") {
			t.Fatalf("%s falsely claims native activation or lost preparation: %s", action, out)
		}
		assertNative()
	}
	state, err := fixture.store.Load()
	if err != nil || len(state.Installations) != 1 || len(state.Installations[0].Clients) != 1 {
		t.Fatalf("unexpected persisted installation: %+v, %v", state, err)
	}
	for _, binding := range state.Installations[0].Clients {
		if binding.InstallIntent != domain.InstallIntentPrepare {
			t.Fatal("Kiro installation lost prepared runtime boundary")
		}
	}
	installationID := state.Installations[0].InstallationID
	// Uninstall must revoke the granted tool consent through the public CLI,
	// not only through a direct native-config adapter call.
	stdout, stderr, err := fixture.execute(false, "remove", "demo", "--target", "kiro", "--format", "json")
	if err != nil {
		t.Fatalf("remove failed: %s %s (%v)", stdout, stderr, err)
	}
	if body, err := os.ReadFile(nativePath); err == nil {
		var config map[string]any
		if err := json.Unmarshal(body, &config); err != nil {
			t.Fatal(err)
		}
		if servers, ok := config["mcpServers"].(map[string]any); ok {
			if _, present := servers["demo"]; present {
				t.Fatalf("remove retained the approved Kiro MCP server: %s", body)
			}
		}
	} else if !os.IsNotExist(err) {
		t.Fatal(err)
	}
	state, err = fixture.store.Load()
	if err != nil || len(state.Installations) != 1 {
		t.Fatalf("remove lost retained-data installation: %+v, %v", state, err)
	}
	retained := state.Installations[0]
	if retained.InstallationID != installationID || len(retained.Clients) != 0 ||
		!retained.DataRetained || len(retained.DataReceipts) == 0 {
		t.Fatalf("remove failed Kiro retained-data contract: %+v", retained)
	}
}
