package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/validate"
)

type fakeValidateRunner struct {
	report validate.Report
	err    error
}

func (f fakeValidateRunner) Run(_ string, _ string) (validate.Report, error) {
	return f.report, f.err
}

func TestValidateWritesGeminiRuntimeRecoveryHints(t *testing.T) {
	t.Parallel()
	cmd := newValidateCmd(func(root, platform string) (validate.Report, error) {
		return validate.Report{
			Platform: "gemini",
			Failures: []validate.Failure{{
				Kind:    validate.FailureEntrypointMismatch,
				Path:    "hooks/hooks.json",
				Target:  "gemini",
				Message: `Gemini hook "SessionStart" command "${extensionPath}${/}bin${/}old GeminiSessionStart" does not match launcher entrypoint "./bin/demo"`,
			}},
		}, nil
	})
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"."})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
	output := stderr.String()
	for _, want := range []string{
		`Failure: Gemini hook "SessionStart" command`,
		"Hint: rerun plugin-kit-ai generate . to regenerate Gemini hooks/hooks.json from plugin/launcher.yaml",
		"Hint: after validate is green, run make test-gemini-runtime, relink the extension with gemini extensions link .",
		"make test-gemini-runtime-live",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("stderr missing %q:\n%s", want, output)
		}
	}
}

func TestValidateWritesGeminiWarningHints(t *testing.T) {
	t.Parallel()
	cmd := newValidateCmd(func(root, platform string) (validate.Report, error) {
		return validate.Report{
			Platform: "gemini",
			Warnings: []validate.Warning{
				{Kind: validate.WarningGeminiDirNameMismatch, Message: `Gemini extension directory basename "tmp-ext" does not match extension name "demo-ext"`},
				{Kind: validate.WarningGeminiPolicyIgnored, Message: `Gemini extension policies ignore "allow" at extension tier`},
			},
		}, nil
	})
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"."})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	output := stdout.String()
	for _, want := range []string{
		`Warning: Gemini extension directory basename "tmp-ext" does not match extension name "demo-ext"`,
		"Hint: rename the extension directory to match plugin.yaml name before running gemini extensions link .",
		"Hint: Gemini extension-tier policies ignore allow/yolo; keep only documented extension policy keys in targets/gemini/policies/*.toml.",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("stdout missing %q:\n%s", want, output)
		}
	}
}

func TestValidateWritesGeminiSuccessHintsForRuntimeLane(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "plugin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "plugin", "launcher.yaml"), []byte("runtime: go\nentrypoint: ./bin/demo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := newValidateCmd(func(gotRoot, platform string) (validate.Report, error) {
		if gotRoot != root {
			t.Fatalf("root = %q, want %q", gotRoot, root)
		}
		return validate.Report{Platform: "gemini"}, nil
	})
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{root})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	output := stdout.String()
	for _, want := range []string{
		"Validated " + root,
		"Hint: Gemini Go runtime is validate-clean; run make test-gemini-runtime before relinking the extension.",
		"Hint: relink the extension with gemini extensions link . before checking the runtime path in a real Gemini CLI session.",
		"Hint: use make test-gemini-runtime-live when you need real CLI evidence after the repo-local runtime gate is green.",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("stdout missing %q:\n%s", want, output)
		}
	}
}

func TestValidateWritesJSONOutput(t *testing.T) {
	t.Parallel()
	cmd := newValidateCmd(fakeValidateRunner{
		report: validate.Report{
			Platform: "codex-runtime",
			Checks:   []string{"plugin_manifest"},
			Warnings: []validate.Warning{{
				Kind:    validate.WarningManifestUnknownField,
				Path:    "plugin/plugin.yaml",
				Message: "unknown plugin.yaml field: extra_field",
			}},
		},
	}.Run)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--platform", "codex-runtime", "--format", "json", "."})
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatal(err)
	}
	output := buf.String()
	for _, want := range []string{
		`"format": "plugin-kit-ai/validate-report"`,
		`"schema_version": 1`,
		`"requested_platform": "codex-runtime"`,
		`"outcome": "passed"`,
		`"platform": "codex-runtime"`,
		`"ok": true`,
		`"strict_mode": false`,
		`"strict_failed": false`,
		`"warning_count": 1`,
		`"failure_count": 0`,
		`"checks": [`,
		`"warnings": [`,
		`"failures": []`,
		`"path": "plugin/plugin.yaml"`,
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("json output missing %q:\n%s", want, output)
		}
	}
}

func TestValidateJSONIncludesPublicationSummaryWhenDiscoverable(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "plugin", "targets", "codex-package"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "plugin", "publish", "codex"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "plugin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "plugin", "plugin.yaml"), []byte("api_version: v1\nname: \"demo\"\nversion: \"0.1.0\"\ndescription: \"demo\"\ntargets: [\"codex-package\"]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "plugin", "targets", "codex-package"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "plugin", "targets", "codex-package", "package.yaml"), []byte("homepage: https://example.com/demo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "plugin", "targets", "codex-package", "interface.json"), []byte(`{"defaultPrompt":["Inspect"]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "plugin", "publish", "codex"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "plugin", "publish", "codex", "marketplace.yaml"), []byte("api_version: v1\nmarketplace_name: local-repo\ncategory: Productivity\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := newValidateCmd(func(gotRoot, platform string) (validate.Report, error) {
		if gotRoot != root {
			t.Fatalf("root = %q, want %q", gotRoot, root)
		}
		return validate.Report{
			Platform: "codex-package",
			Checks:   []string{"plugin_manifest", "publication"},
		}, nil
	})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--format", "json", root})
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(buf.Bytes(), &payload); err != nil {
		t.Fatalf("json parse: %v\n%s", err, buf.Bytes())
	}
	publication, ok := payload["publication"].(map[string]any)
	if !ok {
		t.Fatalf("publication payload missing: %+v", payload)
	}
	channels, ok := publication["channels"].([]any)
	if !ok || len(channels) != 1 {
		t.Fatalf("publication.channels = %+v", publication["channels"])
	}
}

