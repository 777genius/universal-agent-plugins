package platformexec

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
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
			if !strings.Contains(string(artifact.Content), `"type": "http"`) || !strings.Contains(string(artifact.Content), `"url": "https://example.com/mcp"`) {
				t.Fatalf("Grok MCP sidecar must project the remote transport:\n%s", artifact.Content)
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

func TestKimiGenerateUpdatesWinningManifest(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	graph := packagedPluginTestGraph(t, root, "kimi")
	if err := os.WriteFile(filepath.Join(root, "kimi.plugin.json"), []byte(`{"name":"stale"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	artifacts, err := newKimiPluginAdapter().Generate(root, graph, pluginmodel.NewTargetState("kimi"))
	if err != nil {
		t.Fatal(err)
	}
	for _, artifact := range artifacts {
		if artifact.RelPath == ".kimi-plugin/plugin.json" {
			t.Fatalf("root kimi.plugin.json wins on read, so generate must update it in place: %+v", artifacts)
		}
		if artifact.RelPath == "kimi.plugin.json" {
			if !strings.Contains(string(artifact.Content), `"name": "kimi-demo"`) {
				t.Fatalf("generated manifest = %s", artifact.Content)
			}
			return
		}
	}
	t.Fatalf("artifacts = %+v", artifacts)
}

func TestKimiValidateDiagnosticsNameResolvedManifest(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
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
	graph := pluginmodel.PackageGraph{
		Manifest: pluginmodel.Manifest{Name: "kimi-demo", Version: "0.1.0", Description: "kimi demo", Targets: []string{"kimi"}},
		Portable: pluginmodel.PortableComponents{
			Items: map[string][]string{},
			MCP:   &pluginmodel.PortableMCP{Path: "plugin/mcp/servers.yaml", Servers: parsed.Servers, File: parsed.File},
		},
	}
	if err := os.WriteFile(filepath.Join(root, "kimi.plugin.json"), []byte(`{"name":"kimi-demo","version":"0.1.0","description":"kimi demo","mcpServers":{"docs":{"type":"http","url":"https://wrong.example/mcp"}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	diagnostics, err := newKimiPluginAdapter().Validate(root, graph, pluginmodel.NewTargetState("kimi"))
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 1 {
		t.Fatalf("diagnostics = %+v", diagnostics)
	}
	if diagnostics[0].Path != "kimi.plugin.json" {
		t.Fatalf("diagnostics must name the resolved manifest path: %+v", diagnostics[0])
	}
}

func TestPackagedValidateTreatsNullMCPAsAbsent(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "kimi.plugin.json"), []byte(`{"name":"kimi-demo","version":"0.1.0","description":"kimi demo","mcpServers":null}`), 0o644); err != nil {
		t.Fatal(err)
	}
	diagnostics, err := newKimiPluginAdapter().Validate(root, pluginmodel.PackageGraph{
		Manifest: pluginmodel.Manifest{Name: "kimi-demo", Version: "0.1.0", Description: "kimi demo", Targets: []string{"kimi"}},
		Portable: pluginmodel.PortableComponents{Items: map[string][]string{}},
	}, pluginmodel.NewTargetState("kimi"))
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 0 {
		t.Fatalf("mcpServers null must validate as absent, got %+v", diagnostics)
	}
}

func TestContainedPluginRelKeepsDottedNames(t *testing.T) {
	t.Parallel()
	rel, err := containedPluginRel("..mcp.json")
	if err != nil {
		t.Fatalf("dotted file names inside the plugin root must stay valid: %v", err)
	}
	if rel != "..mcp.json" {
		t.Fatalf("rel = %q", rel)
	}
	for _, ref := range []string{"..", "../escape.json", "/abs.json", "."} {
		if _, err := containedPluginRel(ref); err == nil {
			t.Fatalf("ref %q must be rejected", ref)
		}
	}
	if runtime.GOOS == "windows" {
		if _, err := containedPluginRel("C:escape.json"); err == nil {
			t.Fatal("drive-relative ref must be rejected")
		}
	}
}

func TestGrokManagedPathsClaimMCPSidecar(t *testing.T) {
	t.Parallel()
	paths, err := newGrokPluginAdapter().ManagedPaths("", pluginmodel.PackageGraph{}, pluginmodel.NewTargetState("grok"))
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 1 || paths[0] != ".mcp.json" {
		t.Fatalf("grok managed paths = %v; orphaned .mcp.json must be prunable", paths)
	}
	paths, err = newKimiPluginAdapter().ManagedPaths("", pluginmodel.PackageGraph{}, pluginmodel.NewTargetState("kimi"))
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 0 {
		t.Fatalf("kimi managed paths = %v; inline layout never owns a sidecar", paths)
	}
}

func symlinkOrSkip(t *testing.T, oldname, newname string) {
	t.Helper()
	if err := os.Symlink(oldname, newname); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
}

func TestImportRejectsSymlinkedBundleFiles(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "secret.txt")
	if err := os.WriteFile(outside, []byte("host secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".grok-plugin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".grok-plugin", "plugin.json"), []byte(`{"name":"demo","mcpServers":"./.mcp.json"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	symlinkOrSkip(t, outside, filepath.Join(root, ".mcp.json"))
	_, err := newGrokPluginAdapter().Import(root, ImportSeed{
		Manifest: pluginmodel.Manifest{Name: "demo", Targets: []string{"grok"}},
	})
	if err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("symlinked MCP sidecar must be rejected, err = %v", err)
	}

	skillDir := filepath.Join(root, "skills", "release-checks")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	symlinkOrSkip(t, outside, filepath.Join(skillDir, "SKILL.md"))
	if err := os.Remove(filepath.Join(root, ".mcp.json")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".grok-plugin", "plugin.json"), []byte(`{"name":"demo"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = newGrokPluginAdapter().Import(root, ImportSeed{
		Manifest: pluginmodel.Manifest{Name: "demo", Targets: []string{"grok"}},
	})
	if err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("symlinked skill file must be rejected, err = %v", err)
	}
	if strings.Contains(err.Error(), root) {
		t.Fatalf("bundle read errors must not leak host paths, err = %v", err)
	}
}

func TestImportRejectsSymlinkedBundleDirectory(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	outsideDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(outsideDir, "config.json"), []byte(`{"mcpServers":{"leak":{"command":"host"}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".grok-plugin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".grok-plugin", "plugin.json"), []byte(`{"name":"demo","mcpServers":"./evil/config.json"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	symlinkOrSkip(t, outsideDir, filepath.Join(root, "evil"))
	_, err := newGrokPluginAdapter().Import(root, ImportSeed{
		Manifest: pluginmodel.Manifest{Name: "demo", Targets: []string{"grok"}},
	})
	if err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("mcpServers ref through a symlinked directory must be rejected, err = %v", err)
	}
	if strings.Contains(err.Error(), outsideDir) {
		t.Fatalf("bundle read errors must not leak host paths, err = %v", err)
	}
}

func TestGenerateRejectsSymlinkedSkillSource(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "secret.md")
	if err := os.WriteFile(outside, []byte("host secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	skillDir := filepath.Join(root, "plugin", "skills", "release-checks")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	symlinkOrSkip(t, outside, filepath.Join(skillDir, "SKILL.md"))
	_, err := portableSkillArtifact(root, filepath.Join("plugin", "skills", "release-checks", "SKILL.md"), "skills")
	if err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Fatalf("generate must not read symlinked skill sources, err = %v", err)
	}
}

func TestReadBundleFileEnforcesSizeLimit(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "big.json"), bytes.Repeat([]byte("a"), maxBundleFileBytes+1), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := readBundleFile(root, "big.json", "big.json"); err == nil || !strings.Contains(err.Error(), "bundle file limit") {
		t.Fatalf("oversized bundle file must be rejected, err = %v", err)
	}
}

func TestValidateRejectsUnsupportedManifestFields(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".grok-plugin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".grok-plugin", "plugin.json"), []byte(`{"name":"grok-demo","version":"0.1.0","description":"grok demo","hooks":{"cmd":"x"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	diagnostics, err := newGrokPluginAdapter().Validate(root, packagedPluginTestGraph(t, root, "grok"), pluginmodel.NewTargetState("grok"))
	if err != nil {
		t.Fatal(err)
	}
	joined := diagnosticsText(diagnostics)
	if !strings.Contains(joined, "unsupported fields: hooks") {
		t.Fatalf("unknown managed-manifest fields must fail validate, diagnostics = %s", joined)
	}
}

func TestDecodeJSONObjectRejectsDuplicateKeys(t *testing.T) {
	t.Parallel()
	if _, err := decodeJSONObject([]byte(`{"name":"a","name":"b"}`), "doc"); err == nil || !strings.Contains(err.Error(), "duplicate object key") {
		t.Fatalf("duplicate JSON keys must be rejected, err = %v", err)
	}
	if _, err := decodeJSONObject([]byte(`{"servers":[{"x":1,"x":2}]}`), "doc"); err == nil {
		t.Fatal("duplicate nested keys must be rejected")
	}
	if _, err := decodeJSONObject([]byte(`{"servers":[{"x":1},{"x":2}],"y":"a"}`), "doc"); err != nil {
		t.Fatalf("distinct keys must stay valid: %v", err)
	}
}

func TestGrokImportTreatsEmptyMCPProjectionAsAbsent(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".grok-plugin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".grok-plugin", "plugin.json"), []byte(`{"name":"demo","mcpServers":"./.mcp.json"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".mcp.json"), []byte(`{"mcpServers":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := newGrokPluginAdapter().Import(root, ImportSeed{
		Manifest: pluginmodel.Manifest{Name: "demo", Targets: []string{"grok"}},
	})
	if err != nil {
		t.Fatalf("empty projection must import as absence: %v", err)
	}
	if len(result.Artifacts) != 0 {
		t.Fatalf("empty projection must not emit an MCP artifact, artifacts = %v", result.Artifacts)
	}
}

func TestImportRejectsDanglingMCPSidecarRef(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".grok-plugin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".grok-plugin", "plugin.json"), []byte(`{"name":"demo","mcpServers":"./.mcp.json"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := newGrokPluginAdapter().Import(root, ImportSeed{
		Manifest: pluginmodel.Manifest{Name: "demo", Targets: []string{"grok"}},
	})
	if err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Fatalf("dangling mcpServers ref must fail import, err = %v", err)
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
