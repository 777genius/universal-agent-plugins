package app

import (
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/plugininstall/domain"
)

func TestShouldPromotePublishDraft(t *testing.T) {
	t.Parallel()

	if !shouldPromotePublishDraft(&domain.Release{Draft: true}, false) {
		t.Fatal("expected draft promotion")
	}
}

func TestRequirePublishAssetReplacementRejectsMissingForce(t *testing.T) {
	t.Parallel()

	err := requirePublishAssetReplacement(&domain.Asset{ID: 1}, "demo.tar.gz", false)
	if err == nil || !strings.Contains(err.Error(), "--force") {
		t.Fatalf("error = %v", err)
	}
}
