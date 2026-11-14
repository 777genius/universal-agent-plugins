package app

import (
	"context"
	"testing"

	"github.com/777genius/plugin-kit-ai/plugininstall/domain"
)

func TestLoadBundleGitHubReleaseUsesLatest(t *testing.T) {
	t.Parallel()

	rel := &domain.Release{TagName: "v9.0.0"}
	got, err := loadBundleGitHubRelease(context.Background(), fakeBundleReleaseSource{latest: rel}, "demo", "repo", PluginBundleFetchOptions{Latest: true})
	if err != nil {
		t.Fatalf("loadBundleGitHubRelease: %v", err)
	}
	if got != rel {
		t.Fatalf("release = %#v", got)
	}
}

func TestBuildBundleGitHubSourceLabelFallsBackToRequestedTag(t *testing.T) {
	t.Parallel()

	label := buildBundleGitHubSourceLabel("demo", "repo", &domain.Release{}, PluginBundleFetchOptions{Tag: "v1.2.3"}, "demo_bundle.tar.gz")
	if label != "github release demo/repo@v1.2.3 (tag) asset=demo_bundle.tar.gz" {
		t.Fatalf("label = %q", label)
	}
}
