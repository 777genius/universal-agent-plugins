package platformexec

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

func TestGrokGenerateWritesWrappedMCPSidecar(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	graph := packagedPluginTestGraph(t, root, "grok")
	artifacts, err := newGrokPluginAdapter().Generate(root, graph, pluginmodel.NewTargetState("grok"))
	if err != nil {
		t.Fatal(err)
	}
	var sawManifest, sawMCP, sawSkill bool
	for _, artifact := range artifacts {
		switch artifact.RelPath {
		case ".grok-plugin/plugin.json":
			sawManifest = true
			if !strings.Contains(string(artifact.Content), `"skills": "./skills/"`) {
				t.Fatalf("missing skills ref:\n%s", artifact.Content)
			}
			if !strings.Contains(string(artifact.Content), `"mcpServers": "./.mcp.json"`) {
				t.Fatalf("missing MCP ref:\n%s", artifact.Content)
			}
		case ".mcp.json":
			sawMCP = true
			if !strings.Contains(string(artifact.Content), `"mcpServers"`) || !strings.Contains(string(artifact.Content), `"docs"`) {
				t.Fatalf("Grok MCP sidecar should wrap servers:\n%s", artifact.Content)
			}
		case filepath.ToSlash(filepath.Join("skills", "release-checks", "SKILL.md")):
			sawSkill = true
		}
	}
	if !sawManifest || !sawMCP || !sawSkill {
		t.Fatalf("artifacts = %+v", artifacts)
	}
}

func TestGrokImportAcceptsVendorMCPFileRef(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".grok-plugin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "agents", "grok"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".grok-plugin", "plugin.json"), []byte(`{"name":"demo","version":"0.1.0","description":"demo","mcpServers":"./agents/grok/mcp.json"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "agents", "grok", "mcp.json"), []byte(`{"mcpServers":{"docs":{"type":"http","url":"https://example.com/mcp"}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	imported, err := newGrokPluginAdapter().Import(root, ImportSeed{
		Manifest: pluginmodel.Manifest{Name: "demo", Version: "0.1.0", Description: "demo", Targets: []string{"grok"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(imported.Artifacts) != 1 || !strings.Contains(string(imported.Artifacts[0].Content), "docs:") {
		t.Fatalf("imported artifacts = %+v", imported.Artifacts)
	}
}

func TestKimiGenerateInlinesMCP(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	graph := packagedPluginTestGraph(t, root, "kimi")
	artifacts, err := newKimiPluginAdapter().Generate(root, graph, pluginmodel.NewTargetState("kimi"))
	if err != nil {
		t.Fatal(err)
	}
	var sawManifest, sawSidecar, sawSkill bool
	for _, artifact := range artifacts {
		switch artifact.RelPath {
		case ".kimi-plugin/plugin.json":
			sawManifest = true
			if !strings.Contains(string(artifact.Content), `"skills": "./skills/"`) {
				t.Fatalf("missing skills ref:\n%s", artifact.Content)
			}
			if !strings.Contains(string(artifact.Content), `"mcpServers"`) || !strings.Contains(string(artifact.Content), `"docs"`) {
				t.Fatalf("Kimi MCP should be inline:\n%s", artifact.Content)
			}
			if strings.Contains(string(artifact.Content), `"./.mcp.json"`) {
				t.Fatalf("Kimi must not use an MCP file ref:\n%s", artifact.Content)
			}
		case ".mcp.json":
			sawSidecar = true
		case filepath.ToSlash(filepath.Join("skills", "release-checks", "SKILL.md")):
			sawSkill = true
		}
	}
	if !sawManifest || sawSidecar || !sawSkill {
		t.Fatalf("artifacts = %+v", artifacts)
	}
}

func TestKimiImportRejectsMCPFileRef(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".kimi-plugin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".kimi-plugin", "plugin.json"), []byte(`{"name":"demo","mcpServers":"./.mcp.json"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := newKimiPluginAdapter().Import(root, ImportSeed{
		Manifest: pluginmodel.Manifest{Name: "demo", Targets: []string{"kimi"}},
	}); err == nil || !strings.Contains(err.Error(), "does not resolve MCP file refs") {
		t.Fatalf("err = %v", err)
	}
}

func TestKimiDetectNativeAcceptsRootManifest(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "kimi.plugin.json"), []byte(`{"name":"demo"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if !newKimiPluginAdapter().DetectNative(root) {
		t.Fatal("expected kimi.plugin.json to detect the Kimi plugin")
	}
}

func TestKimiValidateRequiresInlineMCP(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	graph := packagedPluginTestGraph(t, root, "kimi")
	if err := os.MkdirAll(filepath.Join(root, ".kimi-plugin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "skills", "release-checks"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "skills", "release-checks", "SKILL.md"), []byte("---\nname: release-checks\ndescription: release checks\n---\n\nUse this skill.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".kimi-plugin", "plugin.json"), []byte(`{"name":"kimi-demo","version":"0.1.0","description":"kimi demo","skills":"./skills/","mcpServers":"./.mcp.json"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	diagnostics, err := newKimiPluginAdapter().Validate(root, graph, pluginmodel.NewTargetState("kimi"))
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) == 0 || !strings.Contains(diagnostics[0].Message, "inline MCP object") {
		t.Fatalf("diagnostics = %+v", diagnostics)
	}
}

func packagedPluginTestGraph(t *testing.T, root, target string) pluginmodel.PackageGraph {
	t.Helper()
	skillPath := filepath.Join(root, "plugin", "skills", "release-checks")
	if err := os.MkdirAll(skillPath, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillPath, "SKILL.md"), []byte("---\nname: release-checks\ndescription: release checks\n---\n\nUse this skill.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	parsed, err := pluginmodel.ParsePortableMCP("plugin/mcp/servers.yaml", []byte(`api_version: v1

servers:
  docs:
    type: remote
    remote:
      protocol: streamable_http
      url: "https://example.com/mcp"
`))
	if err != nil {
		t.Fatal(err)
	}
	return pluginmodel.PackageGraph{
		Manifest: pluginmodel.Manifest{Name: target + "-demo", Version: "0.1.0", Description: target + " demo", Targets: []string{target}},
		Portable: pluginmodel.PortableComponents{
			Items: map[string][]string{
				"skills": {filepath.ToSlash(filepath.Join("plugin", "skills", "release-checks", "SKILL.md"))},
			},
			MCP: &pluginmodel.PortableMCP{Path: filepath.ToSlash(filepath.Join("plugin", "mcp", "servers.yaml")), Servers: parsed.Servers, File: parsed.File},
		},
	}
}
