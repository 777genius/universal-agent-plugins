package app

import (
	"archive/tar"
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestValidateBundleMetadataEnvelopeRejectsMissingPluginName(t *testing.T) {
	t.Parallel()

	err := validateBundleMetadataEnvelope(exportMetadata{
		BundleFormat: "tar.gz",
		GeneratedBy:  "plugin-kit-ai export",
	})
	if err == nil || !strings.Contains(err.Error(), "metadata.plugin_name") {
		t.Fatalf("error = %v", err)
	}
}

func TestValidateBundleMetadataRuntimeRejectsUnsupportedRuntime(t *testing.T) {
	t.Parallel()

	err := validateBundleMetadataRuntime(exportMetadata{
		Platform: "claude",
		Runtime:  "shell",
	})
	if err == nil || !strings.Contains(err.Error(), "python/node") {
		t.Fatalf("error = %v", err)
	}
}

func TestExtractBundleArchiveEntryRejectsUnsupportedType(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	tr := tar.NewReader(&buf)
	err := extractBundleArchiveEntry(tr, t.TempDir(), &tar.Header{Name: "demo", Typeflag: tar.TypeSymlink}, newBundleArchivePolicy())
	if err == nil || !strings.Contains(err.Error(), "symlink entry") {
		t.Fatalf("error = %v", err)
	}
}

func TestRemoveInstalledBundleDestRejectsNonEmptyWithoutForce(t *testing.T) {
	t.Parallel()

	dest := t.TempDir()
	if err := os.WriteFile(dest+"/keep.txt", []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := removeInstalledBundleDest(dest, true, false)
	if err == nil || !strings.Contains(err.Error(), "--force") {
		t.Fatalf("error = %v", err)
	}
}
