package platformexec

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

func TestClaudeImportNormalizesCustomPathsAndInfersLauncher(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeClaudeTestFile(t, filepath.Join(root, ".claude-plugin", "plugin.json"), `{
  "name": "claude-demo",
  "version": "0.1.0",
  "description": "claude demo",
  "skills": "./custom/skills/",
  "commands": "./custom/commands/",
  "agents": "./custom/agents/",
  "hooks": "./custom/hooks.json"
}`)
	writeClaudeTestFile(t, filepath.Join(root, "custom", "skills", "release-checks", "SKILL.md"), "# Skill\n")
	writeClaudeTestFile(t, filepath.Join(root, "custom", "commands", "deploy.md"), "# Deploy\n")
	writeClaudeTestFile(t, filepath.Join(root, "custom", "agents", "reviewer.md"), "# Reviewer\n")
	writeClaudeTestFile(t, filepath.Join(root, "custom", "hooks.json"), "{\n  \"hooks\": {\n    \"Stop\": [{\"hooks\": [{\"type\": \"command\", \"command\": \"./bin/demo Stop\"}]}]\n  }\n}\n")

	imported, err := (claudeAdapter{}).Import(root, ImportSeed{
		Manifest: pluginmodel.Manifest{
			Name:        "seed-name",
			Version:     "0.0.1",
			Description: "seed-description",
			Targets:     []string{"claude"},
		},
	})
	if err != nil {
		t.Fatalf("Import error = %v", err)
	}
	if imported.Launcher == nil || imported.Launcher.Entrypoint != "./bin/demo" {
		t.Fatalf("launcher = %+v", imported.Launcher)
	}
	for _, want := range []string{
		filepath.ToSlash(filepath.Join("skills", "release-checks", "SKILL.md")),
		filepath.ToSlash(filepath.Join(pluginmodel.SourceDirName, "targets", "claude", "commands", "deploy.md")),
		filepath.ToSlash(filepath.Join(pluginmodel.SourceDirName, "targets", "claude", "agents", "reviewer.md")),
		filepath.ToSlash(filepath.Join("targets", "claude", "hooks", "hooks.json")),
	} {
		if !hasArtifactPath(imported.Artifacts, want) {
			t.Fatalf("artifacts missing %s: %+v", want, imported.Artifacts)
		}
	}
	warnings := warningsText(imported.Warnings)
	for _, want := range []string{
		"custom Claude skills paths were normalized into canonical package-standard layout",
		"custom Claude commands paths were normalized into canonical package-standard layout",
		"custom Claude agents paths were normalized into canonical package-standard layout",
		"custom Claude hooks path was normalized into targets/claude/hooks/hooks.json",
		"normalized Claude plugin identity into canonical package-standard plugin.yaml",
	} {
		if !strings.Contains(warnings, want) {
			t.Fatalf("warnings missing %q:\n%s", want, warnings)
		}
	}
}

func TestClaudeImportMergesMultipleHookRefs(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeClaudeTestFile(t, filepath.Join(root, ".claude-plugin", "plugin.json"), `{
  "name": "claude-demo",
  "version": "0.1.0",
  "description": "claude demo",
  "hooks": ["./custom/stop.json", "./custom/notify.json"]
}`)
	writeClaudeTestFile(t, filepath.Join(root, "custom", "stop.json"), "{\n  \"hooks\": {\n    \"Stop\": [{\"hooks\": [{\"type\": \"command\", \"command\": \"./bin/demo Stop\"}]}]\n  }\n}\n")
	writeClaudeTestFile(t, filepath.Join(root, "custom", "notify.json"), "{\n  \"hooks\": {\n    \"Notification\": [{\"hooks\": [{\"type\": \"command\", \"command\": \"./bin/demo Notification\"}]}]\n  }\n}\n")

	imported, err := (claudeAdapter{}).Import(root, ImportSeed{
		Manifest: pluginmodel.Manifest{
			Name:        "claude-demo",
			Version:     "0.1.0",
			Description: "claude demo",
			Targets:     []string{"claude"},
		},
	})
	if err != nil {
		t.Fatalf("Import error = %v", err)
	}

	var hooksJSON string
	for _, artifact := range imported.Artifacts {
		if artifact.RelPath == filepath.ToSlash(filepath.Join("targets", "claude", "hooks", "hooks.json")) {
			hooksJSON = string(artifact.Content)
			break
		}
	}
	if hooksJSON == "" {
		t.Fatalf("hooks artifact missing: %+v", imported.Artifacts)
	}
	for _, want := range []string{`"Stop"`, `"Notification"`, `./bin/demo Stop`, `./bin/demo Notification`} {
		if !strings.Contains(hooksJSON, want) {
			t.Fatalf("hooks artifact missing %q:\n%s", want, hooksJSON)
		}
	}
}

