//go:build darwin || linux

package dirswap

import (
	"bytes"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestOriginalPublicationRefusesOriginalUnixOwnershipDrift(t *testing.T) {
	for _, field := range []string{"uid", "gid", "links"} {
		t.Run(field, func(t *testing.T) {
			_, input := originalPublicationFixture(t)
			held := holdOriginalPublication(t, input.StagingPath, true)
			original := held.entries[2].Info
			stat, ok := original.Sys().(*syscall.Stat_t)
			if !ok {
				t.Fatal("original held stat authority unavailable")
			}
			expected := *stat
			switch field {
			case "uid":
				stat.Uid ^= 1
			case "gid":
				stat.Gid ^= 1
			case "links":
				stat.Nlink++
			}
			current, err := held.marker.Stat()
			if err != nil || !os.SameFile(original, current) || original.Mode() != current.Mode() ||
				!original.ModTime().Equal(current.ModTime()) || original.Size() != current.Size() {
				t.Fatalf("ownership-only original snapshot fixture changed another fact: %v", err)
			}
			proof, err := CaptureOriginalPublication(held.root, held.entries)
			if err == nil || !proof.IsZero() {
				t.Fatalf("re-baselined original %s instead of refusing: %v", field, err)
			}
			assertOriginalUnixPublicationUnchanged(t, held, input.StagingPath, expected)
		})
	}
}

func assertOriginalUnixPublicationUnchanged(t *testing.T, held *heldPublicationFixture, stage string, expected syscall.Stat_t) {
	t.Helper()
	after, err := held.marker.Stat()
	if err != nil {
		t.Fatal(err)
	}
	stat, ok := after.Sys().(*syscall.Stat_t)
	if !ok || stat.Dev != expected.Dev || stat.Ino != expected.Ino || stat.Mode != expected.Mode ||
		stat.Uid != expected.Uid || stat.Gid != expected.Gid || stat.Nlink != expected.Nlink || stat.Size != expected.Size {
		t.Fatal("refusal changed the real held file metadata")
	}
	if !os.SameFile(held.entries[2].Info, after) || !held.entries[2].Info.ModTime().Equal(after.ModTime()) {
		t.Fatal("refusal changed original file identity or modification time")
	}
	named, err := held.selected.Lstat("marker")
	if err != nil || !os.SameFile(after, named) {
		t.Fatalf("refusal detached the original held file from its name: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(stage, "plugins", "marker"))
	if err != nil || !bytes.Equal(body, []byte(objectPublicationMarker)) {
		t.Fatalf("refusal changed actual original bytes: %v", err)
	}
}
