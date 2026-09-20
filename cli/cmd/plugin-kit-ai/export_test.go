package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/app"
)

type fakeExportRunner struct {
	result app.PluginExportResult
	err    error
}

func (f fakeExportRunner) Export(app.PluginExportOptions) (app.PluginExportResult, error) {
	return f.result, f.err
}

func TestExportHelpIncludesPortableBundleLanguage(t *testing.T) {
	t.Parallel()
	cmd := newExportCmd(fakeExportRunner{})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	output := buf.String()
	for _, want := range []string{"portable", ".tar.gz", "python, node, and shell"} {
		if !strings.Contains(output, want) {
			t.Fatalf("help output missing %q:\n%s", want, output)
		}
	}
}

func TestExportWritesRunnerOutput(t *testing.T) {
	t.Parallel()
	cmd := newExportCmd(fakeExportRunner{
		result: app.PluginExportResult{
			Lines: []string{
				"Project: lane=codex-runtime runtime=python manager=requirements.txt (pip)",
				"Exported bundle: /tmp/demo.tar.gz",
			},
		},
	})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--platform", "codex-runtime", "."})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	output := buf.String()
	if !strings.Contains(output, "Exported bundle: /tmp/demo.tar.gz") {
		t.Fatalf("output = %s", output)
	}
}
