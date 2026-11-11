package antigravity

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestAntigravityRemoteEndpointAliasesPreserveDestination(t *testing.T) {
	t.Parallel()
	const approved = "https://approved.example.test/mcp"
	for _, tc := range []struct {
		name  string
		entry map[string]any
		valid bool
	}{
		{"portable-url", map[string]any{"url": approved}, true},
		{"native-server-url", map[string]any{"serverUrl": approved}, true},
		{"matching-aliases", map[string]any{"url": approved, "serverUrl": approved}, true},
		{"conflicting-aliases", map[string]any{"url": approved, "serverUrl": "https://different.example.test/mcp"}, false},
		{"empty-url", map[string]any{"url": "", "serverUrl": approved}, false},
		{"empty-server-url", map[string]any{"url": approved, "serverUrl": ""}, false},
		{"null-url", map[string]any{"url": nil, "serverUrl": approved}, false},
		{"null-server-url", map[string]any{"url": approved, "serverUrl": nil}, false},
		{"nonstring-url", map[string]any{"url": 1, "serverUrl": approved}, false},
		{"nonstring-server-url", map[string]any{"url": approved, "serverUrl": 1}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			before, err := json.Marshal(tc.entry)
			if err != nil {
				t.Fatal(err)
			}
			envelope := domain.PackageEnvelope{
				Manifest: domain.PluginManifest{Name: "endpoint-plugin"},
				MCP: domain.MCPComponent{Servers: map[string]domain.MCPServer{
					"api": {Name: "api", Type: "streamable-http", Decoded: tc.entry},
				}},
			}
			plan := domain.DeliveryPlan{
				ClientID: domain.ClientAntigravity, ActivePath: filepath.Join(t.TempDir(), "active"),
				Components: []domain.ComponentDecision{{Kind: domain.ComponentMCPServer, Name: "api", Support: domain.SupportNative}},
			}
			_, err = New().Project(t.Context(), clients.ProjectionInput{StagingPath: root, Envelope: envelope, Plan: plan})
			if (err == nil) != tc.valid {
				t.Fatalf("projection acceptance differs from endpoint contract: %v", err)
			}
			after, marshalErr := json.Marshal(tc.entry)
			if marshalErr != nil || string(before) != string(after) {
				t.Fatalf("projection mutated source MCP declaration: %v", marshalErr)
			}
			if !tc.valid {
				for _, name := range []string{"plugin.json", "mcp_config.json"} {
					if _, err := os.Lstat(filepath.Join(root, name)); !os.IsNotExist(err) {
						t.Fatalf("refused endpoint emitted %s: %v", name, err)
					}
				}
				return
			}
			body, err := os.ReadFile(filepath.Join(root, "mcp_config.json"))
			if err != nil {
				t.Fatal(err)
			}
			var config struct {
				Servers map[string]map[string]any `json:"mcpServers"`
			}
			if err := json.Unmarshal(body, &config); err != nil {
				t.Fatal(err)
			}
			if config.Servers["api"]["serverUrl"] != approved {
				t.Fatalf("native projection changed the approved endpoint: %s", body)
			}
			if _, exists := config.Servers["api"]["url"]; exists {
				t.Fatal("native config retained the portable URL alias")
			}
		})
	}
}
