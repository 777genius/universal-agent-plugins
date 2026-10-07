//go:build windows && (amd64 || arm64)

package directoryidentity

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Regression: relative NtCreateFile opens by ambient path, allows a case alias,
// or maps a closed HANDLE to unsupported. Legacy identity did no ancestry walk.
func TestWindowsNativeRelativeDirectoryAndAlias(t *testing.T) {
	parentPath := t.TempDir()
	childPath := filepath.Join(parentPath, "CaseProof")
	if err := os.Mkdir(childPath, 0o700); err != nil {
		t.Fatal(err)
	}

	paths, err := ancestryPaths(parentPath)
	if err != nil {
		t.Fatal(err)
	}
	var parent *os.File
	for i, p := range paths {
		name := p
		if i > 0 {
			name = filepath.Base(p)
		}
		next, err := openDirectory(parent, name)
		if parent != nil {
			if closeErr := parent.Close(); closeErr != nil {
				t.Fatal(closeErr)
			}
		}
		if err != nil {
			if errors.Is(err, ErrUnsupported) {
				t.Skipf("NOT_QUALIFIED: actual TEST drive is unsupported: %v", err)
			}
			t.Fatal(err)
		}
		parent = next
	}
	defer func() {
		if err := parent.Close(); err != nil {
			t.Error(err)
		}
	}()
	child, err := openDirectory(parent, "CaseProof")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := child.Close(); err != nil {
			t.Error(err)
		}
	}()
	if err := verifyCanonicalName(child, childPath); err != nil {
		t.Fatalf("canonical directory: %v", err)
	}
	if err := verifyCanonicalName(child, filepath.Join(parentPath, "caseproof")); err == nil {
		t.Fatal("case alias accepted")
	}
	volume, object, err := directoryFacts(child)
	if err != nil || volume == "" || !strings.Contains(object, ":") {
		t.Fatalf("native facts: %q %q %v", volume, object, err)
	}
	closed, err := openDirectory(parent, "CaseProof")
	if err != nil {
		t.Fatal(err)
	}
	if err := closed.Close(); err != nil {
		t.Fatal(err)
	}
	if _, _, err := directoryFacts(closed); err == nil || errors.Is(err, ErrUnsupported) {
		t.Fatalf("closed handle: %v", err)
	}
	t.Logf("ACTUAL_NTFS_HELD_DIRECTORY=%s:%s; native qualification requires exact host evidence", volume, object)
}
