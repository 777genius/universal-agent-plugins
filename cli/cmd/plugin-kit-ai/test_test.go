package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/app"
	"github.com/777genius/plugin-kit-ai/cli/internal/exitx"
)

type fakeTestRunner struct {
	result app.PluginTestResult
	err    error
}

func (f fakeTestRunner) Test(context.Context, app.PluginTestOptions) (app.PluginTestResult, error) {
	return f.result, f.err
}

func TestTestHelpIncludesFixtureAndGoldenLanguage(t *testing.T) {
	t.Parallel()
	cmd := newTestCmd(fakeTestRunner{})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	output := buf.String()
	for _, want := range []string{"fixture", "golden", "stdout/stderr/exitcode", "Gemini", "production-ready 9-hook Go runtime", "make test-gemini-runtime", "make test-gemini-runtime-live"} {
		if !strings.Contains(output, want) {
			t.Fatalf("help output missing %q:\n%s", want, output)
		}
	}
}

func TestTestWritesJSONOutput(t *testing.T) {
	t.Parallel()
	cmd := newTestCmd(fakeTestRunner{
		result: app.PluginTestResult{
			Passed: true,
			Summary: app.PluginTestSummary{
				Total:         1,
				Passed:        1,
				GoldenMatched: 1,
			},
			Cases: []app.PluginTestCase{{
				Platform:     "claude",
				Event:        "Stop",
				GoldenStatus: "matched",
				Passed:       true,
			}},
		},
	})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--platform", "claude", "--event", "Stop", "--format", "json", "."})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	output := buf.String()
	for _, want := range []string{`"passed": true`, `"event": "Stop"`, `"golden_status": "matched"`, `"summary": {`, `"golden_matched": 1`} {
		if !strings.Contains(output, want) {
			t.Fatalf("json output missing %q:\n%s", want, output)
		}
	}
}

func TestTestReturnsExitCodeOneWhenCasesFail(t *testing.T) {
	t.Parallel()
	cmd := newTestCmd(fakeTestRunner{
		result: app.PluginTestResult{
			Passed: false,
			Lines:  []string{"FAIL claude/Stop fixture=/tmp/fixtures/claude/Stop.json golden=mismatch mismatches=stdout"},
		},
	})
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"--platform", "claude", "--event", "Stop", "."})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
	if code := exitx.Code(err); code != 1 {
		t.Fatalf("exit code = %d", code)
	}
	if !strings.Contains(buf.String(), "FAIL claude/Stop") {
		t.Fatalf("output = %s", buf.String())
	}
}
