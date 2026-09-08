//go:build windows

package scaffold

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

func TestWindowsRenameExclusiveExistingDestination(t *testing.T) {
	parent := t.TempDir()
	for _, name := range []string{"source", "destination"} {
		path := filepath.Join(parent, name)
		if err := os.Mkdir(path, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(path, "marker"), []byte(name), 0600); err != nil {
			t.Fatal(err)
		}
	}
	dir, err := os.Open(parent)
	if err != nil {
		t.Fatal(err)
	}
	defer dir.Close()
	err = renameExclusive(dir, "source", dir, "destination")
	if !errors.Is(err, os.ErrExist) {
		t.Fatalf("expected os.ErrExist, got %T: %v", err, err)
	}
	for _, name := range []string{"source", "destination"} {
		contents, err := os.ReadFile(filepath.Join(parent, name, "marker"))
		if err != nil || string(contents) != name {
			t.Errorf("%s changed: contents=%q err=%v", name, contents, err)
		}
	}
}

// Deliberately hold a directory without delete sharing. This models a reader's
// ancestry pin and distinguishes opening the source from committing the rename.
func TestWindowsRenameExclusiveHeldDirectoryDiagnostics(t *testing.T) {
	for _, heldName := range []string{"source", "destination"} {
		t.Run(heldName, func(t *testing.T) {
			parent := t.TempDir()
			for _, name := range []string{"source", "destination"} {
				dir := filepath.Join(parent, name)
				if err := os.Mkdir(dir, 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, "marker"), []byte(name), 0600); err != nil {
					t.Fatal(err)
				}
			}
			name, err := windows.UTF16PtrFromString(filepath.Join(parent, heldName))
			if err != nil {
				t.Fatal(err)
			}
			held, err := windows.CreateFile(name, windows.FILE_LIST_DIRECTORY, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
			if err != nil {
				t.Fatal(err)
			}
			defer windows.CloseHandle(held)
			dir, err := os.Open(parent)
			if err != nil {
				t.Fatal(err)
			}
			defer dir.Close()
			err = renameExclusive(dir, "source", dir, "destination")
			t.Logf("held=%s raw_error=%T %v exists=%t sharing=%t", heldName, err, err, errors.Is(err, os.ErrExist), errors.Is(err, windows.ERROR_SHARING_VIOLATION))
			if heldName == "source" {
				if !errors.Is(err, windows.ERROR_SHARING_VIOLATION) || !strings.Contains(err.Error(), "open source directory for exclusive rename") {
					t.Fatalf("source pin must preserve source-open sharing error: %v", err)
				}
			} else {
				// Diagnostic checkpoint: Windows can report destination sharing before
				// name collision. Neither outcome authorizes replacing that destination.
				// The existing end-to-end concurrency test still strictly requires ErrExist.
				if err == nil || !strings.Contains(err.Error(), "commit exclusive directory rename") || (!errors.Is(err, os.ErrExist) && !errors.Is(err, windows.ERROR_SHARING_VIOLATION)) {
					t.Fatalf("unexpected held-destination native result: %v", err)
				}
			}
			for _, name := range []string{"source", "destination"} {
				body, readErr := os.ReadFile(filepath.Join(parent, name, "marker"))
				if readErr != nil || string(body) != name {
					t.Fatalf("%s modified: %q %v", name, body, readErr)
				}
			}
		})
	}
}

// A peer package reader pins shared ancestors with read sharing only. Compare
// this exact share policy against a write-sharing control without any timing.
func TestWindowsRenameExclusiveHeldParentDiagnostics(t *testing.T) {
	for _, share := range []uint32{windows.FILE_SHARE_READ, windows.FILE_SHARE_READ | windows.FILE_SHARE_WRITE | windows.FILE_SHARE_DELETE} {
		t.Run(fmt.Sprintf("share-%d", share), func(t *testing.T) {
			parent := t.TempDir()
			if err := os.Mkdir(filepath.Join(parent, "source"), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(parent, "source", "marker"), []byte("original"), 0600); err != nil {
				t.Fatal(err)
			}
			name, err := windows.UTF16PtrFromString(parent)
			if err != nil {
				t.Fatal(err)
			}
			held, err := windows.CreateFile(name, windows.FILE_READ_ATTRIBUTES|windows.FILE_LIST_DIRECTORY, share, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
			if err != nil {
				t.Fatal(err)
			}
			defer windows.CloseHandle(held)
			dir, err := os.Open(parent)
			if err != nil {
				t.Fatal(err)
			}
			defer dir.Close()
			err = renameExclusive(dir, "source", dir, "destination")
			t.Logf("parent_share=%d raw_error=%T %v exists=%t sharing=%t", share, err, err, errors.Is(err, os.ErrExist), errors.Is(err, windows.ERROR_SHARING_VIOLATION))
			if err != nil && (!errors.Is(err, windows.ERROR_SHARING_VIOLATION) || !strings.Contains(err.Error(), "commit exclusive directory rename")) {
				t.Fatalf("unexpected parent-pin outcome: %v", err)
			}
			location := "destination"
			if err != nil {
				location = "source"
			}
			body, readErr := os.ReadFile(filepath.Join(parent, location, "marker"))
			if readErr != nil || string(body) != "original" {
				t.Fatalf("tree changed: %q %v", body, readErr)
			}
			if share == windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE && err != nil {
				t.Fatalf("fully shared control must rename: %v", err)
			}
		})
	}
}