func TestClaudeImportWarnsOnUnsupportedMixedHookArray(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeClaudeTestFile(t, filepath.Join(root, ".claude-plugin", "plugin.json"), `{
  "name": "claude-demo",
  "version": "0.1.0",
  "description": "claude demo",
  "hooks": ["./custom/stop.json", {"inline": true}]
}`)

	imported, err := (claudeAdapter{}).Import(root, ImportSeed{
		Manifest: pluginmodel.Manifest{
			Name:        "claude-demo",
			Version:     "0.1.0",
			Description: "claude demo",
			Targets:     []string{"claude"},
		},
	})
	if err != nil {
		t.Fatalf("Import error = %v", err)
	}
	if text := warningsText(imported.Warnings); !strings.Contains(text, `Claude manifest field "hooks" uses an unsupported mixed array shape; skipped during import normalization`) {
		t.Fatalf("warnings = %s", text)
	}
}

func TestClaudeImportPreservesUnsupportedManifestFields(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeClaudeTestFile(t, filepath.Join(root, ".claude-plugin", "plugin.json"), `{
  "name": "claude-demo",
  "version": "0.1.0",
  "description": "claude demo",
  "customFlag": true
}`)

	imported, err := (claudeAdapter{}).Import(root, ImportSeed{
		Manifest: pluginmodel.Manifest{
			Name:        "claude-demo",
			Version:     "0.1.0",
			Description: "claude demo",
			Targets:     []string{"claude"},
		},
	})
	if err != nil {
		t.Fatalf("Import error = %v", err)
	}
	extraPath := filepath.ToSlash(filepath.Join(pluginmodel.SourceDirName, "targets", "claude", "manifest.extra.json"))
	if !hasArtifactPath(imported.Artifacts, extraPath) {
		t.Fatalf("artifacts missing %s: %+v", extraPath, imported.Artifacts)
	}
	if text := warningsText(imported.Warnings); !strings.Contains(text, "preserved unsupported Claude manifest fields under targets/claude/manifest.extra.json") {
		t.Fatalf("warnings = %s", text)
	}
}

func TestClaudeImportNormalizesInlineMCPServersToPortableMCP(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeClaudeTestFile(t, filepath.Join(root, ".claude-plugin", "plugin.json"), `{
  "name": "claude-demo",
  "version": "0.1.0",
  "description": "claude demo",
  "mcpServers": {
    "docs": {
      "command": "node",
      "args": ["./bin/docs.js"]
    }
  }
}`)

	imported, err := (claudeAdapter{}).Import(root, ImportSeed{
		Manifest: pluginmodel.Manifest{
			Name:        "claude-demo",
			Version:     "0.1.0",
			Description: "claude demo",
			Targets:     []string{"claude"},
		},
	})
	if err != nil {
		t.Fatalf("Import error = %v", err)
	}
	mcpPath := filepath.ToSlash(filepath.Join(pluginmodel.SourceDirName, "mcp", "servers.yaml"))
	if !hasArtifactPath(imported.Artifacts, mcpPath) {
		t.Fatalf("artifacts missing %s: %+v", mcpPath, imported.Artifacts)
	}
	if text := warningsText(imported.Warnings); !strings.Contains(text, "inline Claude mcpServers were normalized into plugin/mcp/servers.yaml") {
		t.Fatalf("warnings = %s", text)
	}
}

