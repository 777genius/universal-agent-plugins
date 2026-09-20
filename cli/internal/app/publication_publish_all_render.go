package app

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/publicationmodel"
)

func ignoredPublishPlanWarnings(opts PluginPublishOptions) []string {
	var warnings []string
	if dest := strings.TrimSpace(opts.Dest); dest != "" {
		warnings = append(warnings, fmt.Sprintf("destination root %s is ignored because the authored publication channels are repository/release rooted", filepath.Clean(dest)))
	}
	if pkg := strings.TrimSpace(opts.PackageRoot); pkg != "" {
		warnings = append(warnings, fmt.Sprintf("package root %s is ignored because the authored publication channels are repository/release rooted", filepath.Clean(pkg)))
	}
	return warnings
}

func publishPlanLines(results []PluginPublishResult, warnings []string, next []string, ready bool, dest string, channels []publicationmodel.Channel) []string {
	lines := []string{
		"Publish selection: all authored channels",
		fmt.Sprintf("Mode: %s", publicationModeLabel(true)),
		fmt.Sprintf("Channel count: %d", len(results)),
	}
	channelNames := make([]string, 0, len(results))
	for _, result := range results {
		channelNames = append(channelNames, result.Channel)
	}
	lines = append(lines, fmt.Sprintf("Authored channels: %s", strings.Join(channelNames, ", ")))
	if cleaned := strings.TrimSpace(dest); cleaned != "" && channelsNeedLocalDest(channels) {
		lines = append(lines, fmt.Sprintf("Destination root: %s", filepath.Clean(cleaned)))
	}
	for _, warning := range warnings {
		lines = append(lines, "Warning: "+warning)
	}
	lines = append(lines, publishPlanChannelLines(results)...)
	if ready {
		lines = append(lines, "Status: ready (every authored publication channel is ready for its bounded dry-run workflow)")
	} else {
		lines = append(lines, "Status: needs_attention (one or more authored publication channels still need follow-up)")
	}
	if len(next) > 0 {
		lines = append(lines, "Next:")
		for _, step := range next {
			lines = append(lines, "  "+step)
		}
	}
	return lines
}

func publishPlanChannelLines(results []PluginPublishResult) []string {
	lines := make([]string, 0, len(results)*6)
	for i, result := range results {
		lines = append(lines, fmt.Sprintf("Channel %d/%d: %s", i+1, len(results), result.Channel))
		lines = append(lines, fmt.Sprintf("  Status: %s", result.Status))
		lines = append(lines, fmt.Sprintf("  Workflow: %s", result.WorkflowClass))
		if result.Target != "" {
			lines = append(lines, fmt.Sprintf("  Target: %s", result.Target))
		}
		if result.PackageRoot != "" {
			lines = append(lines, fmt.Sprintf("  Package root: %s", result.PackageRoot))
		}
		for _, issue := range result.Issues {
			lines = append(lines, fmt.Sprintf("  Issue[%s]: %s", issue.Code, issue.Message))
		}
		for _, warning := range result.Warnings {
			lines = append(lines, "  Warning: "+warning)
		}
	}
	return lines
}
