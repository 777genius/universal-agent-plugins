package packagesnapshot

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestBuildCreatesSealedSnapshotAndPreservesExecutableIntent(t *testing.T) {
	t.Parallel()
	source := t.TempDir()
	mustWrite(t, filepath.Join(source, "plugin.json"), []byte(`{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"demo"}`), 0o644)
	mustWrite(t, filepath.Join(source, "bin", "run"), []byte("#!/bin/sh\n"), 0o755)
	mustWrite(t, filepath.Join(source, ".git", "ignored"), []byte("ignored"), 0o644)
	mustWrite(t, filepath.Join(source, "nested", ".uap-integrations.lock"), []byte("ignored"), 0o644)
	mustWrite(t, filepath.Join(source, "nested", ".plugin-kit-ai.lock"), []byte("ignored"), 0o644)

	snapshot, err := (Builder{}).Build(context.Background(), source)
	if err != nil {
		t.Fatalf("build snapshot: %v", err)
	}
	snapshotParent := filepath.Dir(snapshot.Root)
	defer snapshot.Close()
	if snapshot.Digest == "" || snapshot.FileCount != 2 {
		t.Fatalf("snapshot metadata = %+v", snapshot)
	}
	if len(snapshot.ExecutableFiles) != 1 || snapshot.ExecutableFiles[0] != "bin/run" {
		t.Fatalf("executable files = %#v", snapshot.ExecutableFiles)
	}
	if _, err := os.Stat(filepath.Join(snapshot.Root, ".git")); !os.IsNotExist(err) {
		t.Fatalf(".git copied into snapshot: %v", err)
	}
	for _, name := range []string{".uap-integrations.lock", ".plugin-kit-ai.lock"} {
		if _, err := os.Stat(filepath.Join(snapshot.Root, "nested", name)); !os.IsNotExist(err) {
			t.Fatalf("workspace lock %s copied into snapshot: %v", name, err)
		}
	}
	executable, err := os.Stat(filepath.Join(snapshot.Root, "bin", "run"))
	if err != nil {
		t.Fatal(err)
	}
	if executable.Mode().Perm() != 0o555 {
		t.Fatalf("executable mode = %o, want 555", executable.Mode().Perm())
	}
	regular, err := os.Stat(filepath.Join(snapshot.Root, "plugin.json"))
	if err != nil {
		t.Fatal(err)
	}
	if regular.Mode().Perm() != 0o444 {
		t.Fatalf("regular mode = %o, want 444", regular.Mode().Perm())
	}
	if err := snapshot.Close(); err != nil {
		t.Fatalf("close snapshot: %v", err)
	}
	if _, err := os.Stat(snapshotParent); !os.IsNotExist(err) {
		t.Fatalf("snapshot parent survived cleanup: %v", err)
	}
}

func TestBuildDigestChangesWithExecutableIntent(t *testing.T) {
	t.Parallel()
	source := t.TempDir()
	path := filepath.Join(source, "run")
	mustWrite(t, path, []byte("same bytes"), 0o644)
	first, err := (Builder{}).Build(context.Background(), source)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	if err := os.Chmod(path, 0o755); err != nil {
		t.Fatal(err)
	}
	second, err := (Builder{}).Build(context.Background(), source)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	if first.Digest == second.Digest {
		t.Fatal("digest ignored executable intent")
	}
}

func TestBuildRejectsSymlink(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation is not generally available without elevated privileges")
	}
	source := t.TempDir()
	mustWrite(t, filepath.Join(source, "real"), []byte("data"), 0o644)
	if err := os.Symlink("real", filepath.Join(source, "link")); err != nil {
		t.Fatal(err)
	}
	if _, err := (Builder{}).Build(context.Background(), source); err == nil {
		t.Fatal("symlink accepted")
	}
}

func TestBuildRejectsHardlinks(t *testing.T) {
	t.Parallel()
	source := t.TempDir()
	original := filepath.Join(source, "original")
	mustWrite(t, original, []byte("data"), 0o644)
	if err := os.Link(original, filepath.Join(source, "second-link")); err != nil {
		t.Skipf("hardlinks unavailable: %v", err)
	}
	if _, err := (Builder{}).Build(context.Background(), source); err == nil || !strings.Contains(err.Error(), "hardlinks are not allowed") {
		t.Fatalf("hardlink error = %v", err)
	}
}

func TestBuildRejectsAncestorReplacedBySymlinkDuringCopy(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation is not generally available without elevated privileges")
	}
	source := t.TempDir()
	outside := t.TempDir()
	mustWrite(t, filepath.Join(source, "nested", "plugin.json"), []byte("inside"), 0o644)
	mustWrite(t, filepath.Join(outside, "plugin.json"), []byte("outside"), 0o644)
	swapped := false
	builder := Builder{beforeCopy: func(rel string) error {
		if rel != "nested/plugin.json" || swapped {
			return nil
		}
		swapped = true
		original := filepath.Join(source, "nested")
		if err := os.Rename(original, original+"-old"); err != nil {
			return err
		}
		return os.Symlink(outside, original)
	}}
	if _, err := builder.Build(context.Background(), source); err == nil {
		t.Fatal("concurrent ancestor symlink substitution was accepted")
	}
}

func TestBuildRejectsNonPortablePathSegments(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"CON", "nul.txt", "COM1", "trailing.", "trailing ", `back\\slash`, "colon:name", "star*name", "question?name", "control\x01name"} {
		name := name
		t.Run(name, func(t *testing.T) {
			source := t.TempDir()
			mustWrite(t, filepath.Join(source, name), []byte("data"), 0o644)
			if _, err := (Builder{}).Build(context.Background(), source); err == nil {
				t.Fatalf("non-portable name %q was accepted", name)
			}
		})
	}
}

func TestBuildEnforcesLimits(t *testing.T) {
	t.Parallel()
	source := t.TempDir()
	mustWrite(t, filepath.Join(source, "one"), []byte("1"), 0o644)
	mustWrite(t, filepath.Join(source, "two"), []byte("2"), 0o644)
	_, err := (Builder{Limits: Limits{MaxFiles: 1, MaxFileBytes: 10, MaxTreeBytes: 10, MaxDepth: 2}}).Build(context.Background(), source)
	if err == nil {
		t.Fatal("file-count limit was not enforced")
	}
}

func mustWrite(t *testing.T, path string, body []byte, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, body, mode); err != nil {
		t.Fatal(err)
	}
}
