package app

import (
	"fmt"

	"github.com/777genius/plugin-kit-ai/cli/internal/publicationmodel"
	"github.com/777genius/plugin-kit-ai/cli/internal/repostate"
)

func geminiPublishPlanSteps(root string, channel publicationmodel.Channel) []string {
	steps := []string{fmt.Sprintf("run plugin-kit-ai publication doctor %s --target gemini", root)}
	switch channel.Details["distribution"] {
	case "github_release":
		steps = append(steps, "build a release archive that keeps gemini-extension.json at the archive root")
	default:
		steps = append(steps, "keep gemini-extension.json at the repository root for git-based installs and gallery indexing")
	}
	steps = append(steps, "use gemini extensions link <path> for live Gemini CLI verification before publishing")
	return steps
}

func diagnoseGeminiPublishEnvironment(root string, channel publicationmodel.Channel) (string, []PluginPublishIssue, []string) {
	repoIssues, repoSteps := diagnoseGeminiRepositoryContext(root, channel)
	issues := make([]PluginPublishIssue, 0, len(repoIssues))
	for _, issue := range repoIssues {
		issues = append(issues, PluginPublishIssue{Code: issue.Code, Message: issue.Message})
	}
	steps := append([]string{}, repoSteps...)
	steps = append(steps, geminiPublishPlanSteps(root, channel)...)
	status := "ready"
	if len(issues) > 0 {
		status = "needs_repository"
	}
	return status, issues, appendUniquePublishSteps(steps)
}

func diagnoseGeminiRepositoryContext(root string, channel publicationmodel.Channel) ([]PluginPublishIssue, []string) {
	state := repostate.Inspect(root)
	var issues []PluginPublishIssue
	var next []string
	if !state.GitAvailable {
		issues = append(issues, PluginPublishIssue{
			Code:    "gemini_git_cli_unavailable",
			Message: "git is unavailable, so repository-rooted Gemini gallery prerequisites cannot be verified",
		})
		next = append(next, "install git and rerun plugin-kit-ai publish --channel gemini-gallery --dry-run")
		return issues, next
	}
	if !state.InGitRepo {
		issues = append(issues, PluginPublishIssue{
			Code:    "gemini_git_repository_missing",
			Message: "Gemini gallery publication expects a Git repository, but the current workspace is not inside one",
		})
		next = append(next, "initialize a Git repository for this plugin before publishing to the Gemini gallery")
	}
	if !state.HasOriginRemote {
		issues = append(issues, PluginPublishIssue{
			Code:    "gemini_origin_remote_missing",
			Message: "Gemini gallery publication expects a GitHub-backed repository or release source, but no origin remote is configured",
		})
		next = append(next, "add a GitHub origin remote for this plugin repository before publishing")
	} else if !state.OriginIsGitHub {
		issues = append(issues, PluginPublishIssue{
			Code:    "gemini_origin_not_github",
			Message: fmt.Sprintf("Gemini gallery publication expects GitHub distribution metadata, but origin points to %s", state.OriginHost),
		})
		next = append(next, "move the publication remote to a public GitHub repository before publishing to the Gemini gallery")
	}
	if len(issues) == 0 {
		next = append(next, "confirm the GitHub repository stays public and tagged with the gemini-cli-extension topic")
	} else if channel.Details["distribution"] == "github_release" {
		next = append(next, "prepare a public GitHub repository first, then publish release archives from that repository")
	}
	return issues, next
}
