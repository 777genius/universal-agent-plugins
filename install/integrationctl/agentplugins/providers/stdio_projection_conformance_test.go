package providers_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/loader"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/specregistry"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// Test the real standard loader and native JSON projection, not an invented
// configuration map alone. No vendor executable or model account is involved.
func TestProjectMCPPreservesOpaqueStandardPlaceholders(t *testing.T) {
	envelope := loadPlaceholderPackage(t)
	original := placeholderJSON(t, envelope.MCP.Servers["probe"].Decoded)
	cases := []struct {
		name    string
		dialect shared.MCPDialect
		file    string
	}{
		{"openai", shared.MCPDialectOpenAI, ".mcp.json"},
		{"cursor", shared.MCPDialectCursor, "mcp.json"},
		{"kiro", shared.MCPDialectKiro, "mcp.json"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stage, data := t.TempDir(), t.TempDir()
			active := filepath.Join(t.TempDir(), "future")
			err := shared.ProjectMCPServers(shared.MCPProjection{
				Root: stage, Envelope: envelope, Names: []string{"probe"},
				Dialect: tc.dialect, PluginRoot: active, DataPath: data,
			})
			if err != nil {
				t.Fatal(err)
			}
			assertPlaceholderProjection(t, filepath.Join(stage, tc.file), active, data)
			if string(placeholderJSON(t, envelope.MCP.Servers["probe"].Decoded)) != string(original) {
				t.Fatal("native projection mutated the source envelope")
			}
		})
	}
}

func loadPlaceholderPackage(t *testing.T) domain.PackageEnvelope {
	t.Helper()
	root := t.TempDir()
	manifest := map[string]any{"$schema": domain.PluginSchemaV1, "name": "placeholder-probe"}
	server := map[string]any{
		"type": "stdio", "command": "node", "cwd": "./",
		"args": []string{"${PLUGIN_ROOT}/config", "${PLUGIN_DATA}/cache", "${package.root}/literal", "${UNKNOWN}", "$HOME"},
		"env":  map[string]string{"ROOT": "${PLUGIN_ROOT}/config", "DATA": "${PLUGIN_DATA}/cache", "LITERAL": "${package.root}/literal", "UNKNOWN": "${UNKNOWN}", "SHELL": "$HOME"},
	}
	mcp := map[string]any{"$schema": domain.MCPSchemaV1, "mcpServers": map[string]any{"probe": server}}
	for name, document := range map[string]any{"plugin.json": manifest, "mcp.json": mcp} {
		if err := os.WriteFile(filepath.Join(root, name), placeholderJSON(t, document), 0600); err != nil {
			t.Fatal(err)
		}
	}
	registry, err := specregistry.New()
	if err != nil {
		t.Fatal(err)
	}
	envelope, err := (loader.Loader{Registry: registry}).Load(context.Background(), domain.LoadInput{SnapshotRoot: root})
	if err != nil {
		t.Fatal(err)
	}
	if server, ok := envelope.MCP.Servers["probe"]; !ok || server.Type != "stdio" {
		t.Fatalf("valid standard MCP server was not loaded: %+v", envelope.Diagnostics)
	}
	return envelope
}

func assertPlaceholderProjection(t *testing.T, path, active, data string) {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var document struct {
		Servers map[string]struct {
			Command string            `json:"command"`
			CWD     string            `json:"cwd"`
			Args    []string          `json:"args"`
			Env     map[string]string `json:"env"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal(body, &document); err != nil {
		t.Fatal(err)
	}
	server := document.Servers["probe"]
	args := []string{active + "/config", data + "/cache", "${package.root}/literal", "${UNKNOWN}", "$HOME"}
	env := map[string]string{
		"ROOT": active + "/config", "DATA": data + "/cache", "LITERAL": "${package.root}/literal",
		"UNKNOWN": "${UNKNOWN}", "SHELL": "$HOME", "PLUGIN_ROOT": active, "PLUGIN_DATA": data,
	}
	if server.Command != "node" || server.CWD != active || !reflect.DeepEqual(server.Args, args) || !reflect.DeepEqual(server.Env, env) {
		t.Fatalf("unexpected native projection: %s", body)
	}
}

func placeholderJSON(t *testing.T, value any) []byte {
	t.Helper()
	body, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return body
}
