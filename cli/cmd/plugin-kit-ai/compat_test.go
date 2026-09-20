package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/app"
	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmanifest"
)

type fakeCompatRunner struct {
	report   pluginmanifest.SourceInspection
	warnings []pluginmanifest.Warning
	err      error
}

func (f fakeCompatRunner) Compat(app.PluginCompatOptions) (pluginmanifest.SourceInspection, []pluginmanifest.Warning, error) {
	return f.report, f.warnings, f.err
}

func TestCompatTextShowsPartialCompatibility(t *testing.T) {
	t.Parallel()
	cmd := newCompatCmd(fakeCompatRunner{
		report: pluginmanifest.SourceInspection{
			RequestedSource: "github:acme/demo@v1.2.3//claude-plugin",
			ResolvedSource:  "https://github.com/acme/demo@abc123",
			SourceKind:      "github_repo_path",
			SourceDigest:    "sha256:demo",
			ImportSource:    "claude",
			OriginTargets:   []string{"claude"},
			Inspection: pluginmanifest.Inspection{
				Manifest: pluginmanifest.Manifest{Name: "demo", Version: "1.2.3"},
			},
			Compatibility: []pluginmanifest.SourceCompatibility{
				{Target: "claude", Status: pluginmanifest.CompatibilityFull, SupportedKinds: []string{"skills", "commands"}},
				{Target: "codex-package", Status: pluginmanifest.CompatibilityPartial, SupportedKinds: []string{"skills"}, UnsupportedKinds: []string{"commands"}, Notes: []string{"installable with degraded surface coverage"}},
			},
		},
	})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"github:acme/demo@v1.2.3//claude-plugin"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	output := buf.String()
	for _, want := range []string{
		"source: github:acme/demo@v1.2.3//claude-plugin",
		"mode: imported-native from=claude",
		"- claude: status=full supported=skills,commands unsupported=-",
		"- codex-package: status=partial supported=skills unsupported=commands",
		"note=installable with degraded surface coverage",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("compat output missing %q:\n%s", want, output)
		}
	}
}

func TestCompatJSONIncludesCompatibilityArray(t *testing.T) {
	t.Parallel()
	cmd := newCompatCmd(fakeCompatRunner{
		report: pluginmanifest.SourceInspection{
			RequestedSource:  "./demo",
			ResolvedSource:   "./demo",
			SourceKind:       "local_path",
			SourceDigest:     "sha256:demo",
			CanonicalPackage: true,
			OriginTargets:    []string{"cursor"},
			Compatibility: []pluginmanifest.SourceCompatibility{
				{Target: "cursor", Status: pluginmanifest.CompatibilityFull, SupportedKinds: []string{"skills"}},
			},
		},
	})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"./demo", "--format", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var report map[string]any
	if err := json.Unmarshal(buf.Bytes(), &report); err != nil {
		t.Fatalf("compat json parse: %v\n%s", err, buf.Bytes())
	}
	compat, ok := report["compatibility"].([]any)
	if !ok || len(compat) != 1 {
		t.Fatalf("compatibility payload missing: %+v", report)
	}
	first, ok := compat[0].(map[string]any)
	if !ok || first["status"] != string(pluginmanifest.CompatibilityFull) {
		t.Fatalf("compatibility[0] malformed: %+v", compat[0])
	}
}
