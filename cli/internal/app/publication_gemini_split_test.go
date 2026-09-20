package app

import (
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/publicationmodel"
)

func TestGeminiPublishPlanStepsUsesReleaseArchiveForGitHubRelease(t *testing.T) {
	t.Parallel()

	steps := geminiPublishPlanSteps(".", publicationmodel.Channel{
		Details: map[string]string{"distribution": "github_release"},
	})
	joined := strings.Join(steps, "\n")
	if !strings.Contains(joined, "release archive") {
		t.Fatalf("steps = %v", steps)
	}
}

func TestGeminiPublishStatusLineReflectsRepositoryReadiness(t *testing.T) {
	t.Parallel()

	if !strings.Contains(geminiPublishStatusLine("ready"), "Status: ready") {
		t.Fatal("expected ready status line")
	}
	if !strings.Contains(geminiPublishStatusLine("needs_repository"), "Status: needs_repository") {
		t.Fatal("expected needs_repository status line")
	}
}
