package app

import (
	"strings"
	"testing"
)

func TestBundleFetchUsesURLSourceDetectsURLMode(t *testing.T) {
	t.Parallel()

	if !bundleFetchUsesURLSource(PluginBundleFetchOptions{URL: " https://example.com/demo.tar.gz "}) {
		t.Fatal("expected URL mode")
	}
}

func TestValidateFetchedBundlePlatformRejectsMismatch(t *testing.T) {
	t.Parallel()

	err := validateFetchedBundlePlatform(exportMetadata{Platform: "claude"}, "codex-runtime")
	if err == nil || !strings.Contains(err.Error(), `requested platform "codex-runtime"`) {
		t.Fatalf("error = %v", err)
	}
}
