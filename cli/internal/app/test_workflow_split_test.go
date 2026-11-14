package app

import (
	"slices"
	"testing"
)

func TestPluginTestGoldenHints(t *testing.T) {
	t.Parallel()

	got := pluginTestGoldenHints(true)
	want := []string{
		"Tip: rerun with --update-golden to capture the current stdout/stderr/exit contract.",
		"CI hint: once goldens are committed, `plugin-kit-ai test --format json` provides machine-readable case and summary output.",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("pluginTestGoldenHints(true) = %v", got)
	}
	if got := pluginTestGoldenHints(false); got != nil {
		t.Fatalf("pluginTestGoldenHints(false) = %v", got)
	}
}

func TestAppendPluginTestCaseUpdatesSummary(t *testing.T) {
	t.Parallel()

	result := PluginTestResult{
		Passed: true,
		Summary: PluginTestSummary{
			Total: 1,
		},
	}
	notConfigured := appendPluginTestCase(&result, PluginTestCase{
		Event:        "Stop",
		Passed:       false,
		GoldenStatus: "not_configured",
		Failure:      "goldens missing",
	})
	if !notConfigured {
		t.Fatal("expected not_configured flag")
	}
	if result.Passed {
		t.Fatalf("result.Passed = %v", result.Passed)
	}
	if result.Summary.Failed != 1 || result.Summary.GoldenNotConfigured != 1 {
		t.Fatalf("summary = %+v", result.Summary)
	}
}
