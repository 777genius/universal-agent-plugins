package authoredpath

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverPrefersCanonicalPluginLayout(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mustWriteAuthoredFile(t, filepath.Join(root, "plugin", "plugin.yaml"))
	mustWriteAuthoredFile(t, filepath.Join(root, "src", "plugin.yaml"))

	manifestPath, authoredRoot, ok := Discover(root)
	if !ok {
		t.Fatal("expected authored layout")
	}
	if manifestPath != filepath.Join(root, "plugin", "plugin.yaml") {
		t.Fatalf("manifestPath = %q", manifestPath)
	}
	if authoredRoot != filepath.Join(root, "plugin") {
		t.Fatalf("authoredRoot = %q", authoredRoot)
	}
}

func TestDiscoverSupportsLegacySrcLayout(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	mustWriteAuthoredFile(t, filepath.Join(root, "src", "plugin.yaml"))

	manifestPath, authoredRoot, ok := Discover(root)
	if !ok {
		t.Fatal("expected legacy src layout")
	}
	if manifestPath != filepath.Join(root, "src", "plugin.yaml") {
		t.Fatalf("manifestPath = %q", manifestPath)
	}
	if authoredRoot != filepath.Join(root, "src") {
		t.Fatalf("authoredRoot = %q", authoredRoot)
	}
	if got, want := Join(root, "mcp", "servers.yaml"), filepath.Join(root, "src", "mcp", "servers.yaml"); got != want {
		t.Fatalf("Join(...) = %q, want %q", got, want)
	}
}

func TestDirFallsBackToPluginLayoutWhenManifestIsMissing(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	if got, want := Dir(root), filepath.Join(root, "plugin"); got != want {
		t.Fatalf("Dir(...) = %q, want %q", got, want)
	}
	if HasManifest(root) {
		t.Fatal("HasManifest unexpectedly returned true")
	}
}

func mustWriteAuthoredFile(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %q: %v", path, err)
	}
	if err := os.WriteFile(path, []byte("api_version: v1\n"), 0o644); err != nil {
		t.Fatalf("write %q: %v", path, err)
	}
}