func TestClaudeGeneratePackageOnlyModeSkipsGeneratedHooks(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeClaudeTestFile(t, filepath.Join(root, "plugin", "targets", "claude", "settings.json"), "{\n  \"agent\": \"reviewer\"\n}\n")

	state := pluginmodel.NewTargetState("claude")
	state.SetDoc("settings", filepath.Join("plugin", "targets", "claude", "settings.json"))
	artifacts, err := (claudeAdapter{}).Generate(root, pluginmodel.PackageGraph{
		Manifest: pluginmodel.Manifest{
			Name:        "claude-demo",
			Version:     "0.1.0",
			Description: "claude demo",
			Targets:     []string{"claude"},
		},
		Portable: pluginmodel.NewPortableComponents(),
	}, state)
	if err != nil {
		t.Fatalf("Generate error = %v", err)
	}
	if !hasArtifactPath(artifacts, filepath.ToSlash(filepath.Join(".claude-plugin", "plugin.json"))) {
		t.Fatalf("artifacts missing plugin manifest: %+v", artifacts)
	}
	if !hasArtifactPath(artifacts, "settings.json") {
		t.Fatalf("artifacts missing settings.json: %+v", artifacts)
	}
	if hasArtifactPath(artifacts, filepath.ToSlash(filepath.Join("hooks", "hooks.json"))) {
		t.Fatalf("unexpected generated hooks in package-only mode: %+v", artifacts)
	}
}

func TestClaudeValidateReportsHookEntrypointMismatchAndUserConfigShape(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeClaudeTestFile(t, filepath.Join(root, "plugin", "targets", "claude", "hooks", "hooks.json"), "{\n  \"hooks\": {\n    \"Stop\": [{\"hooks\": [{\"type\": \"command\", \"command\": \"./bin/other Stop\"}]}]\n  }\n}\n")
	writeClaudeTestFile(t, filepath.Join(root, "plugin", "targets", "claude", "user-config.json"), "{\n  \"bad\": true\n}\n")

	state := pluginmodel.NewTargetState("claude")
	state.AddComponent("hooks", filepath.Join("plugin", "targets", "claude", "hooks", "hooks.json"))
	state.SetDoc("user_config", filepath.Join("plugin", "targets", "claude", "user-config.json"))

	diagnostics, err := (claudeAdapter{}).Validate(root, pluginmodel.PackageGraph{
		Launcher: &pluginmodel.Launcher{Entrypoint: "./bin/demo"},
	}, state)
	if err != nil {
		t.Fatalf("Validate error = %v", err)
	}
	joined := diagnosticsText(diagnostics)
	for _, want := range []string{
		"entrypoint mismatch: Claude hook",
		"must be a JSON object",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("diagnostics missing %q:\n%s", want, joined)
		}
	}
}

func TestClaudeValidateRequiresLauncherWhenHooksAreAuthored(t *testing.T) {
	t.Parallel()
	state := pluginmodel.NewTargetState("claude")
	state.AddComponent("hooks", filepath.Join("plugin", "targets", "claude", "hooks", "hooks.json"))

	diagnostics, err := (claudeAdapter{}).Validate(t.TempDir(), pluginmodel.PackageGraph{}, state)
	if err != nil {
		t.Fatalf("Validate error = %v", err)
	}
	if text := diagnosticsText(diagnostics); !strings.Contains(text, "Claude hooks require plugin/launcher.yaml") {
		t.Fatalf("diagnostics = %s", text)
	}
}

func writeClaudeTestFile(t *testing.T, path string, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
