package app

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/publicationmodel"
)

func buildGeminiPublishResult(opts PluginPublishOptions, channel publicationmodel.Channel, status string, issues []PluginPublishIssue, nextSteps []string) PluginPublishResult {
	return PluginPublishResult{
		Channel:       "gemini-gallery",
		Target:        "gemini",
		Ready:         status == "ready",
		Status:        status,
		Mode:          publicationModeLabel(true),
		WorkflowClass: "repository_release_plan",
		Details: map[string]string{
			"distribution":          channel.Details["distribution"],
			"manifest_root":         channel.Details["manifest_root"],
			"repository_visibility": channel.Details["repository_visibility"],
			"github_topic":          channel.Details["github_topic"],
			"publication_model":     "repository_or_release_rooted",
		},
		IssueCount: len(issues),
		Issues:     issues,
		NextSteps:  nextSteps,
		Lines:      buildGeminiPublishLines(opts, channel, status, issues, nextSteps),
	}
}

func buildGeminiPublishLines(opts PluginPublishOptions, channel publicationmodel.Channel, status string, issues []PluginPublishIssue, nextSteps []string) []string {
	lines := []string{
		"Publish channel: gemini-gallery",
		"Publish target: gemini",
		fmt.Sprintf("Mode: %s", publicationModeLabel(true)),
		fmt.Sprintf("Channel manifest: %s", channel.Path),
		fmt.Sprintf("Distribution: %s", channel.Details["distribution"]),
		fmt.Sprintf("Manifest root: %s", channel.Details["manifest_root"]),
		fmt.Sprintf("Repository visibility: %s", channel.Details["repository_visibility"]),
		fmt.Sprintf("GitHub topic: %s", channel.Details["github_topic"]),
		"Publication model: repository/release rooted (no local marketplace root is materialized)",
	}
	if dest := strings.TrimSpace(opts.Dest); dest != "" {
		lines = append(lines, fmt.Sprintf("Destination root ignored: %s", filepath.Clean(dest)))
	}
	for _, issue := range issues {
		lines = append(lines, fmt.Sprintf("Issue[%s]: %s", issue.Code, issue.Message))
	}
	lines = append(lines, geminiPublishStatusLine(status), "Next:")
	for _, step := range nextSteps {
		lines = append(lines, "  "+step)
	}
	return lines
}

func geminiPublishStatusLine(status string) string {
	if status == "ready" {
		return "Status: ready (repository or release publication plan is consistent with the current workspace)"
	}
	return "Status: needs_repository (repository context is not yet ready for Gemini gallery publication)"
}
