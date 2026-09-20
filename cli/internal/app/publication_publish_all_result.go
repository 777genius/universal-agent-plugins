package app

import (
	"fmt"

	"github.com/777genius/plugin-kit-ai/cli/internal/publicationmodel"
)

type publishAllPlan struct {
	results  []PluginPublishResult
	warnings []string
	next     []string
	ready    bool
}

func emptyPublishPlan(root string) PluginPublishResult {
	next := []string{
		"author at least one publication channel under publish/...",
		fmt.Sprintf("run plugin-kit-ai publication doctor %s", root),
	}
	lines := []string{
		"Publish selection: all authored channels",
		fmt.Sprintf("Mode: %s", publicationModeLabel(true)),
		"Channel count: 0",
		"Status: needs_channels (no authored publication channels exist under publish/...)",
		"Next:",
	}
	for _, step := range next {
		lines = append(lines, "  "+step)
	}
	return PluginPublishResult{
		Ready:         false,
		Status:        "needs_channels",
		Mode:          publicationModeLabel(true),
		WorkflowClass: "multi_channel_plan",
		Details:       map[string]string{},
		IssueCount:    0,
		Issues:        []PluginPublishIssue{},
		WarningCount:  0,
		Warnings:      []string{},
		NextSteps:     next,
		ChannelCount:  0,
		Channels:      []PluginPublishResult{},
		Lines:         lines,
	}
}

func buildPublishPlanResult(opts PluginPublishOptions, channels []publicationmodel.Channel, plan publishAllPlan) PluginPublishResult {
	status := "ready"
	if !plan.ready {
		status = "needs_attention"
	}
	return PluginPublishResult{
		Ready:         plan.ready,
		Status:        status,
		Mode:          publicationModeLabel(true),
		WorkflowClass: "multi_channel_plan",
		Dest:          cleanedDestForMulti(opts.Dest, channels),
		Details:       map[string]string{},
		IssueCount:    0,
		Issues:        []PluginPublishIssue{},
		WarningCount:  len(plan.warnings),
		Warnings:      cloneStrings(plan.warnings),
		NextSteps:     plan.next,
		ChannelCount:  len(plan.results),
		Channels:      clonePublishResults(plan.results),
		Lines:         publishPlanLines(plan.results, plan.warnings, plan.next, plan.ready, opts.Dest, channels),
	}
}
