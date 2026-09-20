package app

import (
	"strings"
	"testing"
)

func TestValidateBundleURLInputRejectsNonHTTPS(t *testing.T) {
	t.Parallel()

	_, err := validateBundleURLInput("http://example.com/demo.tar.gz", fakeBundleDownloader{})
	if err == nil || !strings.Contains(err.Error(), "https://") {
		t.Fatalf("error = %v", err)
	}
}

func TestValidateBundleURLInputRejectsMissingDownloader(t *testing.T) {
	t.Parallel()

	_, err := validateBundleURLInput("https://example.com/demo.tar.gz", nil)
	if err == nil || !strings.Contains(err.Error(), "downloader is required") {
		t.Fatalf("error = %v", err)
	}
}
