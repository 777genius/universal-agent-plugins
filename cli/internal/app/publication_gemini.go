package app

import (
	"fmt"
	"strings"
)

func (service PluginService) publishGeminiGallery(opts PluginPublishOptions) (PluginPublishResult, error) {
	if !opts.DryRun {
		return PluginPublishResult{}, fmt.Errorf("publish channel %q currently supports only --dry-run planning; Gemini publication is repository/release rooted, not local-catalog rooted", "gemini-gallery")
	}
	root := strings.TrimSpace(opts.Root)
	if root == "" {
		root = "."
	}
	channel, err := inspectGeminiPublicationChannel(root)
	if err != nil {
		return PluginPublishResult{}, err
	}
	status, issues, nextSteps := diagnoseGeminiPublishEnvironment(root, channel)
	return buildGeminiPublishResult(opts, channel, status, issues, nextSteps), nil
}
