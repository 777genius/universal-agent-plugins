package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/app"
	"github.com/777genius/plugin-kit-ai/cli/internal/scaffold"
)

type fakeInitRunner struct {
	gotOpts app.InitOptions
	outDir  string
	err     error
}

func (f *fakeInitRunner) Run(opts app.InitOptions) (string, error) {
	f.gotOpts = opts
	return f.outDir, f.err
}

func executeInitCommand(t *testing.T, cmdArgs ...string) (string, error) {
	t.Helper()
	runner := &fakeInitRunner{outDir: "/tmp/demo plugin"}
	cmd := newInitCmd(runner)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs(cmdArgs)
	err := cmd.Execute()
	return buf.String(), err
}

func TestInitHelpIncludesScenarioLanesAndDefaults(t *testing.T) {
	t.Parallel()
	output, err := executeInitCommand(t, "--help")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"Connect an online service",
		"Connect a local tool",
		"Build custom plugin logic - Advanced",
		"Already have native config",
		"plugin-kit-ai import",
		`--template   Recommended start: "online-service", "local-tool", or "custom-logic".`,
		`--platform   Advanced override: "codex-runtime" (default), "codex-package", "claude", "gemini", "opencode", "cursor", or "cursor-workspace".`,
		`--runtime    Supported: "go" (default), "python", "node", "shell" for launcher-based targets only.`,
		"--typescript Generate a TypeScript scaffold on top of the node runtime lane",
		"--runtime-package",
		"--runtime-package-version",
		"import the shared plugin-kit-ai-runtime package instead of vendoring the helper file",
		"Plain init remains as a legacy compatibility path",
		"--claude-extended-hooks",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("help output missing %q:\n%s", want, output)
		}
	}
}

func TestInitCommandUsesDefaultPlatformAndRuntime(t *testing.T) {
	t.Parallel()
	runner := &fakeInitRunner{outDir: "/tmp/demo plugin"}
	cmd := newInitCmd(runner)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"demo"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if runner.gotOpts.Platform != "codex-runtime" {
		t.Fatalf("platform = %q, want codex-runtime", runner.gotOpts.Platform)
	}
	if runner.gotOpts.Runtime != "go" {
		t.Fatalf("runtime = %q, want go", runner.gotOpts.Runtime)
	}
	if runner.gotOpts.Template != "" {
		t.Fatalf("template = %q, want empty", runner.gotOpts.Template)
	}
}

