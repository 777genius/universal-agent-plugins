package usecase

import (
	"context"
	"encoding/json"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"os"
	"path/filepath"
	"testing"
)

func TestPortableReservedIdentityLifecycle(t *testing.T) {
	for _, name := range []string{"demo", "con", "con.foo"} {
		t.Run(name, func(t *testing.T) {
			service, store, client := serviceFixture(t)
			input := addInput(t, client, "https://example.com/identity")
			input.Confirmed = true
			portableIdentityMCP(t, &input)
			input.Envelope.Manifest.Name = name
			raw, _ := json.Marshal(map[string]any{"$schema": domain.PluginSchemaV1, "name": name, "version": "1.0.0"})
			input.Envelope.Manifest.Raw = raw
			if err := os.WriteFile(filepath.Join(input.Envelope.SnapshotRoot, "plugin.json"), raw, 0600); err != nil {
				t.Fatal(err)
			}
			installed, err := service.Add(context.Background(), input)
			if err != nil {
				t.Fatal(err)
			}
			if err := pathpolicy.ValidateLeafID(installed.Plan.PhysicalArtifactID); err != nil {
				t.Fatal(err)
			}
			if installed.Plan.DeclaredName != name {
				t.Fatal("logical name changed")
			}
			stateBefore, err := store.Load()
			if err != nil {
				t.Fatal(err)
			}
			if len(stateBefore.Installations[0].DataReceipts) != 1 {
				t.Fatal("data receipt missing")
			}
			var data domain.DataReceipt
			for _, receipt := range stateBefore.Installations[0].DataReceipts {
				data = receipt
			}
			marker := filepath.Join(data.Locator, "keep")
			if err := os.WriteFile(marker, []byte("persistent"), 0600); err != nil {
				t.Fatal(err)
			}
			input.OperationID = "identity-update"
			updated, err := service.Update(context.Background(), input)
			if err != nil {
				t.Fatal(err)
			}
			if updated.Plan.ActivePath != installed.Plan.ActivePath {
				t.Fatal("update moved storage")
			}
			if err := os.WriteFile(filepath.Join(installed.Plan.ActivePath, "plugin.json"), []byte("tampered"), 0600); err != nil {
				t.Fatal(err)
			}
			input.OperationID = "identity-repair"
			repaired, err := service.Repair(context.Background(), input)
			if err != nil {
				t.Fatal(err)
			}
			if repaired.Plan.ActivePath != installed.Plan.ActivePath {
				t.Fatal("repair moved storage")
			}
			state, err := store.Load()
			if err != nil {
				t.Fatal(err)
			}
			if len(state.Installations[0].DataReceipts) != 1 || state.Installations[0].DataReceipts[data.DataReceiptID].DataReceiptID != data.DataReceiptID {
				t.Fatal("data identity changed")
			}
			if body, err := os.ReadFile(marker); err != nil || string(body) != "persistent" {
				t.Fatal("update/repair lost data")
			}
			if state.Installations[0].DeclaredName != name {
				t.Fatal("persisted logical name changed")
			}
			if _, err := service.Remove(context.Background(), RemoveInput{Selector: installed.InstallationID, Client: client, Scope: domain.ScopeUser, Confirmed: true, OperationID: "identity-remove"}); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(installed.Plan.ActivePath); !os.IsNotExist(err) {
				t.Fatalf("owned path retained: %v", err)
			}
		})
	}
}

