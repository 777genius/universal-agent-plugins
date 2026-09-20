package main

import (
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/publicationmodel"
)

func TestBuildMissingPublicationNextStepsDedupesAndSorts(t *testing.T) {
	t.Parallel()

	got := buildMissingPublicationNextSteps("", []publicationmodel.Package{
		{Target: "gemini"},
		{Target: "claude"},
		{Target: "gemini"},
	})

	want := []string{
		"add plugin/publish/claude/marketplace.yaml, then rerun plugin-kit-ai generate . and plugin-kit-ai validate . --strict",
		"add plugin/publish/gemini/gallery.yaml, keep gemini-extension.json in the repository or release root, then rerun plugin-kit-ai validate . --strict",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("steps = %+v", got)
	}
}

func TestGeminiReadyPublicationChannelStepsProjectDistributionHint(t *testing.T) {
	t.Parallel()

	got := geminiReadyPublicationChannelSteps(publicationmodel.Channel{
		Family: "gemini-gallery",
		Details: map[string]string{
			"distribution": "github_release",
		},
	})

	want := []string{
		"confirm the GitHub repository stays public and tagged with the gemini-cli-extension topic",
		"ensure GitHub release archives keep gemini-extension.json at the archive root",
		"use gemini extensions link <path> for live Gemini CLI verification before publishing",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("steps = %+v", got)
	}
}

func TestPublicationChannelForTargetDefaultsToPluginRoot(t *testing.T) {
	t.Parallel()

	family, path := publicationChannelForTarget("", "codex-package")
	if family != "codex-marketplace" || path != "plugin/publish/codex/marketplace.yaml" {
		t.Fatalf("family=%q path=%q", family, path)
	}
}
