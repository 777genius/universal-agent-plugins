//go:build !windows

package packagedigest

import (
	"context"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"os"
	"path/filepath"
	"testing"
)

// Case-distinct siblings and DOS device basenames require a POSIX source tree.
// Windows native reparse/device-namespace rejection is tested by packageview.
func TestSnapshotRejectsPOSIXSourceHazards(t *testing.T) {
	tests := map[string]func(*testing.T, string){
		"case collision": func(t *testing.T, root string) {
			write(t, filepath.Join(root, "Readme"), nil, 0o644)
			write(t, filepath.Join(root, "README"), nil, 0o644)
		},
		"device": func(t *testing.T, root string) { write(t, filepath.Join(root, "CON.txt"), nil, 0o644) },
	}
	for name, prepare := range tests {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			prepare(t, root)
			if name == "case collision" {
				entries, err := os.ReadDir(root)
				if err != nil {
					t.Fatal(err)
				}
				if len(entries) != 2 {
					t.Skip("host filesystem cannot represent case-distinct sibling names")
				}
			}
			snapshot, err := (Builder{TempRoot: t.TempDir()}).Snapshot(context.Background(), root, domain.SourceIdentity{})
			if err == nil {
				if removeErr := Remove(snapshot); removeErr != nil {
					t.Fatalf("hazard accepted and snapshot cleanup failed: %v", removeErr)
				}
				t.Fatal("hazard accepted")
			}
		})
	}
}
