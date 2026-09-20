package app

import "fmt"

func appendPublicationVerifyIssueLines(lines []string, issues []PluginPublicationRootIssue) []string {
	for _, issue := range issues {
		lines = append(lines, fmt.Sprintf("Issue[%s]: %s", issue.Code, issue.Message))
	}
	return lines
}

func appendPublicationVerifyReadyLines(lines []string) []string {
	return append(lines, "Status: ready (materialized marketplace root is in sync)")
}

func appendPublicationVerifyNeedsSyncLines(lines, nextSteps []string) []string {
	lines = append(lines, "Status: needs_sync (materialized marketplace root is missing files or has drift)", "Next:")
	for _, step := range nextSteps {
		lines = append(lines, "  "+step)
	}
	return lines
}
