package main

import "github.com/777genius/plugin-kit-ai/cli/internal/app"

func normalizedPublicationLocalRoot(localRoot *app.PluginPublicationVerifyRootResult) *app.PluginPublicationVerifyRootResult {
	if localRoot == nil {
		return nil
	}
	clone := *localRoot
	if clone.Issues == nil {
		clone.Issues = []app.PluginPublicationRootIssue{}
	}
	if clone.NextSteps == nil {
		clone.NextSteps = []string{}
	}
	return &clone
}
