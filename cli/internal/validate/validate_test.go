package validate

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmanifest"
	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
	"github.com/777genius/plugin-kit-ai/cli/internal/runtimecheck"
)

func TestValidate_ManifestMissing(t *testing.T) {
	t.Parallel()
	_, err := Validate(t.TempDir(), "")
	var re *ReportError
	if !errors.As(err, &re) {
		t.Fatalf("expected ReportError, got %v", err)
	}
	if got := re.Report.Failures[0].Kind; got != FailureManifestMissing {
		t.Fatalf("failure kind = %q", got)
	}
	if got := re.Report.Failures[0].Path; got != filepath.Join(pluginmodel.SourceDirName, pluginmanifest.FileName) {
		t.Fatalf("failure path = %q", got)
	}
	if re.Error() != "required manifest missing: plugin/plugin.yaml" {
		t.Fatalf("error = %q", re.Error())
	}
}

func TestValidate_MissingRequiredLauncherSetsFailurePath(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, "plugin.yaml"), `api_version: v1
name: "x"
version: "0.1.0"
description: "x"
targets: ["codex-runtime"]
`)

	report, err := Validate(dir, "codex-runtime")
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Failures) == 0 {
		t.Fatal("expected at least one failure")
	}
	first := report.Failures[0]
	if first.Kind != FailureManifestInvalid {
		t.Fatalf("failure kind = %q", first.Kind)
	}
	if first.Path != filepath.Join(pluginmodel.SourceDirName, pluginmanifest.LauncherFileName) {
		t.Fatalf("failure path = %q", first.Path)
	}
	if !strings.Contains(first.Message, "required launcher missing") {
		t.Fatalf("failure message = %q", first.Message)
	}
}

func TestValidate_LegacyProjectManifestRejected(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mustWriteValidateFile(t, dir, filepath.Join(".plugin-kit-ai", "project.toml"), "schema_version = 1\nplatform = \"codex\"\nruntime = \"shell\"\nexecution_mode = \"launcher\"\nentrypoint = \"./bin/x\"\n")

	_, err := Validate(dir, "")
	var re *ReportError
	if !errors.As(err, &re) {
		t.Fatalf("expected ReportError, got %v", err)
	}
	if got := re.Report.Failures[0].Kind; got != FailureManifestInvalid {
		t.Fatalf("failure kind = %q", got)
	}
	if got := re.Report.Failures[0].Path; got != filepath.Join(".plugin-kit-ai", "project.toml") {
		t.Fatalf("failure path = %q", got)
	}
	if !strings.Contains(re.Error(), ".plugin-kit-ai/project.toml is not supported") {
		t.Fatalf("error = %q", re.Error())
	}
}

func TestValidate_TargetNotEnabledSetsPluginPath(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, "plugin.yaml"), `api_version: v1
name: "x"
version: "0.1.0"
description: "x"
targets: ["codex-package"]
`)

	report, err := Validate(dir, "codex-runtime")
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, failure := range report.Failures {
		if failure.Kind == FailureManifestInvalid &&
			failure.Path == filepath.Join(pluginmodel.SourceDirName, pluginmanifest.FileName) &&
			strings.Contains(failure.Message, `plugin.yaml does not enable target "codex-runtime"`) {
			found = true
		}
	}
	if !found {
		t.Fatalf("failures = %+v", report.Failures)
	}
}

func TestValidate_InvalidLauncherSetsLauncherPath(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, "plugin.yaml"), `api_version: v1
name: "x"
version: "0.1.0"
description: "x"
targets: ["codex-runtime"]
`)
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, pluginmanifest.LauncherFileName), "runtime: go\n")

	report, err := Validate(dir, "codex-runtime")
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, failure := range report.Failures {
		if failure.Kind == FailureManifestInvalid &&
			failure.Path == filepath.Join(pluginmodel.SourceDirName, pluginmanifest.LauncherFileName) &&
			strings.Contains(failure.Message, "entrypoint required") {
			found = true
		}
	}
	if !found {
		t.Fatalf("failures = %+v", report.Failures)
	}
}

func TestValidatePluginRuntimeFilesOptionalLauncherUsesEntrypointOnly(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mustWriteValidateFile(t, dir, filepath.Join("plugin", "plugin.yaml"), `api_version: v1
name: "x"
version: "0.1.0"
description: "x"
targets: ["claude", "codex-package", "gemini"]
`)
	mustWriteValidateFile(t, dir, filepath.Join("bin", "hook"), "#!/bin/sh\nexit 0\n")
	if err := os.Chmod(filepath.Join(dir, "bin", "hook"), 0o755); err != nil {
		t.Fatal(err)
	}

	var report Report
	validatePluginRuntimeFiles(
		dir,
		pluginmanifest.Manifest{Targets: []string{"claude", "codex-package", "gemini"}},
		&pluginmanifest.Launcher{Runtime: "python", Entrypoint: "./bin/hook"},
		&report,
	)

	if len(report.Failures) != 0 {
		t.Fatalf("failures = %+v", report.Failures)
	}
}

func TestHasRuntimeProjectFiles(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		runtimeName string
		path        string
	}{
		{name: "go module", runtimeName: "go", path: "go.mod"},
		{name: "go sources", runtimeName: "go", path: filepath.Join("cmd", "demo", "main.go")},
		{name: "python project", runtimeName: "python", path: "pyproject.toml"},
		{name: "python authored entrypoint", runtimeName: "python", path: filepath.Join(pluginmodel.SourceDirName, "main.py")},
		{name: "node package", runtimeName: "node", path: "package.json"},
		{name: "node authored module", runtimeName: "node", path: filepath.Join(pluginmodel.SourceDirName, "main.mjs")},
		{name: "node authored typescript", runtimeName: "node", path: filepath.Join(pluginmodel.SourceDirName, "main.ts")},
		{name: "shell target", runtimeName: "shell", path: filepath.Join("scripts", "main.sh")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			mustWriteValidateFile(t, dir, tt.path, "fixture")
			if !hasRuntimeProjectFiles(dir, tt.runtimeName) {
				t.Fatalf("hasRuntimeProjectFiles(%q) = false", tt.runtimeName)
			}
		})
	}

	if hasRuntimeProjectFiles(t.TempDir(), "python") {
		t.Fatal("empty optional launcher project detected as a runtime project")
	}
}

func TestValidate_LegacyPortableMCPPathSetsFailurePath(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, "plugin.yaml"), `api_version: v1
name: "x"
version: "0.1.0"
description: "x"
targets: ["codex-package"]
`)
	mustWriteValidateFile(t, dir, filepath.Join("mcp", "servers.json"), "{}\n")

	report, err := Validate(dir, "codex-package")
	if err != nil {
		var re *ReportError
		if !errors.As(err, &re) {
			t.Fatal(err)
		}
		report = re.Report
	}
	var found bool
	for _, failure := range report.Failures {
		if failure.Kind == FailureManifestInvalid &&
			failure.Path == filepath.Join(pluginmodel.SourceDirName, "mcp", "servers.json") &&
			strings.Contains(failure.Message, "unsupported portable MCP authored path") {
			found = true
		}
	}
	if !found {
		t.Fatalf("failures = %+v", report.Failures)
	}
}

func TestValidate_UnsupportedPortableMCPUsesAuthoredPath(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, "plugin.yaml"), `api_version: v1
name: "x"
version: "0.1.0"
description: "x"
targets: ["codex-runtime"]
`)
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, pluginmanifest.LauncherFileName), "runtime: go\nentrypoint: ./bin/x\n")
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, "mcp", "servers.yaml"), `api_version: v1

servers:
  docs:
    type: remote
    remote:
      protocol: streamable_http
      url: "https://example.com/mcp"
`)

	report, err := Validate(dir, "codex-runtime")
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, failure := range report.Failures {
		if failure.Kind == FailureUnsupportedTargetKind &&
			failure.Path == filepath.ToSlash(filepath.Join(pluginmodel.SourceDirName, "mcp", "servers.yaml")) &&
			strings.Contains(failure.Message, "does not support portable component kind mcp_servers") {
			found = true
		}
	}
	if !found {
		t.Fatalf("failures = %+v", report.Failures)
	}
}

func TestUnsupportedTargetKindPathUsesSurfaceLocation(t *testing.T) {
	t.Parallel()
	t.Run("doc path", func(t *testing.T) {
		t.Parallel()
		tc := pluginmanifest.TargetComponents{
			Docs:       map[string]string{"interface": filepath.ToSlash(filepath.Join(pluginmodel.SourceDirName, "targets", "codex-package", "interface.json"))},
			Components: map[string][]string{},
		}
		want := filepath.ToSlash(filepath.Join(pluginmodel.SourceDirName, "targets", "codex-package", "interface.json"))
		if got := unsupportedTargetKindPath("codex-package", tc, "interface"); got != want {
			t.Fatalf("unsupportedTargetKindPath() = %q, want %q", got, want)
		}
	})

	t.Run("component directory", func(t *testing.T) {
		t.Parallel()
		tc := pluginmanifest.TargetComponents{
			Docs: map[string]string{},
			Components: map[string][]string{
				"commands": {filepath.ToSlash(filepath.Join(pluginmodel.SourceDirName, "targets", "codex-runtime", "commands", "review.md"))},
			},
		}
		want := filepath.ToSlash(filepath.Join(pluginmodel.SourceDirName, "targets", "codex-runtime", "commands"))
		if got := unsupportedTargetKindPath("codex-runtime", tc, "commands"); got != want {
			t.Fatalf("unsupportedTargetKindPath() = %q, want %q", got, want)
		}
	})
}

