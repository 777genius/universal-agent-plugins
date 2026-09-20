package main

import (
	"fmt"

	"github.com/777genius/plugin-kit-ai/cli/internal/publicationmodel"
	"github.com/777genius/plugin-kit-ai/cli/internal/repostate"
)

func diagnoseGeminiRepositoryIssues(root string, model publicationmodel.Model) ([]publicationIssue, []string) {
	channel, ok := expectedGeminiPublicationChannel(model)
	if !ok {
		return nil, nil
	}
	return diagnoseGeminiRepositoryState(channel, repostate.Inspect(root))
}

func diagnoseGeminiRepositoryState(channel publicationmodel.Channel, state repostate.State) ([]publicationIssue, []string) {
	var issues []publicationIssue
	if !state.GitAvailable {
		return []publicationIssue{geminiRepositoryIssue(channel, "gemini_git_cli_unavailable", "git is unavailable, so repository-rooted Gemini gallery prerequisites cannot be verified")},
			[]string{"install git and rerun plugin-kit-ai publication doctor . --target gemini"}
	}
	issues = append(issues, geminiRepositoryAvailabilityIssues(channel, state)...)
	if len(issues) == 0 {
		return nil, nil
	}
	return issues, geminiRepositoryNextSteps(channel, state)
}

func geminiRepositoryAvailabilityIssues(channel publicationmodel.Channel, state repostate.State) []publicationIssue {
	var issues []publicationIssue
	if !state.InGitRepo {
		issues = append(issues, geminiRepositoryIssue(channel, "gemini_git_repository_missing", "Gemini gallery publication expects a Git repository, but the current workspace is not inside one"))
	}
	if !state.HasOriginRemote {
		issues = append(issues, geminiRepositoryIssue(channel, "gemini_origin_remote_missing", "Gemini gallery publication expects a GitHub-backed repository or release source, but no origin remote is configured"))
	} else if !state.OriginIsGitHub {
		issues = append(issues, geminiRepositoryIssue(channel, "gemini_origin_not_github", fmt.Sprintf("Gemini gallery publication expects GitHub distribution metadata, but origin points to %s", state.OriginHost)))
	}
	return issues
}

func geminiRepositoryNextSteps(channel publicationmodel.Channel, state repostate.State) []string {
	var next []string
	if !state.InGitRepo {
		next = append(next, "initialize a Git repository for this plugin before publishing to the Gemini gallery")
	}
	if !state.HasOriginRemote {
		next = append(next, "add a GitHub origin remote for this plugin repository before publishing")
	} else if !state.OriginIsGitHub {
		next = append(next, "move the publication remote to a public GitHub repository before publishing to the Gemini gallery")
	}
	next = append(next, "confirm the GitHub repository stays public and tagged with the gemini-cli-extension topic")
	switch channel.Details["distribution"] {
	case "github_release":
		next = append(next, "prepare a public GitHub repository first, then publish release archives that keep gemini-extension.json at the archive root")
	default:
		next = append(next, "keep gemini-extension.json at the repository root once the GitHub repository is ready")
	}
	return appendUniqueStrings(nil, next...)
}

func geminiRepositoryIssue(channel publicationmodel.Channel, code, message string) publicationIssue {
	return publicationIssue{
		Code:          code,
		Target:        "gemini",
		ChannelFamily: "gemini-gallery",
		Path:          channel.Path,
		Message:       message,
	}
}