func TestInitSuccessOutputByLane(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name         string
		args         []string
		want         []string
		notWant      []string
		wantRuntime  string
		wantPlatform string
	}{
		{
			name:         "codex-runtime-go",
			args:         []string{"demo"},
			wantRuntime:  "go",
			wantPlatform: "codex-runtime",
			want: []string{
				`Created plugin "demo" at /tmp/demo plugin`,
				`cd "/tmp/demo plugin"`,
				"go mod tidy",
				"go build -o bin/demo ./cmd/demo",
				"plugin-kit-ai validate . --platform codex-runtime --strict",
				"plugin-kit-ai test . --platform codex-runtime --event Notify",
				"plugin-kit-ai dev . --platform codex-runtime --event Notify",
				"See plugin/README.md for SDK setup and first-run steps",
				"Legacy compatibility path. For a new online service or local tool repo, start with --template online-service or --template local-tool instead.",
			},
		},
		{
			name:         "codex-runtime-python",
			args:         []string{"demo", "--runtime", "python"},
			wantRuntime:  "python",
			wantPlatform: "codex-runtime",
			want: []string{
				"plugin-kit-ai doctor .",
				"plugin-kit-ai bootstrap .",
				"plugin-kit-ai validate . --platform codex-runtime --strict",
				"plugin-kit-ai test . --platform codex-runtime --event Notify",
				"plugin-kit-ai dev . --platform codex-runtime --event Notify",
				"See plugin/README.md for the full first run",
				"Legacy compatibility path. For a new online service or local tool repo, start with --template online-service or --template local-tool instead.",
			},
			notWant: []string{
				"production-ready",
			},
		},
		{
			name:         "codex-runtime-node",
			args:         []string{"demo", "--runtime", "node"},
			wantRuntime:  "node",
			wantPlatform: "codex-runtime",
			want: []string{
				"plugin-kit-ai doctor .",
				"plugin-kit-ai bootstrap .",
				"plugin-kit-ai validate . --platform codex-runtime --strict",
				"plugin-kit-ai test . --platform codex-runtime --event Notify",
				"plugin-kit-ai dev . --platform codex-runtime --event Notify",
				"See plugin/README.md for the full first run",
				"Legacy compatibility path. For a new online service or local tool repo, start with --template online-service or --template local-tool instead.",
			},
			notWant: []string{
				"production-ready",
			},
		},
		{
			name:         "codex-package",
			args:         []string{"demo", "--platform", "codex-package"},
			wantRuntime:  "",
			wantPlatform: "codex-package",
			want: []string{
				"plugin-kit-ai generate .",
				"plugin-kit-ai generate --check .",
				"plugin-kit-ai validate . --platform codex-package --strict",
				"See plugin/README.md for the full first run",
			},
		},
		{
			name:         "gemini-packaging",
			args:         []string{"demo", "--platform", "gemini"},
			wantRuntime:  "",
			wantPlatform: "gemini",
			want: []string{
				"plugin-kit-ai generate .",
				"plugin-kit-ai generate --check .",
				"plugin-kit-ai validate . --platform gemini --strict",
				"See plugin/README.md for the full first run",
			},
		},
		{
			name:         "gemini-go-runtime",
			args:         []string{"demo", "--platform", "gemini", "--runtime", "go"},
			wantRuntime:  "go",
			wantPlatform: "gemini",
			want: []string{
				"go mod tidy",
				"go test ./...",
				"plugin-kit-ai generate .",
				"plugin-kit-ai generate --check .",
				"plugin-kit-ai validate . --platform gemini --strict",
				"plugin-kit-ai inspect . --target gemini",
				"plugin-kit-ai capabilities --mode runtime --platform gemini",
				"make test-gemini-runtime",
				"gemini extensions link .",
				"make test-gemini-runtime-live",
				"See README.md for Gemini runtime steps",
			},
			notWant: []string{
				"plugin-kit-ai test .",
				"plugin-kit-ai dev .",
				"Legacy compatibility path.",
			},
		},
		{
			name:         "opencode",
			args:         []string{"demo", "--platform", "opencode"},
			wantRuntime:  "",
			wantPlatform: "opencode",
			want: []string{
				"plugin-kit-ai generate .",
				"plugin-kit-ai generate --check .",
				"plugin-kit-ai validate . --platform opencode --strict",
				"See plugin/README.md for the full first run",
			},
		},
		{
			name:         "cursor",
			args:         []string{"demo", "--platform", "cursor"},
			wantRuntime:  "",
			wantPlatform: "cursor",
			want: []string{
				"plugin-kit-ai generate .",
				"plugin-kit-ai generate --check .",
				"plugin-kit-ai validate . --platform cursor --strict",
				"See plugin/README.md for the full first run",
			},
		},
		{
			name:         "config-lane-extras",
			args:         []string{"demo", "--platform", "cursor", "--extras"},
			wantRuntime:  "",
			wantPlatform: "cursor",
			want: []string{
				"Portable MCP starter: plugin/mcp/servers.yaml",
				"plugin-kit-ai generate .",
				"plugin-kit-ai generate --check .",
				"plugin-kit-ai validate . --platform cursor --strict",
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			runner := &fakeInitRunner{outDir: "/tmp/demo plugin"}
			cmd := newInitCmd(runner)
			var buf bytes.Buffer
			cmd.SetOut(&buf)
			cmd.SetErr(&buf)
			cmd.SetArgs(tc.args)
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			output := buf.String()
			for _, want := range tc.want {
				if !strings.Contains(output, want) {
					t.Fatalf("output missing %q:\n%s", want, output)
				}
			}
			for _, unwanted := range tc.notWant {
				if strings.Contains(output, unwanted) {
					t.Fatalf("output unexpectedly contains %q:\n%s", unwanted, output)
				}
			}
			if runner.gotOpts.Runtime != tc.wantRuntime {
				t.Fatalf("runtime = %q, want %q", runner.gotOpts.Runtime, tc.wantRuntime)
			}
			if runner.gotOpts.Platform != tc.wantPlatform {
				t.Fatalf("platform = %q, want %q", runner.gotOpts.Platform, tc.wantPlatform)
			}
		})
	}
}