func TestExtractFailurePath_RuntimeNotFoundCases(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		message string
		want    string
	}{
		{
			name:    "missing node in path",
			message: "runtime not found: node runtime required; checked PATH for node. Install Node.js 20+",
			want:    "node",
		},
		{
			name:    "node binary found but unsupported",
			message: "runtime not found: found node at /usr/local/bin/node but version 18.17.0 is below the supported minimum 20.0; install or repair Node.js 20+",
			want:    "/usr/local/bin/node",
		},
		{
			name:    "python interpreter found via pyenv",
			message: "runtime not found: found pyenv interpreter at /Users/demo/.pyenv/shims/python but version 3.9.0 is below the supported minimum 3.10. Run plugin-kit-ai doctor ., then plugin-kit-ai bootstrap .",
			want:    "/Users/demo/.pyenv/shims/python",
		},
		{
			name:    "windows bash required",
			message: "runtime not found: bash (shell runtime on Windows requires bash in PATH; install Git Bash or another bash-compatible shell)",
			want:    "bash",
		},
		{
			name:    "nested parse error",
			message: "runtime not found: python runtime inspection failed: parse targets/codex-runtime/package.yaml: yaml: line 1: did not find expected key",
			want:    filepath.ToSlash(filepath.Join(pluginmodel.SourceDirName, "targets", "codex-runtime", "package.yaml")),
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := extractFailurePath(tc.message); got != tc.want {
				t.Fatalf("extractFailurePath(%q) = %q, want %q", tc.message, got, tc.want)
			}
		})
	}
}

func TestValidate_GeminiRejectsInvalidExtensionName(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, "plugin.yaml"), `api_version: v1
name: "Demo_Extension"
version: "0.1.0"
description: "demo"
targets: ["gemini"]
`)
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, "targets", "gemini", "package.yaml"), "context_file_name: GEMINI.md\n")
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, "targets", "gemini", "contexts", "GEMINI.md"), "# Gemini\n")
	mustWriteValidateFile(t, dir, "gemini-extension.json", "{}\n")

	_, err := Validate(dir, "gemini")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "invalid Gemini extension name") {
		t.Fatalf("error = %q", err)
	}
}

func TestValidate_GeminiRejectsTrustAndAmbiguousContexts(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, "plugin.yaml"), `api_version: v1
name: "gemini-demo"
version: "0.1.0"
description: "demo"
targets: ["gemini"]
`)
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, "mcp", "servers.yaml"), `api_version: v1

servers:
  demo:
    type: stdio
    stdio:
      command: "node server.mjs"
    overrides:
      gemini:
        trust: true
`)
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, "targets", "gemini", "contexts", "FIRST.md"), "# First\n")
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, "targets", "gemini", "contexts", "SECOND.md"), "# Second\n")

	report, err := Validate(dir, "gemini")
	if err != nil {
		t.Fatal(err)
	}
	var foundTrust, foundContext bool
	for _, failure := range report.Failures {
		if strings.Contains(failure.Message, "may not set trust") {
			foundTrust = true
		}
		if strings.Contains(failure.Message, "primary context selection is ambiguous") {
			foundContext = true
		}
	}
	if !foundTrust || !foundContext {
		t.Fatalf("failures = %+v", report.Failures)
	}
	var warnedCommand bool
	for _, warning := range report.Warnings {
		if warning.Kind == WarningGeminiMCPCommandStyle {
			warnedCommand = true
		}
	}
	if !warnedCommand {
		t.Fatalf("warnings = %+v", report.Warnings)
	}
}

func TestValidate_GeminiWarnsOnIgnoredPolicyKeys(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, "plugin.yaml"), `api_version: v1
name: "gemini-policy"
version: "0.1.0"
description: "demo"
targets: ["gemini"]
`)
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, "targets", "gemini", "contexts", "GEMINI.md"), "# Gemini\n")
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, "targets", "gemini", "policies", "review.toml"), "allow = true\nyolo = true\n")

	report, err := Validate(dir, "gemini")
	if err != nil {
		t.Fatal(err)
	}
	var allowWarn, yoloWarn bool
	for _, warning := range report.Warnings {
		if strings.Contains(warning.Message, `"allow"`) {
			allowWarn = true
		}
		if strings.Contains(warning.Message, `"yolo"`) {
			yoloWarn = true
		}
	}
	if !allowWarn || !yoloWarn {
		t.Fatalf("warnings = %+v", report.Warnings)
	}
}

func TestValidate_GeminiRejectsInvalidCommandTOML(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, "plugin.yaml"), `api_version: v1
name: "gemini-command"
version: "0.1.0"
description: "demo"
targets: ["gemini"]
`)
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, "targets", "gemini", "contexts", "GEMINI.md"), "# Gemini\n")
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, "targets", "gemini", "commands", "bad.toml"), "description = [\n")

	report, err := Validate(dir, "gemini")
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, failure := range report.Failures {
		if strings.Contains(failure.Message, "invalid TOML") {
			found = true
		}
	}
	if !found {
		t.Fatalf("failures = %+v", report.Failures)
	}
}

func TestValidate_GeminiRejectsUnsupportedHooksLayout(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, "plugin.yaml"), `api_version: v1
name: "gemini-hooks"
version: "0.1.0"
description: "demo"
targets: ["gemini"]
`)
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, "targets", "gemini", "contexts", "GEMINI.md"), "# Gemini\n")
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, "targets", "gemini", "hooks", "extra.json"), "{}\n")

	report, err := Validate(dir, "gemini")
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, failure := range report.Failures {
		if strings.Contains(failure.Message, "unsupported Gemini hooks layout") {
			found = true
		}
	}
	if !found {
		t.Fatalf("failures = %+v", report.Failures)
	}
}

func TestValidate_GeminiRejectsHooksWithoutTopLevelHooksObject(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, "plugin.yaml"), `api_version: v1
name: "gemini-hooks"
version: "0.1.0"
description: "demo"
targets: ["gemini"]
`)
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, "targets", "gemini", "contexts", "GEMINI.md"), "# Gemini\n")
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, "targets", "gemini", "hooks", "hooks.json"), "{}\n")

	report, err := Validate(dir, "gemini")
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, failure := range report.Failures {
		if strings.Contains(failure.Message, "must define a top-level hooks object") {
			found = true
		}
	}
	if !found {
		t.Fatalf("failures = %+v", report.Failures)
	}
}

func TestValidate_OpenCodeRejectsInvalidPortableSkillForMirroring(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, "plugin.yaml"), `api_version: v1
name: "opencode-demo"
version: "0.1.0"
description: "demo"
targets: ["opencode"]
`)
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, "targets", "opencode", "package.yaml"), "plugins:\n  - \"@acme/demo-opencode\"\n")
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, "skills", "demo", "SKILL.md"), "---\nname: wrong\ndescription: demo\n---\n")
	mustWriteValidateFile(t, dir, "opencode.json", `{
  "$schema": "https://opencode.ai/config.json",
  "plugin": [
    "@acme/demo-opencode"
  ]
}`)

	report, err := Validate(dir, "opencode")
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, failure := range report.Failures {
		if strings.Contains(failure.Message, "OpenCode mirrored skill is incompatible") &&
			strings.Contains(failure.Message, "must match skill directory") {
			found = true
		}
	}
	if !found {
		t.Fatalf("failures = %+v", report.Failures)
	}
}

func TestValidate_OpenCodeWarnsWhenJSONCTakesLowerPrecedence(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, "plugin.yaml"), `api_version: v1
name: "opencode-demo"
version: "0.1.0"
description: "demo"
targets: ["opencode"]
`)
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, "targets", "opencode", "package.yaml"), "plugins:\n  - \"@acme/demo-opencode\"\n")
	mustWriteValidateFile(t, dir, "opencode.json", `{"$schema":"https://opencode.ai/config.json","plugin":["@acme/demo-opencode"]}`)
	mustWriteValidateFile(t, dir, "opencode.jsonc", `{"$schema":"https://opencode.ai/config.json","plugin":["@acme/ignored"],}`)

	report, err := Validate(dir, "opencode")
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, warning := range report.Warnings {
		if strings.Contains(warning.Message, "opencode.json takes precedence") {
			found = true
		}
	}
	if !found {
		t.Fatalf("warnings = %+v", report.Warnings)
	}
}

