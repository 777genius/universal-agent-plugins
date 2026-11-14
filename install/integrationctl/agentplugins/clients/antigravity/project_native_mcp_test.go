package antigravity

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestAntigravityNativeMCPKeepsOwnedPathsWithoutVendorOverrides(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		env  any
	}{
		{"decoded", map[string]any{"ROOT": "${PLUGIN_ROOT}", "DATA": "${PLUGIN_DATA}", "KEEP": "literal"}},
		{"typed", map[string]string{"ROOT": "${PLUGIN_ROOT}", "DATA": "${PLUGIN_DATA}", "KEEP": "literal"}},
		{"absent", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			source, stage, active, data := t.TempDir(), t.TempDir(), t.TempDir(), t.TempDir()
			entry := map[string]any{
				"type": "stdio", "command": "node", "cwd": "${PLUGIN_ROOT}",
				"args": []any{"${PLUGIN_ROOT}/server.cjs", "--data", "${PLUGIN_DATA}"},
			}
			if tc.env != nil {
				entry["env"] = tc.env
			}
			portableConfig := map[string]any{
				"$schema": domain.MCPSchemaV1, "mcpServers": map[string]any{"local": entry},
			}
			portable, err := json.Marshal(portableConfig)
			if err != nil {
				t.Fatal(err)
			}
			for _, root := range []string{source, stage} {
				if err := os.WriteFile(filepath.Join(root, "mcp.json"), portable, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			envelope := domain.PackageEnvelope{
				SnapshotRoot: source, Manifest: domain.PluginManifest{Name: "native-mcp"},
				MCP: domain.MCPComponent{Servers: map[string]domain.MCPServer{
					"local": {Name: "local", Type: "stdio", Decoded: entry},
				}},
			}
			plan := domain.DeliveryPlan{ClientID: domain.ClientAntigravity, ActivePath: active,
				Components: []domain.ComponentDecision{{Kind: domain.ComponentMCPServer, Name: "local", Support: domain.SupportNative}}}
			if _, err := New().Project(t.Context(), clients.ProjectionInput{
				StagingPath: stage, Envelope: envelope, Plan: plan, PluginDataPath: data,
			}); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Lstat(filepath.Join(stage, "mcp.json")); !os.IsNotExist(err) {
				t.Fatalf("ignored portable MCP survived native projection: %v", err)
			}
			if body, err := os.ReadFile(filepath.Join(source, "mcp.json")); err != nil || string(body) != string(portable) {
				t.Fatalf("native projection changed the verified portable source: %v", err)
			}
			body, err := os.ReadFile(filepath.Join(stage, "mcp_config.json"))
			if err != nil {
				t.Fatal(err)
			}
			var native struct {
				Servers map[string]map[string]any `json:"mcpServers"`
			}
			if err := json.Unmarshal(body, &native); err != nil {
				t.Fatal(err)
			}
			config := native.Servers["local"]
			if config["command"] != "node" || config["cwd"] != active ||
				!reflect.DeepEqual(config["args"], []any{active + "/server.cjs", "--data", data}) {
				t.Fatalf("native projection changed committed stdio paths: %s", body)
			}
			env, ok := config["env"].(map[string]any)
			if !ok {
				t.Fatalf("native environment is not an object: %s", body)
			}
			if _, exists := env["PLUGIN_ROOT"]; exists {
				t.Fatal("native config overrides vendor PLUGIN_ROOT")
			}
			if _, exists := env["PLUGIN_DATA"]; exists {
				t.Fatal("native config overrides vendor PLUGIN_DATA")
			}
			if tc.env != nil && !reflect.DeepEqual(env, map[string]any{"ROOT": active, "DATA": data, "KEEP": "literal"}) {
				t.Fatalf("custom environment expansion changed: %+v", env)
			}
			after, err := json.Marshal(portableConfig)
			if err != nil || string(after) != string(portable) {
				t.Fatalf("native projection modified the source declaration: %v", err)
			}
		})
	}
}

func TestAntigravityStillRejectsAuthoredReservedEnvironment(t *testing.T) {
	t.Parallel()
	for _, key := range []string{"PLUGIN_ROOT", "PLUGIN_DATA"} {
		t.Run(key, func(t *testing.T) {
			result := map[string]map[string]any{}
			envelope := domain.PackageEnvelope{MCP: domain.MCPComponent{Servers: map[string]domain.MCPServer{
				"local": {Type: "stdio", Decoded: map[string]any{
					"command": "node", "env": map[string]any{key: "foreign"},
				}},
			}}}
			if err := appendNativeServer(result, "local", envelope, t.TempDir(), t.TempDir(), t.TempDir()); err == nil {
				t.Fatal("authored reserved environment was silently removed instead of refused")
			}
			if len(result) != 0 {
				t.Fatal("refused server emitted native configuration")
			}
		})
	}
}

func TestAntigravityPortableMCPRemovalRefusesDirectory(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := filepath.Join(root, "mcp.json")
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := removeAntigravityPortableMCP(root); err == nil {
		t.Fatal("unexpected portable MCP directory was removed")
	}
	if info, err := os.Lstat(path); err != nil || !info.IsDir() {
		t.Fatalf("refused portable MCP directory changed: %v", err)
	}
}
