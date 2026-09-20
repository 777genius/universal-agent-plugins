package platformexec

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

func TestCodexPackageValidateRejectsMCPProjectionMismatch(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeCodexTestFile(t, filepath.Join(root, ".codex-plugin", "plugin.json"), "{\n  \"name\": \"codex-demo\",\n  \"version\": \"0.1.0\",\n  \"description\": \"codex demo\",\n  \"mcpServers\": \"./.mcp.json\"\n}\n")
	writeCodexTestFile(t, filepath.Join(root, ".mcp.json"), "{\n  \"docs\": {\"type\": \"remote\", \"url\": \"https://wrong.example/mcp\"}\n}\n")
	parsed, err := pluginmodel.ParsePortableMCP("plugin/mcp/servers.yaml", []byte(`api_version: v1

servers:
  docs:
    type: remote
    remote:
      protocol: streamable_http
      url: "https://example.com/mcp"
`))
	if err != nil {
		t.Fatalf("ParsePortableMCP error = %v", err)
	}
	diagnostics, err := (codexPackageAdapter{}).Validate(root, pluginmodel.PackageGraph{
		Manifest: pluginmodel.Manifest{
			Name:        "codex-demo",
			Version:     "0.1.0",
			Description: "codex demo",
			Targets:     []string{"codex-package"},
		},
		Portable: pluginmodel.PortableComponents{
			MCP: &pluginmodel.PortableMCP{
				Path:    filepath.ToSlash(filepath.Join("plugin", "mcp", "servers.yaml")),
				Servers: parsed.Servers,
				File:    parsed.File,
			},
		},
	}, pluginmodel.NewTargetState("codex-package"))
	if err != nil {
		t.Fatalf("Validate error = %v", err)
	}
	if !strings.Contains(diagnosticsText(diagnostics), "Codex MCP manifest .mcp.json does not match authored portable MCP projection") {
		t.Fatalf("diagnostics missing MCP mismatch guidance:\n%s", diagnosticsText(diagnostics))
	}
}
