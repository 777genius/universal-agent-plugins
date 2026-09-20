package app

import "fmt"

func buildLocalPublishResult(channel string, result PluginPublicationMaterializeResult) PluginPublishResult {
	lines := []string{fmt.Sprintf("Publish channel: %s", channel)}
	lines = append(lines, result.Lines...)
	return PluginPublishResult{
		Channel:       channel,
		Target:        result.Target,
		Ready:         true,
		Status:        "ready",
		Mode:          result.Mode,
		WorkflowClass: "local_marketplace_root",
		Dest:          result.Dest,
		PackageRoot:   result.PackageRoot,
		Details:       cloneStringMap(result.Details),
		IssueCount:    0,
		Issues:        []PluginPublishIssue{},
		WarningCount:  0,
		Warnings:      []string{},
		NextSteps:     cloneStrings(result.NextSteps),
		Lines:         lines,
	}
}
