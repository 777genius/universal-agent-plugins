package platformexec

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

func TestCodexPackageImportNormalizesManagedRefsAndPreservesExtras(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeCodexTestFile(t, filepath.Join(root, ".codex-plugin", "plugin.json"), `{
  "name": "codex-demo",
  "version": "0.1.0",
  "description": "codex demo",
  "skills": "./resources/skills/",
  "mcpServers": "./config/mcp.json",
  "apps": "./config/app.json",
  "x-custom": true
}`)
	writeCodexTestFile(t, filepath.Join(root, "resources", "skills", "release-checks", "SKILL.md"), "# Skill\n")
	writeCodexTestFile(t, filepath.Join(root, "config", "app.json"), "{\n  \"entry\": \"open\"\n}\n")
	writeCodexTestFile(t, filepath.Join(root, "config", "mcp.json"), "{\n  \"docs\": {\"command\": \"node\"}\n}\n")
	writeCodexTestFile(t, filepath.Join(root, "hooks", "hooks.json"), `{
  "hooks": {
    "Stop": [
      {
        "hooks": [
          {"type": "command", "command": "./bin/demo Stop"}
        ]
      }
    ]
  }
}
`)

	imported, err := (codexPackageAdapter{}).Import(root, ImportSeed{
		Manifest: pluginmodel.Manifest{
			Name:        "seed-name",
			Version:     "0.0.1",
			Description: "seed-description",
			Targets:     []string{"codex-package"},
		},
	})
	if err != nil {
		t.Fatalf("Import error = %v", err)
	}
	if imported.Manifest.Name != "codex-demo" || imported.Manifest.Version != "0.1.0" || imported.Manifest.Description != "codex demo" {
		t.Fatalf("manifest = %+v", imported.Manifest)
	}
	for _, want := range []string{
		filepath.ToSlash(filepath.Join(pluginmodel.SourceDirName, "targets", "codex-package", "package.yaml")),
		filepath.ToSlash(filepath.Join(pluginmodel.SourceDirName, "targets", "codex-package", "app.json")),
		filepath.ToSlash(filepath.Join(pluginmodel.SourceDirName, "targets", "codex-package", "manifest.extra.json")),
		filepath.ToSlash(filepath.Join(pluginmodel.SourceDirName, "targets", "codex-package", "hooks", "hooks.json")),
		filepath.ToSlash(filepath.Join(pluginmodel.SourceDirName, "mcp", "servers.yaml")),
		filepath.ToSlash(filepath.Join("skills", "release-checks", "SKILL.md")),
	} {
		if !hasArtifactPath(imported.Artifacts, want) {
			t.Fatalf("artifacts missing %s: %+v", want, imported.Artifacts)
		}
	}
	warnings := warningsText(imported.Warnings)
	for _, want := range []string{
		"normalized Codex plugin apps path to the managed ./.app.json location",
		"normalized Codex plugin skills path to the managed ./skills/ location",
		"normalized Codex plugin mcpServers path to the managed ./.mcp.json location",
		"preserved unsupported Codex plugin manifest fields under targets/codex-package/manifest.extra.json",
	} {
		if !strings.Contains(warnings, want) {
			t.Fatalf("warnings missing %q:\n%s", want, warnings)
		}
	}
}

func TestCodexPackageImportWarnsOnIgnoredAgentsDirectory(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeCodexTestFile(t, filepath.Join(root, ".codex-plugin", "plugin.json"), `{
  "name": "codex-demo",
  "version": "0.1.0",
  "description": "codex demo"
}`)
	writeCodexTestFile(t, filepath.Join(root, "agents", "legacy.md"), "# legacy\n")

	imported, err := (codexPackageAdapter{}).Import(root, ImportSeed{
		Manifest: pluginmodel.Manifest{
			Name:        "seed-name",
			Version:     "0.0.1",
			Description: "seed-description",
			Targets:     []string{"codex-package"},
		},
	})
	if err != nil {
		t.Fatalf("Import error = %v", err)
	}
	if text := warningsText(imported.Warnings); !strings.Contains(text, "ignored unsupported import asset: agents") {
		t.Fatalf("warnings = %s", text)
	}
}