func TestValidate_OpenCodeAllowsConfigExtra(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, "plugin.yaml"), `api_version: v1
name: "opencode-demo"
version: "0.1.0"
description: "demo"
targets: ["opencode"]
`)
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, "targets", "opencode", "package.yaml"), "plugins:\n  - \"@acme/demo-opencode\"\n")
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, "targets", "opencode", "config.extra.json"), `{"theme":"midnight"}`)
	mustWriteValidateFile(t, dir, "opencode.json", `{"$schema":"https://opencode.ai/config.json","plugin":["@acme/demo-opencode"]}`)

	report, err := Validate(dir, "opencode")
	if err != nil {
		t.Fatal(err)
	}
	for _, failure := range report.Failures {
		if failure.Path == filepath.ToSlash(filepath.Join(pluginmodel.SourceDirName, "targets", "opencode", "config.extra.json")) {
			t.Fatalf("unexpected config.extra.json failure: %+v", failure)
		}
	}
}

func TestValidate_GeminiRejectsNonYAMLSettingsAndThemes(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, "plugin.yaml"), `api_version: v1
name: "gemini-assets"
version: "0.1.0"
description: "demo"
targets: ["gemini"]
`)
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, "targets", "gemini", "contexts", "GEMINI.md"), "# Gemini\n")
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, "targets", "gemini", "settings", "api-token.json"), "{}\n")

	report, err := Validate(dir, "gemini")
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, failure := range report.Failures {
		if strings.Contains(failure.Message, "unsupported Gemini setting file") {
			found = true
		}
	}
	if !found {
		t.Fatalf("failures = %+v", report.Failures)
	}
}

func TestValidatePythonRuntime_UsesRepoLocalInterpreterFirst(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	restoreLookPath := runtimecheck.LookPath
	restoreRunCommand := runtimecheck.RunCommand
	runtimecheck.LookPath = func(name string) (string, error) { return name, nil }
	runtimecheck.RunCommand = func(dir, name string, args ...string) (string, error) {
		if len(args) == 1 && args[0] == "--version" {
			return "Python 3.11.0", nil
		}
		return "", exec.ErrNotFound
	}
	t.Cleanup(func() {
		runtimecheck.LookPath = restoreLookPath
		runtimecheck.RunCommand = restoreRunCommand
	})

	mustWriteValidateFile(t, root, filepath.Join("bin", "demo"), "#!/usr/bin/env bash\nexit 0\n")
	if runtime.GOOS == "windows" {
		venv := filepath.Join(root, ".venv", "Scripts", "python.exe")
		mustWriteValidateFile(t, root, filepath.Join(".venv", "Scripts", "python.exe"), "binary")
		project, err := runtimecheck.Inspect(runtimecheck.Inputs{
			Root:    root,
			Targets: []string{"codex-runtime"},
			Launcher: &pluginmanifest.Launcher{
				Runtime:    "python",
				Entrypoint: "./bin/demo",
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		if project.Python.ReadyInterpreter != venv {
			t.Fatalf("ready interpreter = %q, want %q", project.Python.ReadyInterpreter, venv)
		}
		return
	}
	venv := filepath.Join(root, ".venv", "bin", "python3")
	mustWriteValidateFile(t, root, filepath.Join(".venv", "bin", "python3"), "binary")
	project, err := runtimecheck.Inspect(runtimecheck.Inputs{
		Root:    root,
		Targets: []string{"codex-runtime"},
		Launcher: &pluginmanifest.Launcher{
			Runtime:    "python",
			Entrypoint: "./bin/demo",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if project.Python.ReadyInterpreter != venv {
		t.Fatalf("ready interpreter = %q, want %q", project.Python.ReadyInterpreter, venv)
	}
}

func TestValidatePythonRuntime_BrokenProjectVenvShowsRecoveryGuidance(t *testing.T) {
	root := t.TempDir()
	mustWriteValidateFile(t, root, filepath.Join("bin", "demo"), "#!/usr/bin/env bash\nexit 0\n")
	restoreLookPath := runtimecheck.LookPath
	restoreRunCommand := runtimecheck.RunCommand
	runtimecheck.LookPath = func(name string) (string, error) { return name, nil }
	runtimecheck.RunCommand = func(dir, name string, args ...string) (string, error) {
		if len(args) == 1 && args[0] == "--version" {
			return "", exec.ErrNotFound
		}
		return "", exec.ErrNotFound
	}
	t.Cleanup(func() {
		runtimecheck.LookPath = restoreLookPath
		runtimecheck.RunCommand = restoreRunCommand
	})
	if runtime.GOOS == "windows" {
		mustWriteValidateFile(t, root, filepath.Join(".venv", "Scripts", "python.exe"), "not-a-real-exe")
	} else {
		mustWriteValidateFile(t, root, filepath.Join(".venv", "bin", "python3"), "not-a-real-exe")
	}

	err := validatePythonRuntime(root, []string{"codex-runtime"}, &pluginmanifest.Launcher{
		Runtime:    "python",
		Entrypoint: "./bin/demo",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "recreate .venv") {
		t.Fatalf("error = %q", err)
	}
}

func TestRequireMinVersion(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		runtime   string
		output    string
		wantMajor int
		wantMinor int
		wantErr   string
	}{
		{
			name:      "python-supported",
			runtime:   "python",
			output:    "Python 3.11.8\n",
			wantMajor: 3,
			wantMinor: 10,
		},
		{
			name:      "python-too-old",
			runtime:   "python",
			output:    "Python 3.9.18\n",
			wantMajor: 3,
			wantMinor: 10,
			wantErr:   "below the supported minimum 3.10",
		},
		{
			name:      "node-supported",
			runtime:   "node",
			output:    "v20.11.1\n",
			wantMajor: 20,
			wantMinor: 0,
		},
		{
			name:      "node-too-old",
			runtime:   "node",
			output:    "v18.19.0\n",
			wantMajor: 20,
			wantMinor: 0,
			wantErr:   "below the supported minimum 20.0",
		},
		{
			name:      "unparseable",
			runtime:   "node",
			output:    "hello",
			wantMajor: 20,
			wantMinor: 0,
			wantErr:   "unsupported version output",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			err := requireMinVersion(tc.runtime, tc.output, tc.wantMajor, tc.wantMinor)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("requireMinVersion() error = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("requireMinVersion() error = %v, want substring %q", err, tc.wantErr)
			}
		})
	}
}

func TestValidate_ManifestProject_ShellRequiresBashOnWindows(t *testing.T) {
	t.Parallel()
	if runtime.GOOS != "windows" {
		t.Skip("windows-specific")
	}
	dir := t.TempDir()
	mustWriteValidateFile(t, dir, "README.md", "# x\n")
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, "plugin.yaml"), `api_version: v1
name: "x"
version: "0.1.0"
description: "x"
targets: ["codex-runtime"]
`)
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, pluginmanifest.LauncherFileName), "runtime: shell\nentrypoint: ./bin/x\n")
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, "targets", "codex-runtime", "package.yaml"), "model_hint: gpt-5.4-mini\n")
	mustWriteValidateFile(t, dir, filepath.Join(".codex", "config.toml"), "notify = [\"./bin/x\", \"notify\"]\n")
	mustWriteValidateFile(t, dir, filepath.Join("bin", "x.cmd"), "@echo off\r\n")
	mustWriteValidateFile(t, dir, filepath.Join("scripts", "main.sh"), "#!/usr/bin/env bash\nexit 0\n")

	report, err := Validate(dir, "codex-runtime")
	if err != nil {
		t.Fatal(err)
	}
	if _, bashErr := exec.LookPath("bash"); bashErr != nil {
		var found bool
		for _, failure := range report.Failures {
			if failure.Kind == FailureRuntimeNotFound &&
				failure.Path == "bash" &&
				strings.Contains(failure.Message, "bash") {
				found = true
			}
		}
		if !found {
			t.Fatalf("failures = %+v", report.Failures)
		}
	}
}

func TestValidate_CodexRejectsManifestExtraCanonicalOverride(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mustWriteValidateFile(t, dir, "README.md", "# x\n")
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, "plugin.yaml"), `api_version: v1
name: "x"
version: "0.1.0"
description: "x"
targets: ["codex-package"]
`)
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, "targets", "codex-package", "manifest.extra.json"), `{"name":"override"}`)
	mustWriteValidateFile(t, dir, filepath.Join(".codex-plugin", "plugin.json"), "{}\n")

	report, err := Validate(dir, "codex-package")
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, failure := range report.Failures {
		if strings.Contains(failure.Message, `codex-package manifest.extra.json may not override canonical field "name"`) {
			found = true
		}
	}
	if !found {
		t.Fatalf("failures = %+v", report.Failures)
	}
}

