package usecase

import (
	"path/filepath"
	"testing"
)

// canonicalCodexProfile gives native-operation tests an explicit profile path
// even when the system temporary directory is reached through a symlink.
func canonicalCodexProfile(t *testing.T, name string) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Join(root, name)
}