func TestInitSuccessOutput_CustomLogicHighlightsAdvancedPath(t *testing.T) {
	t.Parallel()
	output, err := executeInitCommand(t, "demo", "--template", "custom-logic")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"plugin-kit-ai inspect . --authoring",
		"go mod tidy",
		"go build -o bin/demo ./cmd/demo",
		"See plugin/README.md for SDK setup and first-run steps",
		"Advanced path: start with plugin/README.md, then grow into deeper runtime and hook details only when you need them.",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("output missing %q:\n%s", want, output)
		}
	}
	if strings.Contains(output, "Legacy compatibility path.") {
		t.Fatalf("custom-logic output should not look like legacy init:\n%s", output)
	}
}

func TestInitCommandPassesRuntimePackageFlag(t *testing.T) {
	t.Parallel()
	runner := &fakeInitRunner{outDir: "/tmp/demo plugin"}
	cmd := newInitCmd(runner)
	cmd.SetArgs([]string{"demo", "--runtime", "node", "--typescript", "--runtime-package", "--runtime-package-version", scaffold.DefaultRuntimePackageVersion})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !runner.gotOpts.RuntimePackage {
		t.Fatal("runtime package flag was not forwarded")
	}
	if runner.gotOpts.RuntimePackageVersion != scaffold.DefaultRuntimePackageVersion {
		t.Fatalf("runtime package version = %q", runner.gotOpts.RuntimePackageVersion)
	}
}

