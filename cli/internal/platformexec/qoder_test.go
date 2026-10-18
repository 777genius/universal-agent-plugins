package platformexec

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

func TestQoderDetectNativeMatchesPluginManifest(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".qoder-plugin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".qoder-plugin", "plugin.json"), []byte(`{"name":"qoder-demo"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if !(qoderAdapter{}).DetectNative(root) {
		t.Fatal("DetectNative did not match .qoder-plugin/plugin.json")
	}
}

func TestQoderDetectNativeIgnoresStandaloneRootFiles(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".mcp.json"), []byte(`{"mcpServers":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("# Shared agents\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if (qoderAdapter{}).DetectNative(root) {
		t.Fatal("DetectNative unexpectedly matched standalone root files")
	}
}

func TestQoderPackagedGenerateWritesManagedPluginArtifacts(t *testing.T) {
	t.Parallel()
	root, graph := qoderGenerateFixture(t)
	artifacts, err := (qoderAdapter{}).Generate(root, graph, pluginmodel.NewTargetState("qoder"))
	if err != nil {
		t.Fatal(err)
	}
	var sawManifest, sawMCP, sawSkill bool
	for _, artifact := range artifacts {
		switch artifact.RelPath {
		case qoderPluginManifestPath:
			sawManifest = true
			for _, want := range []string{`"name": "qoder-demo"`, `"author": {`, `"name": "Ada"`, `"skills": "./skills/"`, `"mcpServers": "./.mcp.json"`} {
				if !strings.Contains(string(artifact.Content), want) {
					t.Fatalf("plugin manifest missing %q:\n%s", want, artifact.Content)
				}
			}
		case ".mcp.json":
			sawMCP = true
			if !strings.Contains(string(artifact.Content), `"mcpServers": {`) || !strings.Contains(string(artifact.Content), `"docs"`) {
				t.Fatalf("packaged Qoder MCP sidecar should use the wrapped shape:\n%s", artifact.Content)
			}
		case filepath.ToSlash(filepath.Join("skills", "release-checks", "SKILL.md")):
			sawSkill = true
		}
	}
	if !sawManifest || !sawMCP || !sawSkill {
		t.Fatalf("artifacts = %+v", artifacts)
	}
}

func TestQoderPackagedGenerateValidateRoundTrip(t *testing.T) {
	t.Parallel()
	root, graph := qoderGenerateFixture(t)
	artifacts, err := (qoderAdapter{}).Generate(root, graph, pluginmodel.NewTargetState("qoder"))
	if err != nil {
		t.Fatal(err)
	}
	writeQoderArtifacts(t, root, artifacts)
	diagnostics, err := (qoderAdapter{}).Validate(root, graph, pluginmodel.NewTargetState("qoder"))
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics = %+v", diagnostics)
	}
}

func TestQoderPackagedImportRoundTrip(t *testing.T) {
	t.Parallel()
	root, graph := qoderGenerateFixture(t)
	artifacts, err := (qoderAdapter{}).Generate(root, graph, pluginmodel.NewTargetState("qoder"))
	if err != nil {
		t.Fatal(err)
	}
	writeQoderArtifacts(t, root, artifacts)
	imported, err := (qoderAdapter{}).Import(root, ImportSeed{
		Manifest: pluginmodel.Manifest{Name: "demo", Version: "0.1.0", Description: "demo", Targets: []string{"qoder"}},
	})
	if err != nil {
		t.Fatalf("Import error = %v", err)
	}
	if imported.Manifest.Name != "qoder-demo" || imported.Manifest.Version != "0.1.0" || imported.Manifest.Description != "qoder demo" {
		t.Fatalf("imported manifest = %+v", imported.Manifest)
	}
	if imported.Manifest.Author == nil || imported.Manifest.Author.Name != "Ada" {
		t.Fatalf("imported author = %+v", imported.Manifest.Author)
	}
	paths := map[string]bool{}
	for _, artifact := range imported.Artifacts {
		paths[artifact.RelPath] = true
	}
	for _, want := range []string{
		filepath.ToSlash(filepath.Join("skills", "release-checks", "SKILL.md")),
		filepath.ToSlash(filepath.Join("plugin", "mcp", "servers.yaml")),
	} {
		if !paths[want] {
			t.Fatalf("imported artifacts missing %q: %+v", want, imported.Artifacts)
		}
	}
	if text := warningsText(imported.Warnings); text != "" {
		t.Fatalf("unexpected warnings: %s", text)
	}
}

func TestQoderPackagedImportWarnsForUnsupportedAssets(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeQoderTestFile(t, filepath.Join(root, ".qoder-plugin", "plugin.json"), `{"name":"qoder-demo","version":"0.1.0","description":"qoder demo"}`)
	for _, asset := range []string{"commands", "agents", "hooks", "output-styles", "bin"} {
		if err := os.MkdirAll(filepath.Join(root, asset), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeQoderTestFile(t, filepath.Join(root, "hooks", "hooks.json"), "{}\n")
	writeQoderTestFile(t, filepath.Join(root, "commands", "ship.md"), "# ship\n")

	imported, err := (qoderAdapter{}).Import(root, ImportSeed{
		Manifest: pluginmodel.Manifest{Name: "demo", Version: "0.1.0", Description: "demo", Targets: []string{"qoder"}},
	})
	if err != nil {
		t.Fatalf("Import error = %v", err)
	}
	text := warningsText(imported.Warnings)
	for _, asset := range []string{"commands", "agents", "hooks", "output-styles", "bin"} {
		if !strings.Contains(text, "ignored unsupported import asset: "+asset) {
			t.Fatalf("warnings missing dropped kind %q: %s", asset, text)
		}
	}
	for _, artifact := range imported.Artifacts {
		if strings.HasPrefix(artifact.RelPath, "commands/") || strings.HasPrefix(artifact.RelPath, "agents/") || strings.HasPrefix(artifact.RelPath, "hooks/") {
			t.Fatalf("unsupported asset imported: %+v", artifact)
		}
	}
}

func TestQoderPackagedImportRequiresPluginManifest(t *testing.T) {
	t.Parallel()
	_, err := (qoderAdapter{}).Import(t.TempDir(), ImportSeed{
		Manifest: pluginmodel.Manifest{Name: "demo", Version: "0.1.0", Description: "demo", Targets: []string{"qoder"}},
	})
	if err == nil || !strings.Contains(err.Error(), qoderPluginManifestPath) {
		t.Fatalf("Import error = %v", err)
	}
}

func TestQoderPackagedValidateRequiresManifestName(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeQoderTestFile(t, filepath.Join(root, ".qoder-plugin", "plugin.json"), `{"version":"0.1.0","description":"qoder demo"}`)
	diagnostics, err := (qoderAdapter{}).Validate(root, pluginmodel.PackageGraph{
		Manifest: pluginmodel.Manifest{Name: "qoder-demo", Version: "0.1.0", Description: "qoder demo", Targets: []string{"qoder"}},
	}, pluginmodel.NewTargetState("qoder"))
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) == 0 || !strings.Contains(diagnostics[0].Message, `must define required field "name"`) {
		t.Fatalf("diagnostics = %+v", diagnostics)
	}
}

func TestQoderPackagedValidateChecksAuthorIdentity(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeQoderTestFile(t, filepath.Join(root, ".qoder-plugin", "plugin.json"), `{"name":"qoder-demo","version":"0.1.0","description":"qoder demo","author":{"name":"Eve"}}`)
	diagnostics, err := (qoderAdapter{}).Validate(root, pluginmodel.PackageGraph{
		Manifest: pluginmodel.Manifest{Name: "qoder-demo", Version: "0.1.0", Description: "qoder demo", Author: &pluginmodel.Author{Name: "Ada"}, Targets: []string{"qoder"}},
	}, pluginmodel.NewTargetState("qoder"))
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) == 0 || !strings.Contains(diagnostics[0].Message, "does not match author from plugin.yaml") {
		t.Fatalf("diagnostics = %+v", diagnostics)
	}
}

func TestQoderPackagedValidateRequiresManagedSkillsRef(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeQoderTestFile(t, filepath.Join(root, ".qoder-plugin", "plugin.json"), `{"name":"qoder-demo","version":"0.1.0","description":"qoder demo"}`)
	writeQoderTestFile(t, filepath.Join(root, "plugin", "skills", "release-checks", "SKILL.md"), "---\nname: release-checks\ndescription: release checks\n---\n\nUse this skill.\n")
	diagnostics, err := (qoderAdapter{}).Validate(root, pluginmodel.PackageGraph{
		Manifest: pluginmodel.Manifest{Name: "qoder-demo", Version: "0.1.0", Description: "qoder demo", Targets: []string{"qoder"}},
		Portable: pluginmodel.PortableComponents{
			Items: map[string][]string{
				"skills": {filepath.ToSlash(filepath.Join("plugin", "skills", "release-checks", "SKILL.md"))},
			},
		},
	}, pluginmodel.NewTargetState("qoder"))
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) == 0 || !strings.Contains(diagnostics[0].Message, `must reference "./skills/"`) {
		t.Fatalf("diagnostics = %+v", diagnostics)
	}
}

func TestQoderPackagedValidateRejectsSkillsWithoutPortableSkills(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeQoderTestFile(t, filepath.Join(root, ".qoder-plugin", "plugin.json"), `{"name":"qoder-demo","version":"0.1.0","description":"qoder demo","skills":"./skills/"}`)
	diagnostics, err := (qoderAdapter{}).Validate(root, pluginmodel.PackageGraph{
		Manifest: pluginmodel.Manifest{Name: "qoder-demo", Version: "0.1.0", Description: "qoder demo", Targets: []string{"qoder"}},
	}, pluginmodel.NewTargetState("qoder"))
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) == 0 || !strings.Contains(diagnostics[0].Message, "may not define skills when portable skills are absent") {
		t.Fatalf("diagnostics = %+v", diagnostics)
	}
}

func TestQoderPackagedValidateChecksGeneratedSkillProjection(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeQoderTestFile(t, filepath.Join(root, ".qoder-plugin", "plugin.json"), `{"name":"qoder-demo","version":"0.1.0","description":"qoder demo","skills":"./skills/"}`)
	writeQoderTestFile(t, filepath.Join(root, "plugin", "skills", "release-checks", "SKILL.md"), "---\nname: release-checks\ndescription: release checks\n---\n\nSource.\n")
	writeQoderTestFile(t, filepath.Join(root, "skills", "release-checks", "SKILL.md"), "---\nname: release-checks\ndescription: release checks\n---\n\nGenerated drift.\n")
	diagnostics, err := (qoderAdapter{}).Validate(root, pluginmodel.PackageGraph{
		Manifest: pluginmodel.Manifest{Name: "qoder-demo", Version: "0.1.0", Description: "qoder demo", Targets: []string{"qoder"}},
		Portable: pluginmodel.PortableComponents{
			Items: map[string][]string{
				"skills": {filepath.ToSlash(filepath.Join("plugin", "skills", "release-checks", "SKILL.md"))},
			},
		},
	}, pluginmodel.NewTargetState("qoder"))
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) == 0 || !strings.Contains(diagnosticsText(diagnostics), "does not match authored portable skill") {
		t.Fatalf("diagnostics = %+v", diagnostics)
	}
}

func TestQoderPackagedValidateRequiresManagedMCPRef(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeQoderTestFile(t, filepath.Join(root, ".qoder-plugin", "plugin.json"), `{"name":"qoder-demo","version":"0.1.0","description":"qoder demo","mcpServers":"./config/mcp.json"}`)
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
	diagnostics, err := (qoderAdapter{}).Validate(root, pluginmodel.PackageGraph{
		Manifest: pluginmodel.Manifest{Name: "qoder-demo", Version: "0.1.0", Description: "qoder demo", Targets: []string{"qoder"}},
		Portable: pluginmodel.PortableComponents{
			MCP: &pluginmodel.PortableMCP{
				Path:    filepath.ToSlash(filepath.Join("plugin", "mcp", "servers.yaml")),
				Servers: parsed.Servers,
				File:    parsed.File,
			},
		},
	}, pluginmodel.NewTargetState("qoder"))
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) == 0 || !strings.Contains(diagnostics[0].Message, `must use "./.mcp.json"`) {
		t.Fatalf("diagnostics = %+v", diagnostics)
	}
}

func TestQoderPackagedValidateRejectsMCPServersWithoutPortableMCP(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeQoderTestFile(t, filepath.Join(root, ".qoder-plugin", "plugin.json"), `{"name":"qoder-demo","version":"0.1.0","description":"qoder demo","mcpServers":"./.mcp.json"}`)
	diagnostics, err := (qoderAdapter{}).Validate(root, pluginmodel.PackageGraph{
		Manifest: pluginmodel.Manifest{Name: "qoder-demo", Version: "0.1.0", Description: "qoder demo", Targets: []string{"qoder"}},
	}, pluginmodel.NewTargetState("qoder"))
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) == 0 || !strings.Contains(diagnostics[0].Message, "may not define mcpServers when portable MCP is absent") {
		t.Fatalf("diagnostics = %+v", diagnostics)
	}
}

func qoderGenerateFixture(t *testing.T) (string, pluginmodel.PackageGraph) {
	t.Helper()
	root := t.TempDir()
	writeQoderTestFile(t, filepath.Join(root, "plugin", "skills", "release-checks", "SKILL.md"), "---\nname: release-checks\ndescription: release checks\n---\n\nUse this skill.\n")
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
		Manifest: pluginmodel.Manifest{
			Name:        "qoder-demo",
			Version:     "0.1.0",
			Description: "qoder demo",
			Author:      &pluginmodel.Author{Name: "Ada"},
			Targets:     []string{"qoder"},
		},
		Portable: pluginmodel.PortableComponents{
			Items: map[string][]string{
				"skills": {filepath.ToSlash(filepath.Join("plugin", "skills", "release-checks", "SKILL.md"))},
			},
			MCP: &pluginmodel.PortableMCP{
				Path:    filepath.ToSlash(filepath.Join("plugin", "mcp", "servers.yaml")),
				Servers: parsed.Servers,
				File:    parsed.File,
			},
		},
	}
	return root, graph
}

func writeQoderArtifacts(t *testing.T, root string, artifacts []pluginmodel.Artifact) {
	t.Helper()
	for _, artifact := range artifacts {
		writeQoderTestFile(t, filepath.Join(root, filepath.FromSlash(artifact.RelPath)), string(artifact.Content))
	}
}

func writeQoderTestFile(t *testing.T, path string, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
