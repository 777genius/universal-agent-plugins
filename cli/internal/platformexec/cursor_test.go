package platformexec

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

func TestCursorDetectNativeIgnoresStandaloneRootAgents(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("# Shared agents\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if (cursorWorkspaceAdapter{}).DetectNative(root) {
		t.Fatal("DetectNative unexpectedly matched standalone root AGENTS.md")
	}
}

func TestCursorWorkspaceImportIncludeUserScopeMergesGlobalMCP(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".cursor"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".cursor", "mcp.json"), []byte(`{"global":{"command":"node"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	imported, err := (cursorWorkspaceAdapter{}).Import(root, ImportSeed{
		Manifest:         pluginmodel.Manifest{Name: "demo", Version: "0.1.0", Description: "demo", Targets: []string{"cursor-workspace"}},
		IncludeUserScope: true,
	})
	if err != nil {
		t.Fatalf("Import error = %v", err)
	}
	if len(imported.Artifacts) != 1 {
		t.Fatalf("artifacts = %+v", imported.Artifacts)
	}
	if !strings.Contains(string(imported.Artifacts[0].Content), "global:") {
		t.Fatalf("portable MCP import missing global server:\n%s", imported.Artifacts[0].Content)
	}
}

func TestCursorWorkspaceImportExtractsManagedAgentsSection(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	body := "# Shared root\n\n" + cursorAgentsSectionStart + "\n# Managed agents\nUse managed content.\n" + cursorAgentsSectionEnd + "\n"
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	imported, err := (cursorWorkspaceAdapter{}).Import(root, ImportSeed{
		Manifest: pluginmodel.Manifest{Name: "demo", Version: "0.1.0", Description: "demo", Targets: []string{"cursor-workspace"}},
	})
	if err != nil {
		t.Fatalf("Import error = %v", err)
	}
	if len(imported.Artifacts) != 1 {
		t.Fatalf("artifacts = %+v", imported.Artifacts)
	}
	if imported.Artifacts[0].RelPath != filepath.ToSlash(filepath.Join("targets", "cursor-workspace", "AGENTS.md")) {
		t.Fatalf("artifact path = %q", imported.Artifacts[0].RelPath)
	}
	got := string(imported.Artifacts[0].Content)
	if strings.Contains(got, "Shared root") || !strings.Contains(got, "Managed agents") {
		t.Fatalf("managed agents content = %q", got)
	}
}

func TestCursorWorkspaceValidateRejectsNonMdcRules(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "targets", "cursor-workspace", "rules"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "targets", "cursor-workspace", "rules", "project.md"), []byte("# bad\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	diagnostics, err := (cursorWorkspaceAdapter{}).Validate(root, pluginmodel.PackageGraph{
		Portable: pluginmodel.NewPortableComponents(),
	}, pluginmodel.TargetState{
		Target: "cursor-workspace",
		Components: map[string][]string{
			"rules": {filepath.ToSlash(filepath.Join("targets", "cursor-workspace", "rules", "project.md"))},
		},
		Docs: map[string]string{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) == 0 {
		t.Fatal("expected validation diagnostics")
	}
	var found bool
	for _, diagnostic := range diagnostics {
		if strings.Contains(diagnostic.Message, ".mdc") {
			found = true
		}
	}
	if !found {
		t.Fatalf("diagnostics = %+v", diagnostics)
	}
}

func TestCursorWorkspaceValidateRejectsTraversalOrSymlink(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	rulesDir := filepath.Join(root, "targets", "cursor-workspace", "rules")
	if err := os.MkdirAll(rulesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rulesDir, "real.mdc"), []byte("content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("real.mdc", filepath.Join(rulesDir, "linked.mdc")); err != nil {
		t.Fatal(err)
	}
	diagnostics, err := (cursorWorkspaceAdapter{}).Validate(root, pluginmodel.PackageGraph{
		Portable: pluginmodel.NewPortableComponents(),
	}, pluginmodel.TargetState{
		Target: "cursor-workspace",
		Components: map[string][]string{
			"rules": {
				"targets/cursor-workspace/rules/../escape.mdc",
				filepath.ToSlash(filepath.Join("targets", "cursor-workspace", "rules", "linked.mdc")),
			},
		},
		Docs: map[string]string{},
	})
	if err != nil {
		t.Fatal(err)
	}
	var foundTraversal, foundSymlink bool
	for _, diagnostic := range diagnostics {
		if strings.Contains(diagnostic.Message, "path traversal") {
			foundTraversal = true
		}
		if strings.Contains(diagnostic.Message, "must not be a symlink") {
			foundSymlink = true
		}
	}
	if !foundTraversal || !foundSymlink {
		t.Fatalf("diagnostics = %+v", diagnostics)
	}
}

func TestCursorWorkspaceMCPPreservesInterpolationStrings(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	parsed, err := pluginmodel.ParsePortableMCP("mcp/servers.yaml", []byte(`api_version: v1

servers:
  demo:
    type: stdio
    stdio:
      command: env
      args:
        - "API_KEY=${env:API_KEY}"
        - "ROOT=${workspaceFolder}"
        - "TOKEN=${input:token}"
`))
	if err != nil {
		t.Fatal(err)
	}
	graph := pluginmodel.PackageGraph{
		Portable: pluginmodel.PortableComponents{
			Items: map[string][]string{},
			MCP:   &pluginmodel.PortableMCP{Path: "mcp/servers.yaml", Servers: parsed.Servers, File: parsed.File},
		},
	}
	artifacts, err := (cursorWorkspaceAdapter{}).Generate(root, graph, pluginmodel.NewTargetState("cursor-workspace"))
	if err != nil {
		t.Fatal(err)
	}
	if len(artifacts) != 1 {
		t.Fatalf("artifacts = %v", artifacts)
	}
	var got string
	for _, artifact := range artifacts {
		if artifact.RelPath == filepath.ToSlash(filepath.Join(".cursor", "mcp.json")) {
			got = string(artifact.Content)
			break
		}
	}
	if got == "" {
		t.Fatalf("artifacts missing .cursor/mcp.json: %+v", artifacts)
	}
	for _, want := range []string{"${env:API_KEY}", "${workspaceFolder}", "${input:token}"} {
		if !strings.Contains(got, want) {
			t.Fatalf("generated mcp missing %q:\n%s", want, got)
		}
	}
}

func TestCursorPackagedGenerateWritesManagedPluginArtifacts(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
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
	graph := pluginmodel.PackageGraph{
		Manifest: pluginmodel.Manifest{Name: "cursor-demo", Version: "0.1.0", Description: "cursor demo", Targets: []string{"cursor"}},
		Portable: pluginmodel.PortableComponents{
			Items: map[string][]string{
				"skills": {filepath.ToSlash(filepath.Join("plugin", "skills", "release-checks", "SKILL.md"))},
			},
			MCP: &pluginmodel.PortableMCP{Path: filepath.ToSlash(filepath.Join("plugin", "mcp", "servers.yaml")), Servers: parsed.Servers, File: parsed.File},
		},
	}
	artifacts, err := (cursorAdapter{}).Generate(root, graph, pluginmodel.NewTargetState("cursor"))
	if err != nil {
		t.Fatal(err)
	}
	var sawManifest, sawMCP, sawSkill bool
	for _, artifact := range artifacts {
		switch artifact.RelPath {
		case cursorPluginManifestPath:
			sawManifest = true
			if !strings.Contains(string(artifact.Content), `"skills": "./skills/"`) {
				t.Fatalf("plugin manifest missing managed skills ref:\n%s", artifact.Content)
			}
			if !strings.Contains(string(artifact.Content), `"mcpServers": "./.mcp.json"`) {
				t.Fatalf("plugin manifest missing managed MCP ref:\n%s", artifact.Content)
			}
		case ".mcp.json":
			sawMCP = true
			if !strings.Contains(string(artifact.Content), `"docs"`) || strings.Contains(string(artifact.Content), `"mcpServers"`) {
				t.Fatalf("packaged Cursor MCP sidecar should be the shared direct-object shape:\n%s", artifact.Content)
			}
		case filepath.ToSlash(filepath.Join("skills", "release-checks", "SKILL.md")):
			sawSkill = true
		}
	}
	if !sawManifest || !sawMCP || !sawSkill {
		t.Fatalf("artifacts = %+v", artifacts)
	}
}

func TestCursorPackagedValidateRequiresManagedSkillsRef(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".cursor-plugin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "plugin", "skills", "release-checks"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "skills", "release-checks"), 0o755); err != nil {
		t.Fatal(err)
	}
	skillBody := []byte("---\nname: release-checks\ndescription: release checks\n---\n\nUse this skill.\n")
	if err := os.WriteFile(filepath.Join(root, "plugin", "skills", "release-checks", "SKILL.md"), skillBody, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "skills", "release-checks", "SKILL.md"), skillBody, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".cursor-plugin", "plugin.json"), []byte(`{"name":"cursor-demo","version":"0.1.0","description":"cursor demo"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	diagnostics, err := (cursorAdapter{}).Validate(root, pluginmodel.PackageGraph{
		Manifest: pluginmodel.Manifest{Name: "cursor-demo", Version: "0.1.0", Description: "cursor demo", Targets: []string{"cursor"}},
		Portable: pluginmodel.PortableComponents{
			Items: map[string][]string{
				"skills": {filepath.ToSlash(filepath.Join("plugin", "skills", "release-checks", "SKILL.md"))},
			},
		},
	}, pluginmodel.NewTargetState("cursor"))
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) == 0 || !strings.Contains(diagnostics[0].Message, `must reference "./skills/"`) {
		t.Fatalf("diagnostics = %+v", diagnostics)
	}
}

