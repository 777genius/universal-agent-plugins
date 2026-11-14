package validate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmanifest"
)

func TestValidate_PluginProject_CodexGo(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mustWriteValidateFile(t, dir, "go.mod", "module example.com/x\n\ngo 1.22\n")
	mustWriteValidateFile(t, dir, "README.md", "# x\n")
	mustWriteValidateFile(t, dir, filepath.Join("cmd", "x", "main.go"), "package main\nfunc main() {}\n")
	mustSaveValidatedPackage(t, dir, pluginmanifest.Default("x", "codex-runtime", "go", "plugin", false), "go")
	generated, err := pluginmanifest.Generate(dir, "all")
	if err != nil {
		t.Fatal(err)
	}
	if err := pluginmanifest.WriteArtifacts(dir, generated.Artifacts); err != nil {
		t.Fatal(err)
	}

	report, err := Validate(dir, "codex-runtime")
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Failures) != 0 {
		t.Fatalf("failures = %+v", report.Failures)
	}
}

func TestValidate_PluginProjectDetectsDrift(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mustWriteValidateFile(t, dir, "go.mod", "module example.com/x\n\ngo 1.22\n")
	mustWriteValidateFile(t, dir, "README.md", "# x\n")
	mustWriteValidateFile(t, dir, filepath.Join("cmd", "x", "main.go"), "package main\nfunc main() {}\n")
	mustSaveValidatedPackage(t, dir, pluginmanifest.Default("x", "codex-runtime", "go", "plugin", false), "go")
	generated, err := pluginmanifest.Generate(dir, "all")
	if err != nil {
		t.Fatal(err)
	}
	if err := pluginmanifest.WriteArtifacts(dir, generated.Artifacts); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".codex", "config.toml"), []byte("broken\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	report, err := Validate(dir, "codex-runtime")
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, failure := range report.Failures {
		if failure.Kind == FailureGeneratedContractInvalid {
			found = true
		}
	}
	if !found {
		t.Fatalf("failures = %+v", report.Failures)
	}
}

func TestValidate_PluginProjectWarnsOnUnknownFields(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mustWriteValidateFile(t, dir, "go.mod", "module example.com/x\n\ngo 1.22\n")
	mustWriteValidateFile(t, dir, "README.md", "# x\n")
	mustWriteValidateFile(t, dir, filepath.Join("cmd", "x", "main.go"), "package main\nfunc main() {}\n")
	mustWriteValidateFile(t, dir, filepath.Join("plugin", pluginmanifest.FileName), `api_version: v1
name: "x"
version: "0.1.0"
description: "plugin"
targets: ["codex-runtime"]
extra: true
`)
	mustWriteValidateFile(t, dir, filepath.Join("plugin", pluginmanifest.LauncherFileName), "runtime: go\nentrypoint: ./bin/x\n")
	generated, err := pluginmanifest.Generate(dir, "all")
	if err != nil {
		t.Fatal(err)
	}
	if err := pluginmanifest.WriteArtifacts(dir, generated.Artifacts); err != nil {
		t.Fatal(err)
	}

	report, err := Validate(dir, "codex-runtime")
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Failures) != 0 {
		t.Fatalf("failures = %+v", report.Failures)
	}
	if len(report.Warnings) != 1 {
		t.Fatalf("warnings = %+v", report.Warnings)
	}
	if err := Run(dir, "codex-runtime"); err != nil {
		t.Fatalf("warnings-only validate should succeed, got %v", err)
	}
}

func TestValidate_PluginProject_ClaudeHooksMatchEntrypoint(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mustWriteValidateFile(t, dir, "go.mod", "module example.com/x\n\ngo 1.22\n")
	mustWriteValidateFile(t, dir, filepath.Join("cmd", "x", "main.go"), "package main\nfunc main() {}\n")
	mustSaveValidatedPackage(t, dir, pluginmanifest.Default("x", "claude", "go", "plugin", false), "go")
	mustWriteValidateFile(t, dir, filepath.Join("plugin", "targets", "claude", "hooks", "hooks.json"), `{
  "hooks": {
    "Stop": [{"hooks": [{"type": "command", "command": "./bin/x Stop"}]}],
    "PreToolUse": [{"hooks": [{"type": "command", "command": "./bin/x PreToolUse"}]}],
    "UserPromptSubmit": [{"hooks": [{"type": "command", "command": "./bin/x UserPromptSubmit"}]}]
  }
}
`)
	generated, err := pluginmanifest.Generate(dir, "claude")
	if err != nil {
		t.Fatal(err)
	}
	if err := pluginmanifest.WriteArtifacts(dir, generated.Artifacts); err != nil {
		t.Fatal(err)
	}

	report, err := Validate(dir, "claude")
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Failures) != 0 {
		t.Fatalf("failures = %+v", report.Failures)
	}
}

