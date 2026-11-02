package packagedigest

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestSnapshotRejectsParentTraversalAfterDirectorySymlink(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "package")
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(".", filepath.Join(root, "alias")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("alias/..", filepath.Join(root, "parent")); err != nil {
		t.Fatal(err)
	}
	snapshot, err := (Builder{TempRoot: t.TempDir()}).Snapshot(context.Background(), root, domain.SourceIdentity{})
	if err == nil {
		t.Cleanup(func() { _ = Remove(snapshot) })
		t.Fatal("accepted a link whose parent traversal follows a directory symlink")
	}
}

func TestResolveInternalLinkTraversesComponentsInFilesystemOrder(t *testing.T) {
	known := map[string]entry{
		".":          {kind: "directory"},
		"dir":        {kind: "directory"},
		"dir/file":   {kind: "file"},
		"file":       {kind: "file"},
		"alias":      {kind: "symlink", target: "."},
		"directory":  {kind: "symlink", target: "dir"},
		"dir/parent": {kind: "symlink", target: ".."},
		"cycle":      {kind: "symlink", target: "cycle"},
	}
	for _, test := range []struct {
		name, link, target string
		valid              bool
	}{
		{"ordinary file", "link", "dir/file", true},
		{"internal parent", "dir/link", "../file", true},
		{"directory link and parent", "link", "directory/../file", true},
		{"repeated root alias", "link", "alias/alias/file", true},
		{"link to root", "link", ".", true},
		{"root alias then parent", "link", "alias/..", false},
		{"nested alias then parent", "link", "dir/parent/..", false},
		{"missing component before parent", "link", "missing/../file", false},
		{"file component before parent", "link", "file/../file", false},
		{"file component before dot", "link", "file/.", false},
		{"file with trailing separator", "link", "file/", false},
		{"cycle", "link", "cycle", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := resolveInternalLink(test.link, test.target, known)
			if (err == nil) != test.valid {
				t.Fatalf("resolveInternalLink(%q, %q) = %v; valid = %t", test.link, test.target, err, test.valid)
			}
		})
	}
}

func TestSnapshotPreservesLegalDirectoryLinks(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "dir", "file"), []byte("inside package"), 0o600)
	// Single-component targets keep the same stored spelling on every host.
	// Create directory targets before their links so Windows records their type.
	links := []struct{ name, target, file string }{
		{"alias", ".", "dir/file"},
		{"directory", "dir", "file"},
		{"linked", "directory", "file"},
	}
	for _, link := range links {
		if err := os.Symlink(link.target, filepath.Join(root, link.name)); err != nil {
			t.Fatal(err)
		}
	}
	snapshot, err := (Builder{TempRoot: t.TempDir()}).Snapshot(context.Background(), root, domain.SourceIdentity{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := Remove(snapshot); err != nil {
			t.Errorf("remove snapshot: %v", err)
		}
	})
	for _, link := range links {
		body, err := os.ReadFile(filepath.Join(snapshot.Root, link.name, filepath.FromSlash(link.file)))
		if err != nil || string(body) != "inside package" {
			t.Fatalf("internal directory link %q content = %q, %v", link.name, body, err)
		}
		if target, err := os.Readlink(filepath.Join(snapshot.Root, link.name)); err != nil || target != link.target {
			t.Fatalf("original relative link %q changed: %q, %v", link.name, target, err)
		}
	}
}

func TestSnapshotEnforcesNativeLinkTargetPolicy(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "dir", "file"), []byte("inside package"), 0o600)
	const originalTarget = "directory/../alias/dir/file"
	for _, link := range []struct{ name, target string }{
		{"alias", "."},
		{"directory", "dir"},
		{"copy", originalTarget},
	} {
		if err := os.Symlink(link.target, filepath.Join(root, link.name)); err != nil {
			t.Fatal(err)
		}
	}
	// Windows Symlink stores native backslashes. Such text is outside the
	// portable package policy; it must be rejected rather than normalized.
	if target, err := os.Readlink(filepath.Join(root, "copy")); err != nil || target != filepath.FromSlash(originalTarget) {
		t.Fatalf("native link spelling = %q, %v", target, err)
	}
	snapshot, err := (Builder{TempRoot: t.TempDir()}).Snapshot(context.Background(), root, domain.SourceIdentity{})
	if err == nil {
		t.Cleanup(func() {
			if err := Remove(snapshot); err != nil {
				t.Errorf("remove snapshot: %v", err)
			}
		})
	}
	if runtime.GOOS == "windows" {
		const refusal = `inspect package tree: unsafe symlink "copy": target must be a non-empty relative UTF-8 path using forward slashes`
		if err == nil || err.Error() != refusal {
			t.Fatalf("native backslash link refusal = %v; want %q", err, refusal)
		}
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(snapshot.Root, "copy"))
	if err != nil || string(body) != "inside package" {
		t.Fatalf("internal link content = %q, %v", body, err)
	}
	if target, err := os.Readlink(filepath.Join(snapshot.Root, "copy")); err != nil || target != originalTarget {
		t.Fatalf("original relative link changed: %q, %v", target, err)
	}
}
