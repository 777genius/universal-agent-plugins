package contracttest

import (
	"path/filepath"
	"testing"
)

// canonicalTempRoot returns a symlink-free temp directory: native root checks
// demand canonical paths, which t.TempDir() does not guarantee on macOS (/var
// vs /private/var).
func canonicalTempRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return root
}