func TestValidate_CodexRejectsManifestExtraPackageAndInterfaceOverrides(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mustWriteValidateFile(t, dir, "README.md", "# x\n")
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, "plugin.yaml"), `api_version: v1
name: "x"
version: "0.1.0"
description: "x"
targets: ["codex-package"]
`)
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, "targets", "codex-package", "manifest.extra.json"), `{"homepage":"https://override.example.com"}`)
	mustWriteValidateFile(t, dir, filepath.Join(".codex-plugin", "plugin.json"), "{}\n")

	report, err := Validate(dir, "codex-package")
	if err != nil {
		t.Fatal(err)
	}
	var foundHomepage bool
	for _, failure := range report.Failures {
		if strings.Contains(failure.Message, `codex-package manifest.extra.json may not override canonical field "homepage"`) {
			foundHomepage = true
		}
	}
	if !foundHomepage {
		t.Fatalf("failures = %+v", report.Failures)
	}

	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, "targets", "codex-package", "manifest.extra.json"), `{"interface":{"defaultPrompt":["override"]}}`)
	report, err = Validate(dir, "codex-package")
	if err != nil {
		t.Fatal(err)
	}
	var foundInterface bool
	for _, failure := range report.Failures {
		if strings.Contains(failure.Message, `codex-package manifest.extra.json may not override canonical field "interface"`) {
			foundInterface = true
		}
	}
	if !foundInterface {
		t.Fatalf("failures = %+v", report.Failures)
	}
}

func TestValidate_CodexRejectsMalformedStructuredDocs(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mustWriteValidateFile(t, dir, "README.md", "# x\n")
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, "plugin.yaml"), `api_version: v1
name: "x"
version: "0.1.0"
description: "x"
targets: ["codex-package"]
`)
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, "targets", "codex-package", "interface.json"), `{"defaultPrompt":"override"}`)

	report, err := Validate(dir, "codex-package")
	if err != nil {
		t.Fatal(err)
	}
	var foundInterface bool
	for _, failure := range report.Failures {
		if failure.Path == filepath.Join(pluginmodel.SourceDirName, "targets", "codex-package", "interface.json") &&
			strings.Contains(failure.Message, "interface.defaultPrompt must be an array of strings") {
			foundInterface = true
		}
	}
	if !foundInterface {
		t.Fatalf("failures = %+v", report.Failures)
	}

	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, "targets", "codex-package", "interface.json"), `{"defaultPrompt":["override"]}`)
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, "targets", "codex-package", "app.json"), `["demo-app"]`)
	report, err = Validate(dir, "codex-package")
	if err != nil {
		t.Fatal(err)
	}
	var foundApp bool
	for _, failure := range report.Failures {
		if failure.Path == filepath.Join(pluginmodel.SourceDirName, "targets", "codex-package", "app.json") &&
			strings.Contains(failure.Message, "Codex app manifest must be a JSON object") {
			foundApp = true
		}
	}
	if !foundApp {
		t.Fatalf("failures = %+v", report.Failures)
	}
}

func TestValidate_CodexRejectsMalformedGeneratedPluginManifest(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mustWriteValidateFile(t, dir, "README.md", "# x\n")
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, "plugin.yaml"), `api_version: v1
name: "x"
version: "0.1.0"
description: "x"
targets: ["codex-package"]
`)
	mustWriteValidateFile(t, dir, filepath.Join(".codex-plugin", "plugin.json"), `{"name":"x","version":"0.1.0","description":"x","author":"maintainer"}`)

	report, err := Validate(dir, "codex-package")
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, failure := range report.Failures {
		if failure.Path == filepath.ToSlash(filepath.Join(".codex-plugin", "plugin.json")) &&
			strings.Contains(failure.Message, "Codex plugin author must be a JSON object") {
			found = true
		}
	}
	if !found {
		t.Fatalf("failures = %+v", report.Failures)
	}
}

func TestValidate_CodexRejectsUnexpectedPluginDirEntries(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mustWriteValidateFile(t, dir, "README.md", "# x\n")
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, "plugin.yaml"), `api_version: v1
name: "x"
version: "0.1.0"
description: "x"
targets: ["codex-package"]
`)
	mustWriteValidateFile(t, dir, filepath.Join(".codex-plugin", "plugin.json"), `{"name":"x","version":"0.1.0","description":"x"}`)
	mustWriteValidateFile(t, dir, filepath.Join(".codex-plugin", "notes.txt"), "unexpected\n")

	report, err := Validate(dir, "codex-package")
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, failure := range report.Failures {
		if failure.Path == filepath.ToSlash(filepath.Join(".codex-plugin", "notes.txt")) &&
			strings.Contains(failure.Message, "may only contain plugin.json") {
			found = true
		}
	}
	if !found {
		t.Fatalf("failures = %+v", report.Failures)
	}
}

func TestValidate_CodexRejectsUnreferencedBundleSidecars(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mustWriteValidateFile(t, dir, "README.md", "# x\n")
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, "plugin.yaml"), `api_version: v1
name: "x"
version: "0.1.0"
description: "x"
targets: ["codex-package"]
`)
	mustWriteValidateFile(t, dir, filepath.Join(".codex-plugin", "plugin.json"), `{"name":"x","version":"0.1.0","description":"x"}`)
	mustWriteValidateFile(t, dir, ".app.json", `{"name":"demo-app"}`)
	mustWriteValidateFile(t, dir, ".mcp.json", `{"docs":{"url":"https://example.com/mcp"}}`)

	report, err := Validate(dir, "codex-package")
	if err != nil {
		t.Fatal(err)
	}
	var foundApp, foundMCP bool
	for _, failure := range report.Failures {
		switch {
		case failure.Path == ".app.json" && strings.Contains(failure.Message, "without a matching .codex-plugin/plugin.json ref"):
			foundApp = true
		case failure.Path == ".mcp.json" && strings.Contains(failure.Message, "without a matching .codex-plugin/plugin.json ref"):
			foundMCP = true
		}
	}
	if !foundApp || !foundMCP {
		t.Fatalf("failures = %+v", report.Failures)
	}
}

func TestValidate_CodexRejectsNonCanonicalGeneratedManifestRefs(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mustWriteValidateFile(t, dir, "README.md", "# x\n")
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, "plugin.yaml"), `api_version: v1
name: "x"
version: "0.1.0"
description: "x"
targets: ["codex-package"]
`)
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, "skills", "x", "SKILL.md"), "---\nname: x\ndescription: x\n---\nDo x.\n")
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, "mcp", "servers.yaml"), `api_version: v1

servers:
  docs:
    type: remote
    remote:
      protocol: streamable_http
      url: "https://example.com/mcp"
    targets:
      - "codex-package"
`)
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, "targets", "codex-package", "app.json"), `{"name":"demo-app"}`)
	mustWriteValidateFile(t, dir, filepath.Join(".codex-plugin", "plugin.json"), `{"name":"x","version":"0.1.0","description":"x","skills":"./custom-skills/","mcpServers":"./custom.mcp.json","apps":"./custom.app.json"}`)

	report, err := Validate(dir, "codex-package")
	if err != nil {
		t.Fatal(err)
	}
	var foundSkills, foundMCP, foundApps bool
	for _, failure := range report.Failures {
		switch {
		case strings.Contains(failure.Message, `must use "./skills/" for skills when present`):
			foundSkills = true
		case strings.Contains(failure.Message, `must use "./.mcp.json" for mcpServers when present`):
			foundMCP = true
		case strings.Contains(failure.Message, `must use "./.app.json" for apps when present`):
			foundApps = true
		}
	}
	if !foundSkills || !foundMCP || !foundApps {
		t.Fatalf("failures = %+v", report.Failures)
	}
}

func TestValidate_CodexRejectsRefsOutsidePluginRoot(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mustWriteValidateFile(t, dir, "README.md", "# x\n")
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, "plugin.yaml"), `api_version: v1
name: "x"
version: "0.1.0"
description: "x"
targets: ["codex-package"]
`)
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, "targets", "codex-package", "app.json"), `{"name":"demo-app"}`)
	mustWriteValidateFile(t, dir, filepath.Join(".codex-plugin", "plugin.json"), `{"name":"x","version":"0.1.0","description":"x","apps":"../outside.app.json"}`)

	report, err := Validate(dir, "codex-package")
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, failure := range report.Failures {
		if failure.Path == ".codex-plugin/plugin.json" && strings.Contains(failure.Message, `uses an invalid apps ref "../outside.app.json"`) {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("failures = %+v", report.Failures)
	}
}

func TestValidate_CodexRejectsGeneratedMetadataMismatch(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mustWriteValidateFile(t, dir, "README.md", "# x\n")
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, "plugin.yaml"), `api_version: v1
name: "x"
version: "0.1.0"
description: "x"
targets: ["codex-package"]
`)
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, "targets", "codex-package", "package.yaml"), `author:
  name: Example Maintainer
homepage: https://example.com/x
keywords:
  - codex
`)
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, "targets", "codex-package", "interface.json"), `{"defaultPrompt":["Help with x."]}`)
	mustWriteValidateFile(t, dir, filepath.Join(".codex-plugin", "plugin.json"), `{"name":"other","version":"9.9.9","description":"other","author":{"name":"Different Maintainer"},"homepage":"https://example.com/other","keywords":["wrong"],"interface":{"defaultPrompt":["Different"]}}`)

	report, err := Validate(dir, "codex-package")
	if err != nil {
		t.Fatal(err)
	}
	var foundName, foundMeta, foundInterface bool
	for _, failure := range report.Failures {
		switch {
		case strings.Contains(failure.Message, `sets name "other"; expected "x"`):
			foundName = true
		case strings.Contains(failure.Message, "package metadata does not match plugin.yaml plus optional targets/codex-package/package.yaml overrides"):
			foundMeta = true
		case strings.Contains(failure.Message, "interface does not match targets/codex-package/interface.json"):
			foundInterface = true
		}
	}
	if !foundName || !foundMeta || !foundInterface {
		t.Fatalf("failures = %+v", report.Failures)
	}
}