func TestCursorPackagedValidateRejectsSkillsWithoutPortableSkills(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".cursor-plugin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".cursor-plugin", "plugin.json"), []byte(`{"name":"cursor-demo","version":"0.1.0","description":"cursor demo","skills":"./skills/"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	diagnostics, err := (cursorAdapter{}).Validate(root, pluginmodel.PackageGraph{
		Manifest: pluginmodel.Manifest{Name: "cursor-demo", Version: "0.1.0", Description: "cursor demo", Targets: []string{"cursor"}},
	}, pluginmodel.NewTargetState("cursor"))
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) == 0 || !strings.Contains(diagnostics[0].Message, "may not define skills when portable skills are absent") {
		t.Fatalf("diagnostics = %+v", diagnostics)
	}
}

func TestCursorPackagedValidateChecksGeneratedSkillProjection(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".cursor-plugin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "plugin", "skills", "release-checks"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "skills", "release-checks"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "plugin", "skills", "release-checks", "SKILL.md"), []byte("---\nname: release-checks\ndescription: release checks\n---\n\nSource.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "skills", "release-checks", "SKILL.md"), []byte("---\nname: release-checks\ndescription: release checks\n---\n\nGenerated drift.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".cursor-plugin", "plugin.json"), []byte(`{"name":"cursor-demo","version":"0.1.0","description":"cursor demo","skills":"./skills/"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	diagnostics, err := (cursorAdapter{}).Validate(root, pluginmodel.PackageGraph{
		Manifest: pluginmodel.Manifest{Name: "cursor-demo", Version: "0.1.0", Description: "cursor demo", Targets: []string{"cursor"}},
		Portable: pluginmodel.PortableComponents{
			Items: map[string][]string{
				"skills": {filepath.ToSlash(filepath.Join("plugin", "skills", "release-checks", "SKILL.md"))},
			},
		},
	}, pluginmodel.NewTargetState("cursor"))
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) == 0 || !strings.Contains(diagnosticsText(diagnostics), "does not match authored portable skill") {
		t.Fatalf("diagnostics = %+v", diagnostics)
	}
}

