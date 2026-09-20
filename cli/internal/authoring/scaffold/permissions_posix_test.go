//go:build !windows

package scaffold

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestDeniedParent(t *testing.T) {
	parent := tempRoot(t)
	if err := os.Chmod(parent, 0500); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(parent, 0700)
	// Root/capability-bearing environments cannot establish this fixture.
	probe := filepath.Join(parent, "permission-probe")
	if err := os.Mkdir(probe, 0700); err == nil {
		os.Remove(probe)
		t.Skip("environment bypasses directory write permissions")
	}
	if r, err := Apply(context.Background(), planFor(t, "skill"), ApplyOptions{Destination: filepath.Join(parent, "out"), Validate: realValidation(t)}); err == nil || r.Committed {
		t.Fatal("denied parent accepted")
	}
}