func TestInitCommandUsesOnlineServiceTemplate(t *testing.T) {
	t.Parallel()
	runner := &fakeInitRunner{outDir: "/tmp/demo plugin"}
	cmd := newInitCmd(runner)
	cmd.SetArgs([]string{"demo", "--template", "online-service"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if runner.gotOpts.Template != scaffold.InitTemplateOnlineService {
		t.Fatalf("template = %q", runner.gotOpts.Template)
	}
	if runner.gotOpts.PlatformExplicit {
		t.Fatal("platform should not be explicit")
	}
	if runner.gotOpts.RuntimeExplicit {
		t.Fatal("runtime should not be explicit")
	}
}

func TestInitCommandUsesLocalToolTemplateWithExplicitTarget(t *testing.T) {
	t.Parallel()
	runner := &fakeInitRunner{outDir: "/tmp/demo plugin"}
	cmd := newInitCmd(runner)
	cmd.SetArgs([]string{"demo", "--template", "local-tool", "--platform", "cursor"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if runner.gotOpts.Template != scaffold.InitTemplateLocalTool {
		t.Fatalf("template = %q", runner.gotOpts.Template)
	}
	if runner.gotOpts.Platform != "cursor" || !runner.gotOpts.PlatformExplicit {
		t.Fatalf("platform opts = %+v", runner.gotOpts)
	}
}

func TestInitSuccessOutputForOnlineServiceTemplate(t *testing.T) {
	t.Parallel()
	output := formatInitSuccess("/tmp/demo plugin", app.InitOptions{
		ProjectName: "demo",
		Template:    scaffold.InitTemplateOnlineService,
	})
	for _, want := range []string{
		"plugin-kit-ai inspect . --authoring",
		"plugin-kit-ai generate .",
		"plugin-kit-ai generate --check .",
		"plugin-kit-ai validate . --platform claude --strict",
		"See plugin/README.md for the first run",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("output missing %q:\n%s", want, output)
		}
	}
}

func TestInitCommandRejectsRuntimeOnOnlineServiceTemplate(t *testing.T) {
	t.Parallel()
	cmd := newInitCmd(app.InitRunner{})
	cmd.SetArgs([]string{"demo", "--template", "online-service", "--runtime", "node", "-o", t.TempDir()})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "--runtime is not supported with --template online-service") {
		t.Fatalf("err = %v", err)
	}
}

func TestInitCommandRejectsPackageLaneOnCustomLogicTemplate(t *testing.T) {
	t.Parallel()
	cmd := newInitCmd(app.InitRunner{})
	cmd.SetArgs([]string{"demo", "--template", "custom-logic", "--platform", "cursor", "-o", t.TempDir()})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "--template custom-logic supports launcher-backed targets only") {
		t.Fatalf("err = %v", err)
	}
}

func TestInitCommandUsesReleasedVersionForRuntimePackagePin(t *testing.T) {
	prev := version
	version = "v1.2.3"
	t.Cleanup(func() {
		version = prev
	})

	runner := &fakeInitRunner{outDir: "/tmp/demo plugin"}
	cmd := newInitCmd(runner)
	cmd.SetArgs([]string{"demo", "--runtime", "python", "--runtime-package"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if runner.gotOpts.RuntimePackageVersion != "1.2.3" {
		t.Fatalf("runtime package version = %q, want 1.2.3", runner.gotOpts.RuntimePackageVersion)
	}
}

func TestInitSuccessOutputIncludesSharedHelperDependency(t *testing.T) {
	t.Parallel()
	output := formatInitSuccess("/tmp/demo plugin", app.InitOptions{
		ProjectName:           "demo",
		Platform:              "codex-runtime",
		Runtime:               "python",
		RuntimePackage:        true,
		RuntimePackageVersion: scaffold.DefaultRuntimePackageVersion,
	})
	if !strings.Contains(output, "Shared helper dependency: plugin-kit-ai-runtime@"+scaffold.DefaultRuntimePackageVersion) {
		t.Fatalf("output missing shared helper dependency line:\n%s", output)
	}
	for _, want := range []string{
		"plugin-kit-ai test . --platform codex-runtime --event Notify",
		"plugin-kit-ai dev . --platform codex-runtime --event Notify",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("output missing %q:\n%s", want, output)
		}
	}
}

func TestInitCommandRejectsTypeScriptOutsideNodeRuntime(t *testing.T) {
	t.Parallel()
	cmd := newInitCmd(app.InitRunner{})
	cmd.SetArgs([]string{"demo", "--typescript", "-o", t.TempDir()})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "--typescript requires --runtime node") {
		t.Fatalf("err = %v", err)
	}
}

func TestInitCommandRejectsRuntimePackageOutsidePythonNode(t *testing.T) {
	t.Parallel()
	cmd := newInitCmd(app.InitRunner{})
	cmd.SetArgs([]string{"demo", "--runtime-package", "-o", t.TempDir()})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "--runtime-package requires --runtime python or --runtime node") {
		t.Fatalf("err = %v", err)
	}
}

func TestInitCommandRejectsRuntimePackageWithoutPinnedVersionOnDevBuild(t *testing.T) {
	t.Parallel()
	cmd := newInitCmd(app.InitRunner{})
	cmd.SetArgs([]string{"demo", "--runtime", "python", "--runtime-package", "-o", t.TempDir()})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "--runtime-package requires --runtime-package-version when the CLI build does not have a stable tagged version") {
		t.Fatalf("err = %v", err)
	}
}