func TestValidate_CodexRejectsGeneratedSidecarMismatch(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mustWriteValidateFile(t, dir, "README.md", "# x\n")
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, "plugin.yaml"), `api_version: v1
name: "x"
version: "0.1.0"
description: "x"
targets: ["codex-package"]
`)
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, "mcp", "servers.yaml"), `api_version: v1

servers:
  docs:
    type: remote
    remote:
      protocol: streamable_http
      url: "https://example.com/mcp"
    targets:
      - "codex-package"
`)
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, "targets", "codex-package", "app.json"), `{"name":"demo-app","url":"https://example.com/app"}`)
	mustWriteValidateFile(t, dir, filepath.Join(".codex-plugin", "plugin.json"), `{"name":"x","version":"0.1.0","description":"x","mcpServers":"./.mcp.json","apps":"./.app.json"}`)
	mustWriteValidateFile(t, dir, ".mcp.json", `{"other":{"url":"https://example.com/other"}}`)
	mustWriteValidateFile(t, dir, ".app.json", `{"name":"other-app","url":"https://example.com/other"}`)

	report, err := Validate(dir, "codex-package")
	if err != nil {
		t.Fatal(err)
	}
	var foundMCP, foundApp bool
	for _, failure := range report.Failures {
		switch {
		case strings.Contains(failure.Message, "Codex MCP manifest .mcp.json does not match authored portable MCP projection"):
			foundMCP = true
		case strings.Contains(failure.Message, "Codex app manifest .app.json does not match targets/codex-package/app.json"):
			foundApp = true
		}
	}
	if !foundMCP || !foundApp {
		t.Fatalf("failures = %+v", report.Failures)
	}
}

func TestValidate_CodexRejectsGeneratedMarketplaceMismatch(t *testing.T) {
	root := t.TempDir()
	mustWriteValidateFile(t, root, filepath.Join(pluginmodel.SourceDirName, "plugin.yaml"), `api_version: v1
name: "demo-codex-package"
version: "0.1.0"
description: "demo"
targets: ["codex-package"]
`)
	mustWriteValidateFile(t, root, filepath.Join(pluginmodel.SourceDirName, "targets", "codex-package", "package.yaml"), "homepage: https://example.com/demo\n")
	mustWriteValidateFile(t, root, filepath.Join(pluginmodel.SourceDirName, "publish", "codex", "marketplace.yaml"), "api_version: v1\nmarketplace_name: local-repo\ncategory: Productivity\n")

	result, err := pluginmanifest.Generate(root, "codex-package")
	if err != nil {
		t.Fatal(err)
	}
	if err := pluginmanifest.WriteArtifacts(root, result.Artifacts); err != nil {
		t.Fatal(err)
	}
	mustWriteValidateFile(t, root, filepath.Join(".agents", "plugins", "marketplace.json"), `{"name":"drifted","plugins":[]}`)

	report, err := Validate(root, "codex-package")
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, failure := range report.Failures {
		if failure.Kind == FailureGeneratedContractInvalid &&
			failure.Path == filepath.ToSlash(filepath.Join(".agents", "plugins", "marketplace.json")) {
			found = true
		}
	}
	if !found {
		t.Fatalf("failures = %+v", report.Failures)
	}
}

func TestValidate_ClaudeRejectsGeneratedMarketplaceMismatch(t *testing.T) {
	root := t.TempDir()
	mustWriteValidateFile(t, root, filepath.Join(pluginmodel.SourceDirName, "plugin.yaml"), `api_version: v1
name: "demo-claude"
version: "0.1.0"
description: "demo"
targets: ["claude"]
`)
	mustWriteValidateFile(t, root, filepath.Join(pluginmodel.SourceDirName, pluginmanifest.LauncherFileName), "runtime: go\nentrypoint: ./bin/demo-claude\n")
	mustWriteValidateFile(t, root, filepath.Join(pluginmodel.SourceDirName, "publish", "claude", "marketplace.yaml"), "api_version: v1\nmarketplace_name: acme-tools\nowner_name: ACME Team\n")

	result, err := pluginmanifest.Generate(root, "claude")
	if err != nil {
		t.Fatal(err)
	}
	if err := pluginmanifest.WriteArtifacts(root, result.Artifacts); err != nil {
		t.Fatal(err)
	}
	mustWriteValidateFile(t, root, filepath.Join(".claude-plugin", "marketplace.json"), `{"name":"drifted","owner":{"name":"other"},"plugins":[]}`)

	report, err := Validate(root, "claude")
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, failure := range report.Failures {
		if failure.Kind == FailureGeneratedContractInvalid &&
			failure.Path == filepath.ToSlash(filepath.Join(".claude-plugin", "marketplace.json")) {
			found = true
		}
	}
	if !found {
		t.Fatalf("failures = %+v", report.Failures)
	}
}

func TestValidate_CodexRejectsConfigExtraCanonicalOverrideAndModelDrift(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mustWriteValidateFile(t, dir, "README.md", "# x\n")
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, "plugin.yaml"), `api_version: v1
name: "x"
version: "0.1.0"
description: "x"
targets: ["codex-runtime"]
`)
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, pluginmanifest.LauncherFileName), "runtime: go\nentrypoint: ./bin/x\n")
	mustWriteValidateFile(t, dir, "go.mod", "module example.com/x\n\ngo 1.22\n")
	mustWriteValidateFile(t, dir, filepath.Join("cmd", "x", "main.go"), "package main\nfunc main() {}\n")
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, "targets", "codex-runtime", "package.yaml"), "model_hint: gpt-5.4-mini\n")
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, "targets", "codex-runtime", "config.extra.toml"), "notify = [\"./bin/x\", \"notify\"]\n")
	mustWriteValidateFile(t, dir, filepath.Join(".codex", "config.toml"), "model = \"gpt-4.1\"\nnotify = [\"./bin/other\", \"notify\"]\n")

	report, err := Validate(dir, "codex-runtime")
	if err != nil {
		t.Fatal(err)
	}
	var foundExtra, foundNotify, foundModel bool
	for _, failure := range report.Failures {
		if strings.Contains(failure.Message, `codex-runtime config.extra.toml may not override canonical field "notify"`) {
			foundExtra = true
		}
		if strings.Contains(failure.Message, `entrypoint mismatch: Codex notify argv uses ["./bin/other" "notify"]`) {
			foundNotify = true
		}
		if strings.Contains(failure.Message, `does not match expected model "gpt-5.4-mini"`) {
			foundModel = true
		}
	}
	if !foundExtra || !foundNotify || !foundModel {
		t.Fatalf("failures = %+v", report.Failures)
	}
}

func TestValidate_CodexRejectsMalformedGeneratedConfigShapeAndExtraMismatch(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mustWriteValidateFile(t, dir, "README.md", "# x\n")
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, "plugin.yaml"), `api_version: v1
name: "x"
version: "0.1.0"
description: "x"
targets: ["codex-runtime"]
`)
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, pluginmanifest.LauncherFileName), "runtime: go\nentrypoint: ./bin/x\n")
	mustWriteValidateFile(t, dir, "go.mod", "module example.com/x\n\ngo 1.22\n")
	mustWriteValidateFile(t, dir, filepath.Join("cmd", "x", "main.go"), "package main\nfunc main() {}\n")
	mustWriteValidateFile(t, dir, filepath.Join(".codex", "config.toml"), "model = [\"bad\"]\nnotify = [\"./bin/x\", \"notify\"]\n")

	report, err := Validate(dir, "codex-runtime")
	if err != nil {
		t.Fatal(err)
	}
	var foundMalformed bool
	for _, failure := range report.Failures {
		if failure.Path == filepath.ToSlash(filepath.Join(".codex", "config.toml")) &&
			strings.Contains(failure.Message, `Codex config field "model" must be a string`) {
			foundMalformed = true
		}
	}
	if !foundMalformed {
		t.Fatalf("failures = %+v", report.Failures)
	}

	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, "targets", "codex-runtime", "config.extra.toml"), "approval_policy = \"never\"\n[ui]\nverbose = true\n")
	mustWriteValidateFile(t, dir, filepath.Join(".codex", "config.toml"), "model = \"gpt-5.4-mini\"\nnotify = [\"./bin/x\", \"notify\"]\napproval_policy = \"on-request\"\n[ui]\nverbose = false\n")
	report, err = Validate(dir, "codex-runtime")
	if err != nil {
		t.Fatal(err)
	}
	var foundMismatch bool
	for _, failure := range report.Failures {
		if failure.Path == filepath.ToSlash(filepath.Join(".codex", "config.toml")) &&
			strings.Contains(failure.Message, "passthrough fields do not match targets/codex-runtime/config.extra.toml") {
			foundMismatch = true
		}
	}
	if !foundMismatch {
		t.Fatalf("failures = %+v", report.Failures)
	}
}

