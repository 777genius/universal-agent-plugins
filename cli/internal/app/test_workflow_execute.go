package app

import "context"

func executePluginTestRun(ctx context.Context, run pluginTestRun, opts PluginTestOptions) PluginTestResult {
	result := PluginTestResult{
		Passed: true,
		Summary: PluginTestSummary{
			Total: len(run.selected),
		},
		Lines: append([]string(nil), run.baseLines...),
		Cases: make([]PluginTestCase, 0, len(run.selected)),
	}

	anyNotConfigured := false
	for _, item := range run.selected {
		tc := runRuntimeTestCase(ctx, run.root, run.project, opts, item)
		anyNotConfigured = appendPluginTestCase(&result, tc) || anyNotConfigured
	}
	result.Lines = append(result.Lines, formatRuntimeTestSummary(result.Summary))
	result.Lines = append(result.Lines, pluginTestGoldenHints(anyNotConfigured)...)
	return result
}

func appendPluginTestCase(result *PluginTestResult, tc PluginTestCase) bool {
	result.Cases = append(result.Cases, tc)
	if !tc.Passed {
		result.Passed = false
		result.Summary.Failed++
	} else {
		result.Summary.Passed++
	}
	switch tc.GoldenStatus {
	case "matched":
		result.Summary.GoldenMatched++
	case "updated":
		result.Summary.GoldenUpdated++
	case "not_configured":
		result.Summary.GoldenNotConfigured++
	case "mismatch":
		result.Summary.GoldenMismatch++
	}
	result.Lines = append(result.Lines, formatRuntimeTestCaseLine(tc))
	result.Lines = append(result.Lines, formatRuntimeTestCaseDetails(tc)...)
	return tc.GoldenStatus == "not_configured"
}

func pluginTestGoldenHints(anyNotConfigured bool) []string {
	if !anyNotConfigured {
		return nil
	}
	return []string{
		"Tip: rerun with --update-golden to capture the current stdout/stderr/exit contract.",
		"CI hint: once goldens are committed, `plugin-kit-ai test --format json` provides machine-readable case and summary output.",
	}
}
