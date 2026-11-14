//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func qoderMetadataUnknownMember(t *testing.T, root, name, kind string) string {
	t.Helper()
	path := filepath.Join(root, ".auth", name)
	var err error
	switch kind {
	case "directory":
		err = os.Mkdir(path, 0700)
	case "symlink":
		err = os.Symlink("machine_id", path)
	case "fifo":
		err = syscall.Mkfifo(path, 0600)
	default:
		err = os.WriteFile(path, []byte("unknown body must remain unread and unchanged"), 0000)
	}
	if err != nil {
		t.Fatal(err)
	}
	return path
}

func TestQoderOriginalMetadataRejectsUnknownAuthMemberBeforeBaseline(t *testing.T) {
	for _, item := range []struct{ name, kind string }{
		{"user", "file"}, {"user.production", "file"}, {"unknown", "file"},
		{"unknown-directory", "directory"}, {"unknown-symlink", "symlink"}, {"unknown-fifo", "fifo"},
	} {
		t.Run(item.name, func(t *testing.T) {
			root := qoderMetadataFixture(t)
			recipe, protected := qoderMetadataAuthor(t, root)
			path := qoderMetadataUnknownMember(t, root, item.name, item.kind)
			before, err := os.Lstat(path)
			if err != nil {
				t.Fatal(err)
			}
			// No whole-profile snapshot exists: the recipe itself must refuse.
			if err := recipe.check(); err == nil {
				t.Fatal("unknown auth member accepted before catalog baseline")
			}
			after, err := os.Lstat(path)
			if err != nil || !projectNativeRuntimeStableStat(before.Sys(), after.Sys()) {
				t.Fatal("refusal changed the unknown auth member")
			}
			for original, digest := range protected {
				current, err := projectNativeRuntimeDigest(original)
				if err != nil || current != digest {
					t.Fatal("refusal changed authored metadata bytes")
				}
			}
		})
	}
}

func TestQoderOriginalMetadataInitialCaptureRejectsUnknownAuthMember(t *testing.T) {
	root := qoderMetadataFixture(t)
	qoderMetadataAuthor(t, root)
	qoderMetadataUnknownMember(t, root, "user", "file")
	profile, err := qoderLogOpenRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := profile.Close(); err != nil {
			t.Error(err)
		}
	}()
	auth, err := qoderLogOpenAt(profile, ".auth", true)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := auth.Close(); err != nil {
			t.Error(err)
		}
	}()
	fresh := &qoderOriginalMetadata{root: root}
	if err := fresh.capture(profile, auth); err == nil {
		t.Fatal("initial capture accepted an unknown auth member")
	}
}

func TestQoderOriginalMetadataMembershipChecksUseFreshDirectoryOffset(t *testing.T) {
	root := qoderMetadataFixture(t)
	recipe, _ := qoderMetadataAuthor(t, root)
	for range 3 {
		for _, item := range qoderMetadataRecipe {
			path := filepath.Join(root, item.name)
			before, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(path, item.mode); err != nil {
				t.Fatal(err)
			}
			after, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(before, after) {
				t.Fatal("ordinary metadata read or chmod changed content")
			}
		}
		if err := os.Chmod(filepath.Join(root, ".auth"), 0700); err != nil {
			t.Fatal(err)
		}
		if err := recipe.check(); err != nil {
			t.Fatal("repeated check pinned timestamps or reused a directory offset")
		}
	}
}