func TestValidate_GeminiRejectsManifestExtraCanonicalOverride(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, "plugin.yaml"), `api_version: v1
name: "gemini-demo"
version: "0.1.0"
description: "demo"
targets: ["gemini"]
`)
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, "targets", "gemini", "contexts", "GEMINI.md"), "# Gemini\n")
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, "targets", "gemini", "manifest.extra.json"), `{"plan":{"directory":".gemini/other"}}`)

	report, err := Validate(dir, "gemini")
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, failure := range report.Failures {
		if strings.Contains(failure.Message, `gemini manifest.extra.json may not override canonical field "plan.directory"`) {
			found = true
		}
	}
	if !found {
		t.Fatalf("failures = %+v", report.Failures)
	}
}

func TestValidate_GeminiRejectsInvalidMCPTransportShape(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, "plugin.yaml"), `api_version: v1
name: "gemini-transport"
version: "0.1.0"
description: "demo"
targets: ["gemini"]
`)
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, "targets", "gemini", "contexts", "GEMINI.md"), "# Gemini\n")
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, "mcp", "servers.yaml"), `api_version: v1

servers:
  stdio-and-sse:
    type: stdio
    stdio:
      command: node
    overrides:
      gemini:
        url: "https://example.com/sse"
  bad-args:
    type: remote
    remote:
      protocol: streamable_http
      url: "https://example.com/http"
    overrides:
      gemini:
        args:
          - serve
  bad-cwd:
    type: stdio
    stdio:
      command: node
      args:
        - server.mjs
    overrides:
      gemini:
        cwd: true
`)

	report, err := Validate(dir, "gemini")
	if err != nil {
		t.Fatal(err)
	}
	var foundTransport, foundArgs, foundCwd bool
	for _, failure := range report.Failures {
		switch {
		case strings.Contains(failure.Message, "must define exactly one transport"):
			foundTransport = true
		case strings.Contains(failure.Message, "may only use args with command-based stdio transport"):
			foundArgs = true
		case strings.Contains(failure.Message, "cwd must be a non-empty string"):
			foundCwd = true
		}
	}
	if !foundTransport || !foundArgs || !foundCwd {
		t.Fatalf("failures = %+v", report.Failures)
	}
}

func TestValidate_GeminiRejectsMalformedSettingsThemesAndExcludeTools(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, "plugin.yaml"), `api_version: v1
name: "gemini-assets"
version: "0.1.0"
description: "demo"
targets: ["gemini"]
`)
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, "targets", "gemini", "contexts", "GEMINI.md"), "# Gemini\n")
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, "targets", "gemini", "package.yaml"), "exclude_tools:\n  - \"\"\n")
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, "targets", "gemini", "settings", "api-token.yaml"), "name: api-token\ndescription: token\nenv_var: API_TOKEN\nsensitive: maybe\n")
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, "targets", "gemini", "themes", "empty.yaml"), "name: release-dawn\n")

	report, err := Validate(dir, "gemini")
	if err != nil {
		t.Fatal(err)
	}
	var foundExclude, foundSetting, foundTheme bool
	for _, failure := range report.Failures {
		switch {
		case strings.Contains(failure.Message, "exclude_tools entries must be non-empty strings"):
			foundExclude = true
		case strings.Contains(failure.Message, "Gemini setting file"):
			foundSetting = true
		case strings.Contains(failure.Message, "Gemini themes require at least one theme token besides name"):
			foundTheme = true
		}
	}
	if !foundExclude || !foundSetting || !foundTheme {
		t.Fatalf("failures = %+v", report.Failures)
	}
}

func TestValidate_GeminiRejectsInvalidThemeShapeAndDuplicateSettings(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, "plugin.yaml"), `api_version: v1
name: "gemini-assets"
version: "0.1.0"
description: "demo"
targets: ["gemini"]
`)
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, "targets", "gemini", "contexts", "GEMINI.md"), "# Gemini\n")
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, "targets", "gemini", "settings", "first.yaml"), "name: release-profile\ndescription: one\nenv_var: RELEASE_PROFILE\nsensitive: false\n")
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, "targets", "gemini", "settings", "second.yaml"), "name: duplicate\ndescription: two\nenv_var: RELEASE_PROFILE\nsensitive: false\n")
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, "targets", "gemini", "themes", "broken.yaml"), "name: release-dawn\nbackground: \"#fff9f2\"\n")

	report, err := Validate(dir, "gemini")
	if err != nil {
		t.Fatal(err)
	}
	var foundTheme, foundDuplicate bool
	for _, failure := range report.Failures {
		switch {
		case strings.Contains(failure.Message, `Gemini theme key "background" must be a YAML object`):
			foundTheme = true
		case strings.Contains(failure.Message, `duplicates env_var "RELEASE_PROFILE"`):
			foundDuplicate = true
		}
	}
	if !foundTheme || !foundDuplicate {
		t.Fatalf("failures = %+v", report.Failures)
	}
}

func TestValidate_GeminiRejectsMalformedGeneratedExtensionManifest(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, "plugin.yaml"), `api_version: v1
name: "gemini-assets"
version: "0.1.0"
description: "demo"
targets: ["gemini"]
`)
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, "targets", "gemini", "contexts", "GEMINI.md"), "# Gemini\n")
	mustWriteValidateFile(t, dir, "gemini-extension.json", `{"name":"gemini-assets","version":"0.1.0","description":"demo","settings":{}}`)

	report, err := Validate(dir, "gemini")
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, failure := range report.Failures {
		if failure.Path == "gemini-extension.json" &&
			strings.Contains(failure.Message, `Gemini extension field "settings" must be an array of JSON objects`) {
			found = true
		}
	}
	if !found {
		t.Fatalf("failures = %+v", report.Failures)
	}
}

func TestValidate_GeminiRejectsGeneratedContextMismatch(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, "plugin.yaml"), `api_version: v1
name: "gemini-assets"
version: "0.1.0"
description: "demo"
targets: ["gemini"]
`)
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, "targets", "gemini", "contexts", "GEMINI.md"), "# Gemini\n")
	mustWriteValidateFile(t, dir, "gemini-extension.json", `{"name":"gemini-assets","version":"0.1.0","description":"demo","contextFileName":"OTHER.md"}`)

	report, err := Validate(dir, "gemini")
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, failure := range report.Failures {
		if failure.Path == "gemini-extension.json" &&
			strings.Contains(failure.Message, `sets contextFileName "OTHER.md"; expected "GEMINI.md"`) {
			found = true
		}
	}
	if !found {
		t.Fatalf("failures = %+v", report.Failures)
	}
}

func TestValidate_GeminiRejectsGeneratedMetadataMismatch(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, "plugin.yaml"), `api_version: v1
name: "gemini-assets"
version: "0.1.0"
description: "demo"
targets: ["gemini"]
`)
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, "targets", "gemini", "package.yaml"), `context_file_name: GEMINI.md
exclude_tools:
  - run_shell_command(rm -rf)
migrated_to: https://example.com/new-home
plan_directory: .gemini/plans
`)
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, "targets", "gemini", "contexts", "GEMINI.md"), "# Gemini\n")
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, "targets", "gemini", "settings", "release-profile.yaml"), "name: release-profile\ndescription: Release profile\nenv_var: RELEASE_PROFILE\nsensitive: false\n")
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, "targets", "gemini", "themes", "release-dawn.yaml"), "name: release-dawn\nbackground:\n  primary: \"#fff9f2\"\n")
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, "mcp", "servers.yaml"), `api_version: v1

servers:
  docs:
    type: remote
    remote:
      protocol: streamable_http
      url: "https://example.com/mcp"
    targets:
      - "gemini"
`)
	mustWriteValidateFile(t, dir, "gemini-extension.json", `{"name":"wrong-name","version":"2.0.0","description":"other","contextFileName":"OTHER.md","excludeTools":["other_tool"],"migratedTo":"https://example.com/other","plan":{"directory":"other-plans"},"settings":[{"name":"wrong","description":"Wrong","envVar":"WRONG","sensitive":false}],"themes":[{"name":"wrong-theme","background":{"primary":"#000000"}}],"mcpServers":{"other":{"command":"node","args":["server.mjs"]}}}`)

	report, err := Validate(dir, "gemini")
	if err != nil {
		t.Fatal(err)
	}
	var foundName, foundMeta, foundSettings, foundThemes, foundMCP bool
	for _, failure := range report.Failures {
		switch {
		case strings.Contains(failure.Message, `sets name "wrong-name"; expected "gemini-assets"`):
			foundName = true
		case strings.Contains(failure.Message, "package metadata does not match targets/gemini/package.yaml"):
			foundMeta = true
		case strings.Contains(failure.Message, "settings do not match authored targets/gemini/settings/**"):
			foundSettings = true
		case strings.Contains(failure.Message, "themes do not match authored targets/gemini/themes/**"):
			foundThemes = true
		case strings.Contains(failure.Message, "mcpServers do not match authored portable MCP projection"):
			foundMCP = true
		}
	}
	if !foundName || !foundMeta || !foundSettings || !foundThemes || !foundMCP {
		t.Fatalf("failures = %+v", report.Failures)
	}
}

