package packagedigest

import (
	"context"
	"os"
	"path/filepath"
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
	for name, target := range map[string]string{"alias": ".", "directory": "dir", "copy": "directory/../alias/dir/file"} {
		if err := os.Symlink(target, filepath.Join(root, name)); err != nil {
			t.Fatal(err)
		}
	}
	snapshot, err := (Builder{TempRoot: t.TempDir()}).Snapshot(context.Background(), root, domain.SourceIdentity{})
	if err != nil {
		t.Fatal(err)
	}
	defer Remove(snapshot)
	body, err := os.ReadFile(filepath.Join(snapshot.Root, "copy"))
	if err != nil || string(body) != "inside package" {
		t.Fatalf("internal link content = %q, %v", body, err)
	}
	if target, err := os.Readlink(filepath.Join(snapshot.Root, "copy")); err != nil || target != "directory/../alias/dir/file" {
		t.Fatalf("original relative link changed: %q, %v", target, err)
	}
}