func TestCodexPackageGenerateWritesManagedBundleArtifacts(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeCodexTestFile(t, filepath.Join(root, "plugin", "targets", "codex-package", "interface.json"), "{\n  \"defaultPrompt\": [\"Ship it\"]\n}\n")
	writeCodexTestFile(t, filepath.Join(root, "plugin", "targets", "codex-package", "app.json"), "{\n  \"entry\": \"open\"\n}\n")
	writeCodexTestFile(t, filepath.Join(root, "plugin", "targets", "codex-package", "hooks", "hooks.json"), `{
  "hooks": {
    "Stop": [
      {
        "hooks": [
          {"type": "command", "command": "./bin/demo Stop"}
        ]
      }
    ]
  }
}
`)
	writeCodexTestFile(t, filepath.Join(root, "plugin", "skills", "release-checks", "SKILL.md"), "# Skill\n")

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
			Name:        "codex-demo",
			Version:     "0.1.0",
			Description: "codex demo",
			Targets:     []string{"codex-package"},
		},
		Portable: pluginmodel.PortableComponents{
			Items: map[string][]string{
				"skills": {filepath.ToSlash(filepath.Join("plugin", "skills", "release-checks", "SKILL.md"))},
			},
			MCP: &pluginmodel.PortableMCP{Path: filepath.ToSlash(filepath.Join("plugin", "mcp", "servers.yaml")), Servers: parsed.Servers, File: parsed.File},
		},
	}
	state := pluginmodel.NewTargetState("codex-package")
	state.SetDoc("interface", filepath.Join("plugin", "targets", "codex-package", "interface.json"))
	state.SetDoc("app_manifest", filepath.Join("plugin", "targets", "codex-package", "app.json"))
	state.AddComponent("hooks", filepath.Join("plugin", "targets", "codex-package", "hooks", "hooks.json"))

	artifacts, err := (codexPackageAdapter{}).Generate(root, graph, state)
	if err != nil {
		t.Fatalf("Generate error = %v", err)
	}
	var pluginJSON string
	for _, artifact := range artifacts {
		if artifact.RelPath == filepath.ToSlash(filepath.Join(".codex-plugin", "plugin.json")) {
			pluginJSON = string(artifact.Content)
		}
	}
	if pluginJSON == "" {
		t.Fatalf("artifacts missing plugin manifest: %+v", artifacts)
	}
	for _, want := range []string{`"skills": "./skills/"`, `"apps": "./.app.json"`, `"mcpServers": "./.mcp.json"`} {
		if !strings.Contains(pluginJSON, want) {
			t.Fatalf("plugin manifest missing %q:\n%s", want, pluginJSON)
		}
	}
	for _, want := range []string{
		filepath.ToSlash(filepath.Join(".codex-plugin", "plugin.json")),
		".app.json",
		".mcp.json",
		filepath.ToSlash(filepath.Join("hooks", "hooks.json")),
		filepath.ToSlash(filepath.Join("skills", "release-checks", "SKILL.md")),
	} {
		if !hasArtifactPath(artifacts, want) {
			t.Fatalf("artifacts missing %s: %+v", want, artifacts)
		}
	}
}

func TestCodexPackageValidateReportsInvalidHooks(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeCodexTestFile(t, filepath.Join(root, ".codex-plugin", "plugin.json"), "{\n  \"name\": \"codex-demo\",\n  \"version\": \"0.1.0\",\n  \"description\": \"codex demo\"\n}\n")
	writeCodexTestFile(t, filepath.Join(root, "plugin", "targets", "codex-package", "hooks", "hooks.json"), "{\n  \"Stop\": []\n}\n")

	state := pluginmodel.NewTargetState("codex-package")
	state.AddComponent("hooks", filepath.Join("plugin", "targets", "codex-package", "hooks", "hooks.json"))

	diagnostics, err := (codexPackageAdapter{}).Validate(root, pluginmodel.PackageGraph{
		Manifest: pluginmodel.Manifest{
			Name:        "codex-demo",
			Version:     "0.1.0",
			Description: "codex demo",
			Targets:     []string{"codex-package"},
		},
	}, state)
	if err != nil {
		t.Fatalf("Validate error = %v", err)
	}
	if joined := diagnosticsText(diagnostics); !strings.Contains(joined, `top-level "hooks" object`) {
		t.Fatalf("diagnostics missing hooks shape guidance:\n%s", joined)
	}
}