func TestValidate_PluginProject_ClaudeHookEntrypointMismatch(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mustWriteValidateFile(t, dir, "go.mod", "module example.com/x\n\ngo 1.22\n")
	mustWriteValidateFile(t, dir, filepath.Join("cmd", "x", "main.go"), "package main\nfunc main() {}\n")
	mustSaveValidatedPackage(t, dir, pluginmanifest.Default("x", "claude", "go", "plugin", false), "go")
	mustWriteValidateFile(t, dir, filepath.Join("plugin", "targets", "claude", "hooks", "hooks.json"), `{
  "hooks": {
    "Stop": [{"hooks": [{"type": "command", "command": "./bin/old-x Stop"}]}],
    "PreToolUse": [{"hooks": [{"type": "command", "command": "./bin/x PreToolUse"}]}],
    "UserPromptSubmit": [{"hooks": [{"type": "command", "command": "./bin/x UserPromptSubmit"}]}]
  }
}
`)
	generated, err := pluginmanifest.Generate(dir, "claude")
	if err != nil {
		t.Fatal(err)
	}
	if err := pluginmanifest.WriteArtifacts(dir, generated.Artifacts); err != nil {
		t.Fatal(err)
	}

	report, err := Validate(dir, "claude")
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, failure := range report.Failures {
		if failure.Kind == FailureEntrypointMismatch && strings.Contains(failure.Message, `"./bin/old-x Stop"`) && strings.Contains(failure.Message, `"./bin/x Stop"`) {
			found = true
		}
	}
	if !found {
		t.Fatalf("failures = %+v", report.Failures)
	}
}

func TestValidate_PluginProject_ClaudeExtendedHooksAlsoMatchEntrypoint(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mustWriteValidateFile(t, dir, "go.mod", "module example.com/x\n\ngo 1.22\n")
	mustWriteValidateFile(t, dir, filepath.Join("cmd", "x", "main.go"), "package main\nfunc main() {}\n")
	mustSaveValidatedPackage(t, dir, pluginmanifest.Default("x", "claude", "go", "plugin", false), "go")
	mustWriteValidateFile(t, dir, filepath.Join("plugin", "targets", "claude", "hooks", "hooks.json"), `{
  "hooks": {
    "Stop": [{"hooks": [{"type": "command", "command": "./bin/x Stop"}]}],
    "SessionStart": [{"hooks": [{"type": "command", "command": "./bin/old-x SessionStart"}]}]
  }
}
`)
	generated, err := pluginmanifest.Generate(dir, "claude")
	if err != nil {
		t.Fatal(err)
	}
	if err := pluginmanifest.WriteArtifacts(dir, generated.Artifacts); err != nil {
		t.Fatal(err)
	}

	report, err := Validate(dir, "claude")
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, failure := range report.Failures {
		if failure.Kind == FailureEntrypointMismatch && strings.Contains(failure.Message, "SessionStart") {
			found = true
		}
	}
	if !found {
		t.Fatalf("failures = %+v", report.Failures)
	}
}

func TestValidate_PluginProject_ClaudePackageOnlyMCP(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mustSaveValidatedPackage(t, dir, pluginmanifest.Default("context7", "claude", "go", "plugin", false), "")
	mustWriteValidateFile(t, dir, filepath.Join("plugin", "mcp", "servers.yaml"), `api_version: v1

servers:
  context7:
    type: stdio
    stdio:
      command: npx
      args:
        - -y
        - "@upstash/context7-mcp@2.1.6"
    targets:
      - claude
`)
	generated, err := pluginmanifest.Generate(dir, "claude")
	if err != nil {
		t.Fatal(err)
	}
	if err := pluginmanifest.WriteArtifacts(dir, generated.Artifacts); err != nil {
		t.Fatal(err)
	}

	report, err := Validate(dir, "claude")
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Failures) != 0 {
		t.Fatalf("failures = %+v", report.Failures)
	}
	if _, err := os.Stat(filepath.Join(dir, "hooks", "hooks.json")); !os.IsNotExist(err) {
		t.Fatalf("hooks/hooks.json err = %v, want not exists", err)
	}
}

func mustSaveValidatedPackage(t *testing.T, root string, manifest pluginmanifest.Manifest, runtime string) {
	t.Helper()
	if err := pluginmanifest.Save(root, manifest, false); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(runtime) != "" {
		if err := pluginmanifest.SaveLauncher(root, pluginmanifest.DefaultLauncher(manifest.Name, runtime), false); err != nil {
			t.Fatal(err)
		}
		mustWriteValidateFile(t, root, filepath.Join("bin", manifest.Name), "#!/bin/sh\nexit 0\n")
		if err := os.Chmod(filepath.Join(root, "bin", manifest.Name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
}