func TestValidate_GeminiRejectsGeneratedHooksMismatch(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, "plugin.yaml"), `api_version: v1
name: "gemini-assets"
version: "0.1.0"
description: "demo"
targets: ["gemini"]
`)
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, "targets", "gemini", "contexts", "GEMINI.md"), "# Gemini\n")
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, "targets", "gemini", "hooks", "hooks.json"), `{"hooks":{"SessionStart":[{"matcher":"*","hooks":[{"type":"command","command":"./bin/demo GeminiSessionStart"}]}]}}`)
	mustWriteValidateFile(t, dir, "gemini-extension.json", `{"name":"gemini-assets","version":"0.1.0","description":"demo","contextFileName":"GEMINI.md"}`)
	mustWriteValidateFile(t, dir, filepath.Join("hooks", "hooks.json"), `{"hooks":{"SessionStart":[{"matcher":"resume","hooks":[{"type":"command","command":"./bin/other GeminiSessionStart"}]}]}}`)

	report, err := Validate(dir, "gemini")
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, failure := range report.Failures {
		if failure.Path == filepath.ToSlash(filepath.Join("hooks", "hooks.json")) &&
			strings.Contains(failure.Message, "does not match authored targets/gemini/hooks/hooks.json") {
			found = true
		}
	}
	if !found {
		t.Fatalf("failures = %+v", report.Failures)
	}
}

func TestValidate_GeminiRejectsManagedGeneratedHooksDrift(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, "plugin.yaml"), `api_version: v1
name: "gemini-managed"
version: "0.1.0"
description: "demo"
targets: ["gemini"]
`)
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, pluginmanifest.LauncherFileName), "runtime: go\nentrypoint: ./bin/demo\n")
	mustWriteValidateFile(t, dir, "go.mod", "module example.com/gemini-managed\n\ngo 1.22\n")
	mustWriteValidateFile(t, dir, filepath.Join("cmd", "demo", "main.go"), "package main\nfunc main() {}\n")
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, "targets", "gemini", "contexts", "GEMINI.md"), "# Gemini\n")
	mustWriteValidateFile(t, dir, "gemini-extension.json", `{"name":"gemini-managed","version":"0.1.0","description":"demo","contextFileName":"GEMINI.md"}`)
	mustWriteValidateFile(t, dir, filepath.Join("hooks", "hooks.json"), `{"hooks":{"SessionStart":[{"matcher":"resume","hooks":[{"type":"command","command":"./bin/other GeminiSessionStart"}]}]}}`)

	report, err := Validate(dir, "gemini")
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, failure := range report.Failures {
		if failure.Path == filepath.ToSlash(filepath.Join("hooks", "hooks.json")) &&
			strings.Contains(failure.Message, "does not match the managed launcher-derived hooks projection") {
			found = true
		}
	}
	if !found {
		t.Fatalf("failures = %+v", report.Failures)
	}
}

func TestValidate_RejectsRemovedPortableAgents(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, "plugin.yaml"), `api_version: v1
name: "demo"
version: "0.1.0"
description: "demo"
targets: ["claude"]
`)
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, pluginmanifest.LauncherFileName), "runtime: go\nentrypoint: ./bin/demo\n")
	mustWriteValidateFile(t, dir, "go.mod", "module example.com/demo\n\ngo 1.22\n")
	mustWriteValidateFile(t, dir, filepath.Join("cmd", "demo", "main.go"), "package main\nfunc main() {}\n")
	mustWriteValidateFile(t, dir, filepath.Join("agents", "reviewer.md"), "# reviewer\n")

	report, err := Validate(dir, "claude")
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, failure := range report.Failures {
		if strings.Contains(failure.Message, "portable agents were removed") {
			found = true
		}
	}
	if !found {
		t.Fatalf("failures = %+v", report.Failures)
	}
}

func TestValidate_ClaudeRejectsUnsupportedContextsSurface(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, "plugin.yaml"), `api_version: v1
name: "demo"
version: "0.1.0"
description: "demo"
targets: ["claude"]
`)
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, pluginmanifest.LauncherFileName), "runtime: go\nentrypoint: ./bin/demo\n")
	mustWriteValidateFile(t, dir, "go.mod", "module example.com/demo\n\ngo 1.22\n")
	mustWriteValidateFile(t, dir, filepath.Join("cmd", "demo", "main.go"), "package main\nfunc main() {}\n")
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, "targets", "claude", "hooks", "hooks.json"), "{\"hooks\":{}}\n")
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, "targets", "claude", "contexts", "context.md"), "# context\n")

	report, err := Validate(dir, "claude")
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, failure := range report.Failures {
		if strings.Contains(failure.Message, "does not support authored surface contexts") {
			found = true
		}
	}
	if !found {
		t.Fatalf("failures = %+v", report.Failures)
	}
}

func TestValidate_ClaudeRejectsInvalidSettingsLSPAndUserConfig(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, "plugin.yaml"), `api_version: v1
name: "demo"
version: "0.1.0"
description: "demo"
targets: ["claude"]
`)
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, pluginmanifest.LauncherFileName), "runtime: go\nentrypoint: ./bin/demo\n")
	mustWriteValidateFile(t, dir, "go.mod", "module example.com/demo\n\ngo 1.22\n")
	mustWriteValidateFile(t, dir, filepath.Join("cmd", "demo", "main.go"), "package main\nfunc main() {}\n")
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, "targets", "claude", "hooks", "hooks.json"), "{\"hooks\":{}}\n")
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, "targets", "claude", "settings.json"), `{"agent":42}`)
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, "targets", "claude", "lsp.json"), `{"servers":true}`)
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, "targets", "claude", "user-config.json"), `{"api_token":"not-an-object"}`)

	report, err := Validate(dir, "claude")
	if err != nil {
		t.Fatal(err)
	}
	var foundSettings, foundUserConfig bool
	for _, failure := range report.Failures {
		switch {
		case strings.Contains(failure.Message, `must set "agent" as a non-empty string`):
			foundSettings = true
		case strings.Contains(failure.Message, `must be a JSON object`):
			foundUserConfig = true
		}
	}
	if !foundSettings || !foundUserConfig {
		t.Fatalf("failures = %+v", report.Failures)
	}
}

func TestValidate_ClaudeRejectsHooksWithoutLauncher(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, "plugin.yaml"), `api_version: v1
name: "demo"
version: "0.1.0"
description: "demo"
targets: ["claude"]
`)
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, "targets", "claude", "hooks", "hooks.json"), `{"hooks":{}}`)

	report, err := Validate(dir, "claude")
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, failure := range report.Failures {
		if failure.Path == filepath.ToSlash(filepath.Join(pluginmodel.SourceDirName, "targets", "claude", "hooks", "hooks.json")) &&
			strings.Contains(failure.Message, "require plugin/launcher.yaml") {
			found = true
		}
	}
	if !found {
		t.Fatalf("failures = %+v", report.Failures)
	}
}

func TestValidate_ClaudeRejectsLauncherlessEmptyTarget(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, "plugin.yaml"), `api_version: v1
name: "demo"
version: "0.1.0"
description: "demo"
targets: ["claude"]
`)

	report, err := Validate(dir, "claude")
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, failure := range report.Failures {
		if failure.Path == filepath.Join(pluginmodel.SourceDirName, pluginmanifest.FileName) &&
			strings.Contains(failure.Message, "package-only surface") {
			found = true
		}
	}
	if !found {
		t.Fatalf("failures = %+v", report.Failures)
	}
}

func TestValidate_ManifestProject_WindowsCmdLauncherAccepted(t *testing.T) {
	t.Parallel()
	if runtime.GOOS != "windows" {
		t.Skip("windows-specific")
	}
	dir := t.TempDir()
	mustWriteValidateFile(t, dir, "README.md", "# x\n")
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, "plugin.yaml"), `api_version: v1
name: "x"
version: "0.1.0"
description: "x"
targets: ["codex-runtime"]
`)
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, pluginmanifest.LauncherFileName), "runtime: python\nentrypoint: ./bin/x\n")
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, "targets", "codex-runtime", "package.yaml"), "model_hint: gpt-5.4-mini\n")
	mustWriteValidateFile(t, dir, filepath.Join(".codex", "config.toml"), "notify = [\"./bin/x\", \"notify\"]\n")
	mustWriteValidateFile(t, dir, filepath.Join("bin", "x.cmd"), "@echo off\r\n")
	mustWriteValidateFile(t, dir, filepath.Join(pluginmodel.SourceDirName, "main.py"), "print('ok')\n")

	report, err := Validate(dir, "codex-runtime")
	if err != nil {
		t.Fatal(err)
	}
	for _, failure := range report.Failures {
		if failure.Kind == FailureLauncherInvalid {
			t.Fatalf("unexpected launcher failure: %+v", report.Failures)
		}
	}
}

