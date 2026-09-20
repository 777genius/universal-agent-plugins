package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadFetchedBundleMetadataRejectsRequestedPlatformMismatch(t *testing.T) {
	t.Parallel()

	bundle := mustBundleArchiveBytes(t, exportMetadata{
		PluginName:   "demo",
		Platform:     "claude",
		Runtime:      "node",
		BundleFormat: "tar.gz",
		GeneratedBy:  "plugin-kit-ai export",
	}, map[string]bundleEntry{
		"plugin/plugin.yaml": {mode: 0o644, body: []byte("name: demo\n")},
	})
	archivePath := filepath.Join(t.TempDir(), "demo.tar.gz")
	if err := os.WriteFile(archivePath, bundle, 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := loadFetchedBundleMetadata(archivePath, PluginBundleFetchOptions{
		Platform: "codex-runtime",
		Runtime:  "node",
	})
	if err == nil || !strings.Contains(err.Error(), `requested platform "codex-runtime"`) {
		t.Fatalf("error = %v", err)
	}
}

func TestPrepareFetchedBundleArchiveWritesBody(t *testing.T) {
	t.Parallel()

	archivePath, cleanup, err := prepareFetchedBundleArchive(bundleRemoteSource{
		ArchiveBytes: []byte("bundle"),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	body, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "bundle" {
		t.Fatalf("body = %q", body)
	}
}
