package usecase

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
)

func TestWindsurfLifecycleAddUpdateRepairRemoveInIsolatedHome(t *testing.T) {
	t.Parallel()
	service, store, _ := serviceFixture(t)
	service.NativeObserver = providers.NativeIdentityObserver{Stager: service.Stager}
	configRoot := filepath.Join(t.TempDir(), "home", ".codeium", "windsurf")
	configPath := filepath.Join(configRoot, "mcp_config.json")
	if err := os.MkdirAll(configRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte(`{"mcpServers":{"foreign":{"url":"https://foreign.test"}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	client := domain.DetectedClient{ClientID: domain.ClientWindsurf, Status: domain.DetectionDetected, ConfigRoot: configRoot}

	add := windsurfUsecaseInput(t, client, "one")
	add.Confirmed = true
	installed, err := service.Add(context.Background(), add)
	if err != nil {
		t.Fatal(err)
	}
	if installed.Activation.Activation != domain.ActivationActive || installed.Activation.Verification != domain.VerificationInstalled {
		t.Fatalf("add result = %+v", installed)
	}
	assertUsecaseWindsurfConfig(t, configPath, "one", true)

	update := windsurfUsecaseInput(t, client, "two")
	update.Confirmed = true
	update.OperationID = "windsurf-update"
	updated, err := service.Update(context.Background(), update)
	if err != nil {
		t.Fatal(err)
	}
	if !updated.Mutated || updated.Activation.Activation != domain.ActivationActive {
		t.Fatalf("update result = %+v", updated)
	}
	assertUsecaseWindsurfConfig(t, configPath, "two", true)

	if err := os.WriteFile(filepath.Join(updated.Plan.ActivePath, "damage"), []byte("repair"), 0o600); err != nil {
		t.Fatal(err)
	}
	update.OperationID = "windsurf-repair"
	repaired, err := service.Repair(context.Background(), update)
	if err != nil {
		t.Fatal(err)
	}
	if !repaired.Mutated || repaired.Activation.Verification != domain.VerificationInstalled {
		t.Fatalf("repair result = %+v", repaired)
	}
	assertUsecaseWindsurfConfig(t, configPath, "two", true)

	removed, err := service.Remove(context.Background(), RemoveInput{
		Selector: installed.InstallationID, Client: client, Scope: domain.ScopeUser,
		Confirmed: true, OperationID: "windsurf-remove",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !removed.Mutated {
		t.Fatalf("remove result = %+v", removed)
	}
	assertUsecaseWindsurfConfig(t, configPath, "", true)
	state, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Installations) != 1 || len(state.Installations[0].Clients) != 0 {
		t.Fatalf("Windsurf binding remains after removal: %+v", state.Installations)
	}
}

func TestWindsurfLifecycleRejectsUnmanagedCollisionBeforePackageMutation(t *testing.T) {
	t.Parallel()
	service, store, _ := serviceFixture(t)
	service.NativeObserver = providers.NativeIdentityObserver{Stager: service.Stager}
	configRoot := filepath.Join(t.TempDir(), "home", ".codeium", "windsurf-next")
	configPath := filepath.Join(configRoot, "mcp_config.json")
	if err := os.MkdirAll(configRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	original := `{"mcpServers":{"docs":{"url":"https://unmanaged.test"}}}`
	if err := os.WriteFile(configPath, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	client := domain.DetectedClient{ClientID: domain.ClientWindsurf, Status: domain.DetectionDetected, ConfigRoot: configRoot}
	input := windsurfUsecaseInput(t, client, "one")
	input.Confirmed = true
	if _, err := service.Add(context.Background(), input); err == nil {
		t.Fatal("unmanaged Windsurf collision was accepted")
	}
	body, _ := os.ReadFile(configPath)
	if string(body) != original {
		t.Fatalf("collision changed native config: %s", body)
	}
	state, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Installations) != 0 {
		t.Fatalf("collision persisted lifecycle state: %+v", state.Installations)
	}
}

func windsurfUsecaseInput(t *testing.T, client domain.DetectedClient, revision string) AddInput {
	t.Helper()
	input := addInput(t, client, "./windsurf-plugin")
	mcp := `{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{"docs":{"type":"streamable-http","url":"https://docs.test/` + revision + `"}}}`
	if err := os.WriteFile(filepath.Join(input.Envelope.SnapshotRoot, "mcp.json"), []byte(mcp), 0o644); err != nil {
		t.Fatal(err)
	}
	raw := json.RawMessage(`{"type":"streamable-http","url":"https://docs.test/` + revision + `"}`)
	input.Envelope.MCP = domain.MCPComponent{Present: true, Enabled: true, Servers: map[string]domain.MCPServer{
		"docs": {Name: "docs", Type: "streamable-http", Raw: raw, Decoded: map[string]any{"type": "streamable-http", "url": "https://docs.test/" + revision}},
	}}
	input.Envelope.Inventory.MCPPresent = true
	input.Envelope.Inventory.MCPEnabled = true
	input.Envelope.Inventory.MCPServers = []string{"docs"}
	input.Envelope.TreeDigest = "sha256:windsurf-tree-" + revision
	input.Envelope.ManifestDigest = "sha256:windsurf-manifest-" + revision
	return input
}

func assertUsecaseWindsurfConfig(t *testing.T, path, revision string, foreign bool) {
	t.Helper()
	document := readUsecaseObject(t, path)
	servers := document["mcpServers"].(map[string]any)
	if revision == "" {
		if _, exists := servers["docs"]; exists {
			t.Fatalf("owned Windsurf entry remains: %+v", servers)
		}
	} else if got := servers["docs"].(map[string]any)["serverUrl"]; got != "https://docs.test/"+revision {
		t.Fatalf("Windsurf docs URL = %v", got)
	}
	if foreign && servers["foreign"].(map[string]any)["url"] != "https://foreign.test" {
		t.Fatalf("foreign Windsurf entry changed: %+v", servers)
	}
}