func TestCodexRuntimeValidateUsesModelHintAndConfigExtra(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeCodexTestFile(t, filepath.Join(root, ".codex", "config.toml"), "model = \"gpt-5.4-mini\"\nnotify = [\"./bin/demo\", \"announce\"]\napproval_policy = \"never\"\n")
	writeCodexTestFile(t, filepath.Join(root, "plugin", "targets", "codex-runtime", "package.yaml"), "model_hint: gpt-5.4\n")
	writeCodexTestFile(t, filepath.Join(root, "plugin", "targets", "codex-runtime", "config.extra.toml"), "approval_policy = \"always\"\n")

	state := pluginmodel.NewTargetState("codex-runtime")
	state.SetDoc("package_metadata", filepath.Join("plugin", "targets", "codex-runtime", "package.yaml"))
	state.SetDoc("config_extra", filepath.Join("plugin", "targets", "codex-runtime", "config.extra.toml"))

	diagnostics, err := (codexRuntimeAdapter{}).Validate(root, pluginmodel.PackageGraph{
		Launcher: &pluginmodel.Launcher{Entrypoint: "./bin/demo"},
	}, state)
	if err != nil {
		t.Fatalf("Validate error = %v", err)
	}
	joined := diagnosticsText(diagnostics)
	for _, want := range []string{
		"entrypoint mismatch",
		`expected model "gpt-5.4"`,
		"passthrough fields do not match",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("diagnostics missing %q:\n%s", want, joined)
		}
	}
}

func TestCodexRuntimeGenerateDefaultsModelHint(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeCodexTestFile(t, filepath.Join(root, "plugin", "targets", "codex-runtime", "commands", "announce.toml"), "description = \"announce\"\ncommand = \"echo\"\n")

	state := pluginmodel.NewTargetState("codex-runtime")

	artifacts, err := (codexRuntimeAdapter{}).Generate(root, pluginmodel.PackageGraph{
		Launcher: &pluginmodel.Launcher{Entrypoint: "./bin/demo"},
	}, state)
	if err != nil {
		t.Fatalf("Generate error = %v", err)
	}
	var config string
	for _, artifact := range artifacts {
		if artifact.RelPath == filepath.ToSlash(filepath.Join(".codex", "config.toml")) {
			config = string(artifact.Content)
			break
		}
	}
	if !strings.Contains(config, `model = "gpt-5.4-mini"`) {
		t.Fatalf("config missing default model hint:\n%s", config)
	}
}

func TestCodexPackageValidateRequiresManagedAppsRef(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeCodexTestFile(t, filepath.Join(root, ".codex-plugin", "plugin.json"), "{\n  \"name\": \"codex-demo\",\n  \"version\": \"0.1.0\",\n  \"description\": \"codex demo\",\n  \"apps\": \"./custom/app.json\"\n}\n")
	writeCodexTestFile(t, filepath.Join(root, "custom", "app.json"), "{\n  \"entry\": \"open\"\n}\n")
	writeCodexTestFile(t, filepath.Join(root, "plugin", "targets", "codex-package", "app.json"), "{\n  \"entry\": \"open\"\n}\n")

	state := pluginmodel.NewTargetState("codex-package")
	state.SetDoc("app_manifest", filepath.Join("plugin", "targets", "codex-package", "app.json"))

	diagnostics, err := (codexPackageAdapter{}).Validate(root, pluginmodel.PackageGraph{
		Manifest: pluginmodel.Manifest{
			Name:        "codex-demo",
			Version:     "0.1.0",
			Description: "codex demo",
			Targets:     []string{"codex-package"},
		},
	}, state)
	if err != nil {
		t.Fatalf("Validate error = %v", err)
	}
	joined := diagnosticsText(diagnostics)
	if !strings.Contains(joined, `must use "./.app.json" for apps when present`) {
		t.Fatalf("diagnostics missing managed apps ref guidance:\n%s", joined)
	}
}

func writeCodexTestFile(t *testing.T, path string, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func hasArtifactPath(artifacts []pluginmodel.Artifact, want string) bool {
	for _, artifact := range artifacts {
		if artifact.RelPath == want {
			return true
		}
	}
	return false
}

func warningsText(warnings []pluginmodel.Warning) string {
	var parts []string
	for _, warning := range warnings {
		parts = append(parts, warning.Message)
	}
	return strings.Join(parts, "\n")
}

func diagnosticsText(diagnostics []Diagnostic) string {
	var parts []string
	for _, diagnostic := range diagnostics {
		parts = append(parts, diagnostic.Message)
	}
	return strings.Join(parts, "\n")
}