func TestCursorPackagedValidateRequiresManagedMCPRef(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".cursor-plugin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".cursor-plugin", "plugin.json"), []byte(`{"name":"cursor-demo","version":"0.1.0","description":"cursor demo","mcpServers":"./config/mcp.json"}`), 0o644); err != nil {
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
	diagnostics, err := (cursorAdapter{}).Validate(root, pluginmodel.PackageGraph{
		Manifest: pluginmodel.Manifest{Name: "cursor-demo", Version: "0.1.0", Description: "cursor demo", Targets: []string{"cursor"}},
		Portable: pluginmodel.PortableComponents{
			MCP: &pluginmodel.PortableMCP{
				Path:    filepath.ToSlash(filepath.Join("plugin", "mcp", "servers.yaml")),
				Servers: parsed.Servers,
				File:    parsed.File,
			},
		},
	}, pluginmodel.NewTargetState("cursor"))
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) == 0 || !strings.Contains(diagnostics[0].Message, `must use "./.mcp.json"`) {
		t.Fatalf("diagnostics = %+v", diagnostics)
	}
}

func TestCursorPackagedValidateRejectsMCPServersWithoutPortableMCP(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".cursor-plugin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".cursor-plugin", "plugin.json"), []byte(`{"name":"cursor-demo","version":"0.1.0","description":"cursor demo","mcpServers":"./.mcp.json"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	diagnostics, err := (cursorAdapter{}).Validate(root, pluginmodel.PackageGraph{
		Manifest: pluginmodel.Manifest{Name: "cursor-demo", Version: "0.1.0", Description: "cursor demo", Targets: []string{"cursor"}},
	}, pluginmodel.NewTargetState("cursor"))
	if err != nil {
		t.Fatal(err)
	}
	if len(diagnostics) == 0 || !strings.Contains(diagnostics[0].Message, "may not define mcpServers when portable MCP is absent") {
		t.Fatalf("diagnostics = %+v", diagnostics)
	}
}
