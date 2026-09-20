package app

import (
	"fmt"
	"strconv"
	"strings"
)

func formatRuntimeTestCaseLine(tc PluginTestCase) string {
	status := "PASS"
	if !tc.Passed {
		status = "FAIL"
	}
	line := fmt.Sprintf("%s %s/%s", status, tc.Platform, tc.Event)
	if tc.FixturePath != "" {
		line += " fixture=" + tc.FixturePath
	}
	if tc.Failure != "" {
		line += " reason=" + tc.Failure
		return line
	}
	line += fmt.Sprintf(" exit=%d", tc.ExitCode)
	if tc.GoldenStatus != "" {
		line += " golden=" + tc.GoldenStatus
	}
	if len(tc.Mismatches) > 0 {
		line += " mismatches=" + strings.Join(tc.Mismatches, ",")
	}
	return line
}

func formatRuntimeTestCaseDetails(tc PluginTestCase) []string {
	var lines []string
	if tc.GoldenStatus == "not_configured" {
		lines = append(lines, "  goldens: not configured")
	}
	for _, mismatch := range tc.MismatchInfo {
		label := mismatch.Field
		if mismatch.GoldenFile != "" {
			label += " (" + mismatch.GoldenFile + ")"
		}
		lines = append(lines, fmt.Sprintf("  %s expected=%s actual=%s", label, mismatch.ExpectedPreview, mismatch.ActualPreview))
	}
	return lines
}

func formatRuntimeTestSummary(summary PluginTestSummary) string {
	return fmt.Sprintf(
		"Summary: total=%d passed=%d failed=%d golden_matched=%d golden_updated=%d golden_not_configured=%d golden_mismatch=%d",
		summary.Total,
		summary.Passed,
		summary.Failed,
		summary.GoldenMatched,
		summary.GoldenUpdated,
		summary.GoldenNotConfigured,
		summary.GoldenMismatch,
	)
}

func runtimeTestPreview(text string) string {
	switch {
	case text == "":
		return `"<empty>"`
	default:
		text = strings.ReplaceAll(text, "\n", `\n`)
		if len(text) > 120 {
			text = text[:120] + "..."
		}
		return strconv.Quote(text)
	}
}