func TestValidateNodeRuntimeTarget_BuiltOutputWithoutTSConfigFails(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mustWriteValidateFile(t, dir, filepath.Join("bin", "x"), "#!/usr/bin/env bash\nset -euo pipefail\nROOT=\"$(CDPATH= cd -- \"$(dirname -- \"$0\")/..\" && pwd)\"\nexec node \"$ROOT/dist/main.js\" \"$@\"\n")
	mustChmodExecutable(t, filepath.Join(dir, "bin", "x"))
	mustWriteValidateFile(t, dir, "package.json", `{"scripts":{"build":"node build.js"}}`)

	var report Report
	validateNodeRuntimeTarget(dir, "./bin/x", &report)
	if len(report.Failures) != 1 {
		t.Fatalf("failures = %+v", report.Failures)
	}
	failure := report.Failures[0]
	if failure.Kind != FailureLauncherInvalid {
		t.Fatalf("failure kind = %q", failure.Kind)
	}
	if failure.Path != "dist/main.js" {
		t.Fatalf("failure path = %q", failure.Path)
	}
	if !strings.Contains(failure.Message, "tsconfig.json is missing") {
		t.Fatalf("failure message = %q", failure.Message)
	}
}

func TestValidatePluginLauncher_MissingEntrypointSetsFailurePath(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	var report Report

	validatePluginLauncher(dir, &pluginmanifest.Launcher{
		Runtime:    "python",
		Entrypoint: "./bin/missing-launcher",
	}, &report)

	if len(report.Failures) != 1 {
		t.Fatalf("failures = %+v", report.Failures)
	}
	failure := report.Failures[0]
	if failure.Kind != FailureLauncherInvalid {
		t.Fatalf("failure kind = %q", failure.Kind)
	}
	if failure.Path != "./bin/missing-launcher" {
		t.Fatalf("failure path = %q", failure.Path)
	}
	if !strings.Contains(failure.Message, "launcher invalid: missing ./bin/missing-launcher") {
		t.Fatalf("failure message = %q", failure.Message)
	}
}

func TestValidateNodeRuntimeTarget_TypeScriptLaneShowsTypeScriptGuidance(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mustWriteValidateFile(t, dir, filepath.Join("bin", "x"), "#!/usr/bin/env bash\nset -euo pipefail\nROOT=\"$(CDPATH= cd -- \"$(dirname -- \"$0\")/..\" && pwd)\"\nexec node \"$ROOT/dist/main.js\" \"$@\"\n")
	mustChmodExecutable(t, filepath.Join(dir, "bin", "x"))
	mustWriteValidateFile(t, dir, "tsconfig.json", "{}\n")
	mustWriteValidateFile(t, dir, "package.json", `{"scripts":{"build":"tsc -p tsconfig.json"}}`)

	var report Report
	validateNodeRuntimeTarget(dir, "./bin/x", &report)
	if len(report.Failures) != 1 {
		t.Fatalf("failures = %+v", report.Failures)
	}
	failure := report.Failures[0]
	if failure.Path != "dist/main.js" {
		t.Fatalf("failure path = %q", failure.Path)
	}
	if !strings.Contains(failure.Message, "TypeScript scaffold expects built output") {
		t.Fatalf("failure message = %q", failure.Message)
	}
	if !strings.Contains(failure.Message, "plugin-kit-ai bootstrap .") {
		t.Fatalf("failure message = %q", failure.Message)
	}
}

func TestValidateNodeRuntimeTarget_TypeScriptOutDirMismatchFails(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mustWriteValidateFile(t, dir, filepath.Join("bin", "x"), "#!/usr/bin/env bash\nset -euo pipefail\nROOT=\"$(CDPATH= cd -- \"$(dirname -- \"$0\")/..\" && pwd)\"\nexec node \"$ROOT/build/main.js\" \"$@\"\n")
	mustChmodExecutable(t, filepath.Join(dir, "bin", "x"))
	mustWriteValidateFile(t, dir, "tsconfig.json", `{"compilerOptions":{"outDir":"dist"}}`)
	mustWriteValidateFile(t, dir, "package.json", `{"scripts":{"build":"tsc -p tsconfig.json"}}`)

	var report Report
	validateNodeRuntimeTarget(dir, "./bin/x", &report)
	if len(report.Failures) != 1 {
		t.Fatalf("failures = %+v", report.Failures)
	}
	failure := report.Failures[0]
	if failure.Kind != FailureLauncherInvalid {
		t.Fatalf("failure kind = %q", failure.Kind)
	}
	if failure.Path != "build/main.js" {
		t.Fatalf("failure path = %q", failure.Path)
	}
	if !strings.Contains(failure.Message, "outside tsconfig outDir dist") {
		t.Fatalf("failure message = %q", failure.Message)
	}
}

func TestValidateRuntimeTargetExecutable_NonExecutableScriptFails(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("unix-only executable-bit check")
	}

	dir := t.TempDir()
	mustWriteValidateFile(t, dir, filepath.Join("scripts", "main.sh"), "#!/usr/bin/env bash\nexit 0\n")

	var report Report
	validateRuntimeTargetExecutable(dir, filepath.Join("scripts", "main.sh"), &report)
	if len(report.Failures) != 1 {
		t.Fatalf("failures = %+v", report.Failures)
	}
	failure := report.Failures[0]
	if failure.Kind != FailureRuntimeTargetMissing {
		t.Fatalf("failure kind = %q", failure.Kind)
	}
	if failure.Path != filepath.Join("scripts", "main.sh") {
		t.Fatalf("failure path = %q", failure.Path)
	}
	if !strings.Contains(failure.Message, "is not executable") {
		t.Fatalf("failure message = %q", failure.Message)
	}
}

func TestShellLauncherPassthrough(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("windows shell launcher is mediated through the generated .cmd wrapper")
	}
	parent := t.TempDir()
	dir := filepath.Join(parent, "project with spaces")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	mustWriteValidateFile(t, dir, filepath.Join("bin", "x"), "#!/usr/bin/env bash\nset -euo pipefail\nROOT=\"$(CDPATH= cd -- \"$(dirname -- \"$0\")/..\" && pwd)\"\nexec \"$ROOT/scripts/main.sh\" \"$@\"\n")
	mustWriteValidateFile(t, dir, filepath.Join("scripts", "main.sh"), "#!/usr/bin/env bash\nset -euo pipefail\nhook_name=\"${1:-}\"\nif [[ \"$hook_name\" == \"notify\" ]]; then\n  printf '%s' \"$2\" >&2\n  exit 7\nfi\ncat >/dev/null\nprintf '{}'\n")
	mustChmodExecutable(t, filepath.Join(dir, "bin", "x"))
	mustChmodExecutable(t, filepath.Join(dir, "scripts", "main.sh"))

	cmd := exec.Command(filepath.Join(dir, "bin", "x"), "Stop")
	cmd.Stdin = strings.NewReader("{\"ok\":true}\n")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("launcher stop: %v\n%s", err, out)
	}
	if string(out) != "{}" {
		t.Fatalf("stdout = %q, want {}", string(out))
	}

	cmd = exec.Command(filepath.Join(dir, "bin", "x"), "notify", `{"client":"codex-tui"}`)
	out, err = cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected non-zero exit")
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("want ExitError, got %v", err)
	}
	if exitErr.ExitCode() != 7 {
		t.Fatalf("exit code = %d, want 7", exitErr.ExitCode())
	}
	if strings.TrimSpace(string(out)) != `{"client":"codex-tui"}` {
		t.Fatalf("stderr passthrough = %q", string(out))
	}
}

func mustWriteValidateFile(t *testing.T, root, rel, body string) {
	t.Helper()
	rel = testValidateAuthoredRel(rel)
	full := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func testValidateAuthoredRel(rel string) string {
	rel = filepath.ToSlash(strings.TrimSpace(rel))
	switch {
	case rel == pluginmodel.LegacySourceDirName:
		return pluginmodel.SourceDirName
	case strings.HasPrefix(rel, pluginmodel.LegacySourceDirName+"/"):
		return filepath.ToSlash(filepath.Join(pluginmodel.SourceDirName, strings.TrimPrefix(rel, pluginmodel.LegacySourceDirName+"/")))
	default:
		return rel
	}
}

func mustChmodExecutable(t *testing.T, full string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		return
	}
	if err := os.Chmod(full, 0o755); err != nil {
		t.Fatal(err)
	}
}
