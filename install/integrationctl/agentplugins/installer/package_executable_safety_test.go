package installer

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packagedigest"
)

func TestSnapshotExecutableInventoryRemainsInsidePackageRoot(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "package")
	if err := os.MkdirAll(filepath.Join(root, "bin"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "bin", "run"), []byte("run"), 0o700); err != nil {
		t.Fatal(err)
	}
	tempRoot := filepath.Join(base, "snapshots")
	if err := os.MkdirAll(tempRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := validateAcquiredExecutables(root, []string{"bin/run"}); err != nil {
		t.Fatalf("valid acquired executable: %v", err)
	}
	snapshot, err := snapshotRequestPackage(t.Context(), tempRoot, Request{
		PackageRoot: root, ExecutableFiles: []string{"bin/run"},
	})
	if err != nil {
		t.Fatalf("valid explicit executable snapshot: %v", err)
	}
	if err := packagedigest.Remove(snapshot); err != nil {
		t.Fatal(err)
	}

	outside := filepath.Join(base, "outside")
	if err := os.MkdirAll(outside, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outside, "secret"), []byte("outside"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, rel string
	}{
		{"empty", ""},
		{"parent", ".."},
		{"traversal", "../outside/secret"},
		{"nested traversal", "bin/../../outside/secret"},
		{"absolute", filepath.Join(outside, "secret")},
		{"dot", "."},
		{"directory", "bin"},
		{"missing", "bin/missing"},
		{"embedded separator", "bin//run"},
		{"nul", "bin/\x00run"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := validateAcquiredExecutables(root, []string{tc.rel}); !errors.Is(err, ErrInvalidRequest) {
				t.Fatalf("acquired executable %q was accepted: %v", tc.rel, err)
			}
			_, err := snapshotRequestPackage(t.Context(), tempRoot, Request{
				PackageRoot: root, ExecutableFiles: []string{tc.rel},
			})
			if !errors.Is(err, ErrInvalidRequest) {
				t.Fatalf("explicit executable %q was accepted: %v", tc.rel, err)
			}
		})
	}
}

func TestSnapshotExecutableInventoryRefusesSymlinkEscape(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "package")
	outside := filepath.Join(base, "outside")
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(outside, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outside, "secret"), []byte("outside"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "link")); err != nil {
		t.Skipf("symlink fixture unavailable on this platform: %v", err)
	}
	for _, rel := range []string{"link/secret", "link"} {
		t.Run(rel, func(t *testing.T) {
			if err := validateAcquiredExecutables(root, []string{rel}); !errors.Is(err, ErrInvalidRequest) {
				t.Fatalf("acquired executable traversed a symlink: %q: %v", rel, err)
			}
			_, err := snapshotRequestPackage(t.Context(), filepath.Join(base, "unused"), Request{
				PackageRoot: root, ExecutableFiles: []string{rel},
			})
			if !errors.Is(err, ErrInvalidRequest) {
				t.Fatalf("explicit executable traversed a symlink: %q: %v", rel, err)
			}
		})
	}
}
