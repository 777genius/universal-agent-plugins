package providers

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/planner"
)

func TestCodexTransportSelectionSanitizesBothManifestSurfaces(t *testing.T) {
	e := stagingEnvelope(t)
	raw := []byte(`{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{"notion":{"type":"streamable-http","url":"https://example.invalid/mcp"},"stdio":{"type":"stdio","command":"./bin/run"},"events":{"type":"sse","url":"https://example.invalid/events"},"broken":{"type":"unknown"}}}`)
	writeTestFile(t, filepath.Join(e.SnapshotRoot, "mcp.json"), string(raw))
	var doc struct {
		Servers map[string]json.RawMessage `json:"mcpServers"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	e.MCP.Servers = map[string]domain.MCPServer{}
	for name, body := range doc.Servers {
		if name == "broken" {
			continue
		}
		var decoded map[string]any
		if err := json.Unmarshal(body, &decoded); err != nil {
			t.Fatal(err)
		}
		e.MCP.Servers[name] = domain.MCPServer{Name: name, Type: decoded["type"].(string), Raw: body, Decoded: decoded}
	}
	root := t.TempDir()
	plan, err := (planner.Planner{ManagedRoot: filepath.Join(root, "managed")}).Plan(context.Background(), e, domain.DetectedClient{ClientID: domain.ClientCodex, Status: domain.DetectionDetected, ConfigRoot: filepath.Join(root, "config")}, domain.ScopeUser, "demo-0123456789ab")
	if err != nil {
		t.Fatal(err)
	}
	delivery, err := (Stager{}).Stage(context.Background(), e, plan, "codex-selection", domain.CompatibilityHints{})
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"mcp.json", ".mcp.json"} {
		servers := readObject(t, filepath.Join(delivery.StagingPath, name))["mcpServers"].(map[string]any)
		var keys []string
		for key := range servers {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		if !reflect.DeepEqual(keys, []string{"notion", "stdio"}) {
			t.Fatalf("%s servers=%v", name, keys)
		}
	}
	rootManifest := readObject(t, filepath.Join(delivery.StagingPath, "plugin.json"))
	if rootManifest["name"] != "demo" || rootManifest["$schema"] != domain.PluginSchemaV1 {
		t.Fatalf("standard manifest lost: %v", rootManifest)
	}
	_ = readObject(t, filepath.Join(delivery.StagingPath, ".codex-plugin", "plugin.json"))
	if _, err := os.Stat(filepath.Join(delivery.StagingPath, "skills", "good", "SKILL.md")); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(filepath.Join(e.SnapshotRoot, "mcp.json"))
	if err != nil || string(after) != string(raw) {
		t.Fatal("portable source changed")
	}
}
