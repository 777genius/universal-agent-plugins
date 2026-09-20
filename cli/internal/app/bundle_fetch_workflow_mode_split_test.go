package app

import (
	"strings"
	"testing"
)

func TestValidateBundleFetchURLModeRejectsTag(t *testing.T) {
	t.Parallel()

	err := validateBundleFetchURLMode(PluginBundleFetchOptions{
		URL: "https://example.com/demo.tar.gz",
		Tag: "v1.0.0",
	})
	if err == nil || !strings.Contains(err.Error(), "--tag or --latest") {
		t.Fatalf("error = %v", err)
	}
}

func TestValidateBundleFetchGitHubModeRequiresRef(t *testing.T) {
	t.Parallel()

	err := validateBundleFetchGitHubMode(PluginBundleFetchOptions{Latest: true})
	if err == nil || !strings.Contains(err.Error(), "--url or owner/repo") {
		t.Fatalf("error = %v", err)
	}
}
