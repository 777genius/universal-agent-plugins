package main

import (
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/publicationmodel"
)

func TestBuildPublicationDiagnosisLinesIncludesChannelDetailsAndPackageSummary(t *testing.T) {
	t.Parallel()

	lines, channelTargets := buildPublicationDiagnosisLines(publicationmodel.Model{
		Core: publicationmodel.Core{
			APIVersion: "v1",
			Name:       "demo",
			Version:    "0.1.0",
		},
		Packages: []publicationmodel.Package{{
			Target:           "gemini",
			PackageFamily:    "gemini-extension",
			ChannelFamilies:  []string{"gemini-gallery"},
			ManagedArtifacts: []string{"gemini-extension.json"},
		}},
		Channels: []publicationmodel.Channel{{
			Family:         "gemini-gallery",
			Path:           "plugin/publish/gemini/gallery.yaml",
			PackageTargets: []string{"gemini"},
			Details: map[string]string{
				"distribution": "github_release",
			},
		}},
	})

	joined := strings.Join(lines, "\n")
	for _, want := range []string{
		"Publication: demo 0.1.0 api_version=v1",
		"Channel[gemini-gallery]: path=plugin/publish/gemini/gallery.yaml targets=gemini details=distribution=github_release",
		"Package[gemini]: family=gemini-extension channels=gemini-gallery managed=1",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("lines missing %q:\n%s", want, joined)
		}
	}
	if _, ok := channelTargets["gemini"]; !ok {
		t.Fatalf("channel targets = %+v", channelTargets)
	}
}

func TestMissingPublicationChannelPackagesReturnsOnlyUnmatchedTargets(t *testing.T) {
	t.Parallel()

	missing := missingPublicationChannelPackages([]publicationmodel.Package{
		{Target: "claude"},
		{Target: "gemini"},
	}, map[string]struct{}{"claude": {}})

	if len(missing) != 1 || missing[0].Target != "gemini" {
		t.Fatalf("missing packages = %+v", missing)
	}
}

func TestFinalizePublicationDiagnosisPrefersRepositoryIssuesOverArtifactIssues(t *testing.T) {
	t.Parallel()

	got := finalizePublicationDiagnosis(
		[]string{"Publication: demo 0.1.0 api_version=v1"},
		"plugin",
		publicationmodel.Model{},
		nil,
		[]publicationIssue{{Code: "artifact_issue", Message: "artifact missing"}},
		[]publicationIssue{{Code: "repo_issue", Message: "repo missing"}},
		[]string{"fix repository"},
	)

	if got.Status != "needs_repository" || got.Ready {
		t.Fatalf("diagnosis = %+v", got)
	}
	if len(got.Issues) != 1 || got.Issues[0].Code != "repo_issue" {
		t.Fatalf("issues = %+v", got.Issues)
	}
}
