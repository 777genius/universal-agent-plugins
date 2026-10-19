package pluginmanifest

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestContainedArtifactRelRejectsEscapes(t *testing.T) {
	t.Parallel()
	for _, rel := range []string{"..", "../escape.txt", "/abs.txt", "."} {
		if _, err := containedArtifactRel(rel); err == nil {
			t.Fatalf("artifact path %q must be rejected", rel)
		}
	}
	if runtime.GOOS == "windows" {
		if _, err := containedArtifactRel("C:escape.txt"); err == nil {
			t.Fatal("drive-relative artifact path must be rejected")
		}
	}
	cleaned, err := containedArtifactRel("skills/demo/SKILL.md")
	if err != nil {
		t.Fatal(err)
	}
	if cleaned != filepath.Clean("skills/demo/SKILL.md") {
		t.Fatalf("cleaned = %q", cleaned)
	}
}

func TestWriteAndRemoveArtifactsStayInsideRoot(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "victim.txt")
	if err := os.WriteFile(outside, []byte("safe"), 0o644); err != nil {
		t.Fatal(err)
	}
	rel, err := filepath.Rel(root, outside)
	if err != nil {
		t.Fatal(err)
	}
	if err := writeArtifacts(root, []Artifact{{RelPath: filepath.ToSlash(rel), Content: []byte("pwned")}}); err == nil {
		t.Fatal("writeArtifacts must reject paths escaping the plugin root")
	}
	body, err := os.ReadFile(outside)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "safe" {
		t.Fatalf("outside file was modified: %q", body)
	}
	if err := removeArtifacts(root, []string{filepath.ToSlash(rel)}); err == nil {
		t.Fatal("removeArtifacts must reject paths escaping the plugin root")
	}
	if _, err := os.Stat(outside); err != nil {
		t.Fatalf("outside file must survive: %v", err)
	}
}

func TestWriteAndRemoveRejectSymlinkedDestinations(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	outsideDir := t.TempDir()
	outside := filepath.Join(outsideDir, "victim.txt")
	if err := os.WriteFile(outside, []byte("safe"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outsideDir, filepath.Join(root, "skills")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	err := writeArtifacts(root, []Artifact{{RelPath: "skills/pwned.txt", Content: []byte("pwned")}})
	if err == nil {
		t.Fatal("writeArtifacts must reject symlinked destination components")
	}
	if _, statErr := os.Stat(filepath.Join(outsideDir, "pwned.txt")); !os.IsNotExist(statErr) {
		t.Fatalf("outside dir must stay untouched: %v", statErr)
	}
	if err := writeArtifacts(root, []Artifact{{RelPath: "skills/victim.txt", Content: []byte("pwned")}}); err == nil {
		t.Fatal("writeArtifacts must not write through a symlinked directory")
	}
	if body, readErr := os.ReadFile(outside); readErr != nil || string(body) != "safe" {
		t.Fatalf("outside file was modified: %q, %v", body, readErr)
	}
	if err := removeArtifacts(root, []string{"skills/victim.txt"}); err == nil {
		t.Fatal("removeArtifacts must not remove through a symlinked directory")
	}
	if _, statErr := os.Stat(outside); statErr != nil {
		t.Fatalf("outside file must survive: %v", statErr)
	}
}
