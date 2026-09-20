package main

import (
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/publicationmodel"
	"github.com/777genius/plugin-kit-ai/cli/internal/repostate"
)

func TestDiagnoseGeminiRepositoryStateReportsGitUnavailable(t *testing.T) {
	t.Parallel()

	issues, next := diagnoseGeminiRepositoryState(publicationmodel.Channel{
		Family: "gemini-gallery",
		Path:   "plugin/publish/gemini/gallery.yaml",
	}, repostate.State{})

	if len(issues) != 1 || issues[0].Code != "gemini_git_cli_unavailable" {
		t.Fatalf("issues = %+v", issues)
	}
	if len(next) != 1 || next[0] != "install git and rerun plugin-kit-ai publication doctor . --target gemini" {
		t.Fatalf("next = %+v", next)
	}
}

func TestGeminiRepositoryNextStepsAddsDistributionGuidance(t *testing.T) {
	t.Parallel()

	next := geminiRepositoryNextSteps(publicationmodel.Channel{
		Family: "gemini-gallery",
		Path:   "plugin/publish/gemini/gallery.yaml",
		Details: map[string]string{
			"distribution": "github_release",
		},
	}, repostate.State{
		GitAvailable:    true,
		InGitRepo:       true,
		HasOriginRemote: true,
		OriginIsGitHub:  false,
	})

	if len(next) < 2 {
		t.Fatalf("next = %+v", next)
	}
	if next[0] != "move the publication remote to a public GitHub repository before publishing to the Gemini gallery" {
		t.Fatalf("next = %+v", next)
	}
	if next[len(next)-1] != "prepare a public GitHub repository first, then publish release archives that keep gemini-extension.json at the archive root" {
		t.Fatalf("next = %+v", next)
	}
}
