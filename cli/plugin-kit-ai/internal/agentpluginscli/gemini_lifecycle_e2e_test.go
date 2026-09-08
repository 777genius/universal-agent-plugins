package agentpluginscli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// This exercises the production lifecycle against a disposable Gemini home.
// It never invokes Gemini CLI or reads the user's real configuration.
func TestGeminiDisposableHomeAddUpdateRemoveE2E(t *testing.T) {
	t.Parallel()
	client := fixtureClient(t, domain.ClientGemini)
	fixture := newCLIFixture(t, []domain.DetectedClient{client})
	plugin := writeCLIPlugin(t)
	// The Go toolchain is already required to run this test. This fixture only
	// exercises configuration delivery; it never starts the configured command.
	mcpFixture := `{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{"demo":{"type":"streamable-http","url":"https://example.test/mcp"},"local-fixture":{"type":"stdio","command":"go","args":["version"],"env":{"DATA":"${PLUGIN_DATA}"},"cwd":"./workspace"}}}`
	if err := os.WriteFile(filepath.Join(plugin, "mcp.json"), []byte(mcpFixture), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(plugin, "workspace"), 0o755); err != nil {
		t.Fatal(err)
	}
	skill := filepath.Join(plugin, "skills", "docs", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(skill), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(skill, []byte("---\nname: docs\ndescription: Test docs skill\n---\n\n# Docs\n\nv1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	settings := filepath.Join(client.ConfigRoot, "settings.json")
	if err := os.MkdirAll(client.ConfigRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(settings, []byte(`{"theme":"night","mcpServers":{"foreign":{"url":"https://foreign.test"}}}`), 0o600); err != nil {
		t.Fatal(err)
	}

	stdout, _, err := fixture.execute(false, "add", plugin, "--target", "gemini", "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	assertVersionedJSON(t, stdout, "add")
	assertGeminiCLIProjection(t, client.ConfigRoot, "https://example.test/mcp", "v1", true)

	manifest := filepath.Join(plugin, "plugin.json")
	body, err := os.ReadFile(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifest, []byte(strings.Replace(string(body), `"version": "1.0.0"`, `"version": "2.0.0"`, 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	mcpPath := filepath.Join(plugin, "mcp.json")
	mcpBody, err := os.ReadFile(mcpPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(mcpPath, []byte(strings.Replace(string(mcpBody), "https://example.test/mcp", "https://example.test/v2", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(skill, []byte("---\nname: docs\ndescription: Test docs skill\n---\n\n# Docs\n\nv2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stdout, _, err = fixture.execute(false, "update", "demo", "--target", "gemini", "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	assertVersionedJSON(t, stdout, "update")
	assertGeminiCLIProjection(t, client.ConfigRoot, "https://example.test/v2", "v2", true)

	stdout, _, err = fixture.execute(false, "remove", "demo", "--target", "gemini", "--format", "json")
	if err != nil {
		t.Fatal(err)
	}
	assertVersionedJSON(t, stdout, "remove")
	assertGeminiCLIProjection(t, client.ConfigRoot, "", "", false)
}

func assertGeminiCLIProjection(t *testing.T, root, url, marker string, present bool) {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(root, "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := json.Unmarshal(body, &document); err != nil {
		t.Fatal(err)
	}
	if document["theme"] != "night" {
		t.Fatalf("unmanaged Gemini setting lost: %+v", document)
	}
	servers := document["mcpServers"].(map[string]any)
	if _, ok := servers["foreign"]; !ok {
		t.Fatalf("foreign Gemini MCP entry lost: %+v", servers)
	}
	managed, exists := servers["demo"].(map[string]any)
	if exists != present {
		t.Fatalf("Gemini MCP presence = %v, want %v", exists, present)
	}
	if present && managed["httpUrl"] != url {
		t.Fatalf("Gemini MCP URL = %v, want %v", managed["httpUrl"], url)
	}
	local, localExists := servers["local-fixture"].(map[string]any)
	if localExists != present {
		t.Fatalf("Gemini local MCP presence = %v, want %v", localExists, present)
	}
	if present {
		if local["command"] != "go" || !strings.HasSuffix(local["cwd"].(string), "/workspace") {
			t.Fatalf("Gemini local MCP projection = %+v", local)
		}
	}
	skillPath := filepath.Join(root, "skills", "docs", "SKILL.md")
	if present {
		skillBody, err := os.ReadFile(skillPath)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(skillBody), marker) {
			t.Fatalf("Gemini skill = %q", skillBody)
		}
	} else if _, err := os.Stat(skillPath); !os.IsNotExist(err) {
		t.Fatalf("removed Gemini skill remains: %v", err)
	}
}
