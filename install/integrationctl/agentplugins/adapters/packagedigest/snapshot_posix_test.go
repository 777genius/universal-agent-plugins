//go:build !windows

package packagedigest

import (
	"context"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
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
			if _, err := (Builder{TempRoot: t.TempDir()}).Snapshot(context.Background(), root, domain.SourceIdentity{}); err == nil {
				t.Fatal("hazard accepted")
			}
		})
	}
}