func TestPortableReservedGroupedLifecycle(t *testing.T) {
	t.Parallel()
	service, store, cursor := serviceFixture(t)
	kiro := domain.DetectedClient{ClientID: domain.ClientKiro, Status: domain.DetectionDetected, ConfigRoot: filepath.Join(t.TempDir(), ".kiro")}

	cursorAdd := portableNamedGroupInput(t, cursor, "https://example.com/grouped")
	kiroAdd := portableNamedGroupInput(t, kiro, "https://example.com/grouped")
	added, err := service.AddGroup(context.Background(), GroupInput{
		Targets: []AddInput{cursorAdd, kiroAdd}, OperationGroupID: "group-add", Confirmed: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !added.Mutated || len(added.Receipts) != 2 {
		t.Fatalf("grouped add = %+v", added)
	}
	state, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Installations) != 1 || len(state.Installations[0].Clients) != 2 || state.Installations[0].OperationGroupID != "group-add" {
		t.Fatalf("grouped add state = %+v", state.Installations)
	}
	for _, binding := range state.Installations[0].Clients {
		if binding.PackageRevision == nil || binding.PackageRevision.TreeDigest != "sha256:source-tree" || len(binding.Receipts) != 1 || binding.Receipts[0].OperationGroupID != "group-add" {
			t.Fatalf("grouped add binding = %+v", binding)
		}
	}

	cursorUpdate := portableNamedGroupInput(t, cursor, "https://example.com/grouped")
	kiroUpdate := portableNamedGroupInput(t, kiro, "https://example.com/grouped")
	setEnvelopeVersion(t, &cursorUpdate.Envelope, "2.0.0", "sha256:group-tree-v2", "sha256:group-manifest-v2")
	setEnvelopeVersion(t, &kiroUpdate.Envelope, "2.0.0", "sha256:group-tree-v2", "sha256:group-manifest-v2")
	updated, err := service.UpdateGroup(context.Background(), GroupInput{
		Targets: []AddInput{cursorUpdate, kiroUpdate}, CompatibilityChecks: []AddInput{cursorUpdate, kiroUpdate},
		OperationGroupID: "group-update", Confirmed: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !updated.Mutated || len(updated.Receipts) != 2 {
		t.Fatalf("grouped update = %+v", updated)
	}
	state, err = store.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, binding := range state.Installations[0].Clients {
		if binding.PackageRevision == nil || binding.PackageRevision.Version != "2.0.0" || binding.PackageRevision.TreeDigest != "sha256:group-tree-v2" || len(binding.Receipts) != 2 {
			t.Fatalf("grouped update binding = %+v", binding)
		}
	}

	removed, err := service.RemoveGroup(context.Background(), RemoveGroupInput{
		Selector: added.InstallationID,
		Targets: []RemoveInput{
			{Client: cursor, Scope: domain.ScopeUser, ExternalUninstalled: true},
			{Client: kiro, Scope: domain.ScopeUser, ExternalUninstalled: true},
		},
		OperationGroupID: "group-remove", Confirmed: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !removed.Mutated || len(removed.Receipts) != 2 {
		t.Fatalf("grouped remove = %+v", removed)
	}
	state, err = store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Installations) != 1 || len(state.Installations[0].Clients) != 0 || !state.Installations[0].DataRetained {
		t.Fatalf("grouped remove state = %+v", state.Installations)
	}
	for _, target := range added.Targets {
		if _, err := os.Lstat(target.Plan.ActivePath); !os.IsNotExist(err) {
			t.Fatalf("grouped remove retained %s: %v", target.Plan.ActivePath, err)
		}
	}
}

func portableNamedGroupInput(t *testing.T, client domain.DetectedClient, source string) AddInput {
	input := addInput(t, client, source)
	portableIdentityMCP(t, &input)
	input.Envelope.Manifest.Name = "con.foo"
	body, _ := json.Marshal(map[string]any{"$schema": domain.PluginSchemaV1, "name": "con.foo", "version": "1.0.0"})
	input.Envelope.Manifest.Raw = body
	if err := os.WriteFile(filepath.Join(input.Envelope.SnapshotRoot, "plugin.json"), body, 0600); err != nil {
		t.Fatal(err)
	}
	return input
}

func portableIdentityMCP(t *testing.T, input *AddInput) {
	t.Helper()
	decoded := map[string]any{"type": "stdio", "command": "node", "args": []any{"${PLUGIN_DATA}"}}
	input.Envelope.MCP = domain.MCPComponent{Present: true, Enabled: true, Servers: map[string]domain.MCPServer{"local": {Name: "local", Type: "stdio", Decoded: decoded}}}
	input.Envelope.Inventory.MCPServers = []string{"local"}
	raw, _ := json.Marshal(map[string]any{"mcpServers": map[string]any{"local": decoded}})
	input.Envelope.MCP.Raw = raw
	if err := os.WriteFile(filepath.Join(input.Envelope.SnapshotRoot, "mcp.json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
}
