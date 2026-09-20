//go:build linux || darwin

package nativeimport

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestBuildRejectsSymlinkedAncestor(t *testing.T) {
	realParent := t.TempDir()
	source := filepath.Join(realParent, "claude.json")
	if err := os.WriteFile(source, []byte(`{"mcpServers":{"safe":{"command":"node"}}}`), 0600); err != nil {
		t.Fatal(err)
	}
	linkParent := filepath.Join(t.TempDir(), "linked")
	if err := os.Symlink(realParent, linkParent); err != nil {
		t.Fatal(err)
	}
	_, err := Build(context.Background(), filepath.Join(linkParent, "claude.json"), "claude", "imported-plugin", "Imported package.")
	if nativeCode(t, err) != "source_unavailable" {
		t.Fatalf("symlinked ancestor error = %v", err)
	}
}
