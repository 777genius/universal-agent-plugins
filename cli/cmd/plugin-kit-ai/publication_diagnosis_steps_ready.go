package main

import "github.com/777genius/plugin-kit-ai/cli/internal/publicationmodel"

func buildReadyPublicationNextSteps(model publicationmodel.Model) []string {
	steps := []string{
		"run plugin-kit-ai validate . --strict",
		"run plugin-kit-ai publication . --format json for CI or automation handoff",
	}
	for _, channel := range model.Channels {
		steps = append(steps, readyPublicationChannelSteps(channel)...)
	}
	return appendUniqueStrings(nil, steps...)
}

func readyPublicationChannelSteps(channel publicationmodel.Channel) []string {
	switch channel.Family {
	case "codex-marketplace":
		return []string{"run plugin-kit-ai publication materialize . --target codex-package --dest <marketplace-root> --dry-run to preview the local Codex marketplace root"}
	case "claude-marketplace":
		return []string{"run plugin-kit-ai publication materialize . --target claude --dest <marketplace-root> --dry-run to preview the local Claude marketplace root"}
	case "gemini-gallery":
		return geminiReadyPublicationChannelSteps(channel)
	default:
		return nil
	}
}

func geminiReadyPublicationChannelSteps(channel publicationmodel.Channel) []string {
	steps := []string{
		"confirm the GitHub repository stays public and tagged with the gemini-cli-extension topic",
		"use gemini extensions link <path> for live Gemini CLI verification before publishing",
	}
	switch channel.Details["distribution"] {
	case "github_release":
		steps = append(steps[:1], append([]string{"ensure GitHub release archives keep gemini-extension.json at the archive root"}, steps[1:]...)...)
	default:
		steps = append(steps[:1], append([]string{"keep gemini-extension.json at the repository root for git-based installs and gallery indexing"}, steps[1:]...)...)
	}
	return steps
}

func findPublicationChannelByFamily(channels []publicationmodel.Channel, family string) (publicationmodel.Channel, bool) {
	for _, channel := range channels {
		if channel.Family == family {
			return channel, true
		}
	}
	return publicationmodel.Channel{}, false
}