func TestValidateJSONPrintsReportOnFailure(t *testing.T) {
	t.Parallel()
	report := validate.Report{
		Checks: []string{},
		Failures: []validate.Failure{{
			Kind:    validate.FailureManifestMissing,
			Path:    "plugin/plugin.yaml",
			Message: "required manifest missing: plugin/plugin.yaml",
		}},
	}
	cmd := newValidateCmd(fakeValidateRunner{
		err: &validate.ReportError{Report: report},
	}.Run)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--format", "json", "."})
	err := cmd.ExecuteContext(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	for _, want := range []string{
		`"kind": "manifest_missing"`,
		`"failures": [`,
		`"format": "plugin-kit-ai/validate-report"`,
		`"schema_version": 1`,
		`"outcome": "failed"`,
		`"ok": false`,
		`"strict_mode": false`,
		`"strict_failed": false`,
		`"warning_count": 0`,
		`"failure_count": 1`,
	} {
		if !strings.Contains(buf.String(), want) {
			t.Fatalf("json output missing %q:\n%s", want, buf.String())
		}
	}
}

func TestValidateJSONMarksStrictWarningFailure(t *testing.T) {
	t.Parallel()
	cmd := newValidateCmd(fakeValidateRunner{
		report: validate.Report{
			Warnings: []validate.Warning{{
				Kind:    validate.WarningManifestUnknownField,
				Message: "unknown plugin.yaml field: extra_field",
			}},
		},
	}.Run)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--strict", "--format", "json", "."})
	err := cmd.ExecuteContext(context.Background())
	if err == nil {
		t.Fatal("expected strict warning error")
	}
	for _, want := range []string{
		`"format": "plugin-kit-ai/validate-report"`,
		`"schema_version": 1`,
		`"outcome": "failed_strict_warnings"`,
		`"ok": false`,
		`"strict_mode": true`,
		`"strict_failed": true`,
		`"warning_count": 1`,
		`"failure_count": 0`,
	} {
		if !strings.Contains(buf.String(), want) {
			t.Fatalf("json output missing %q:\n%s", want, buf.String())
		}
	}
}

func TestValidateTextPrintsFailuresForReportErrors(t *testing.T) {
	t.Parallel()
	report := validate.Report{
		Warnings: []validate.Warning{{
			Kind:    validate.WarningManifestUnknownField,
			Message: "unknown plugin.yaml field: extra_field",
		}},
		Failures: []validate.Failure{{
			Kind:    validate.FailureManifestMissing,
			Path:    "plugin/plugin.yaml",
			Message: "required manifest missing: plugin/plugin.yaml",
		}},
	}
	cmd := newValidateCmd(fakeValidateRunner{
		err: &validate.ReportError{Report: report},
	}.Run)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"."})
	err := cmd.ExecuteContext(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	output := buf.String()
	for _, want := range []string{
		"Warning: unknown plugin.yaml field: extra_field",
		"Failure: required manifest missing: plugin/plugin.yaml",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("text output missing %q:\n%s", want, output)
		}
	}
}

func TestValidateTextPrintsPublicationSummaryWhenDiscoverable(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "plugin", "targets", "gemini", "contexts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "plugin", "publish", "gemini"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "plugin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "plugin", "plugin.yaml"), []byte("api_version: v1\nname: \"gemini-publish\"\nversion: \"0.1.0\"\ndescription: \"gemini publish\"\ntargets: [\"gemini\"]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "plugin", "targets", "gemini", "package.yaml"), []byte("homepage: https://example.com/gemini\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "plugin", "targets", "gemini", "contexts", "GEMINI.md"), []byte("# Gemini\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "plugin", "publish", "gemini", "gallery.yaml"), []byte("api_version: v1\ndistribution: github_release\nrepository_visibility: public\ngithub_topic: gemini-cli-extension\nmanifest_root: release_archive_root\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := newValidateCmd(func(gotRoot, platform string) (validate.Report, error) {
		if gotRoot != root {
			t.Fatalf("root = %q, want %q", gotRoot, root)
		}
		return validate.Report{Platform: "gemini"}, nil
	})
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{root})
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatal(err)
	}
	output := stdout.String()
	for _, want := range []string{
		"Validated " + root,
		"Publication: api_version=v1 packages=1 channels=1",
		"Publication channel: gemini-gallery path=plugin/publish/gemini/gallery.yaml targets=gemini",
		"details=distribution=github_release,github_topic=gemini-cli-extension,manifest_root=release_archive_root,repository_visibility=public",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("stdout missing %q:\n%s", want, output)
		}
	}
}

func TestValidateHelpIncludesCursorTarget(t *testing.T) {
	t.Parallel()
	cmd := newValidateCmd(fakeValidateRunner{}.Run)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--help"})
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), `"cursor"`) {
		t.Fatalf("help output missing cursor target:\n%s", buf.String())
	}
}

func TestValidateHelpMentionsJSONContract(t *testing.T) {
	t.Parallel()
	cmd := newValidateCmd(fakeValidateRunner{}.Run)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--help"})
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatal(err)
	}
	output := buf.String()
	for _, want := range []string{
		`output format ("text" or "json")`,
		`plugin-kit-ai/validate-report`,
		`schema_version=1`,
		`failed_strict_warnings`,
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("help output missing %q:\n%s", want, output)
		}
	}
}
