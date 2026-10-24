//go:build windows

package grok

import (
	"os"
	"path/filepath"
	"testing"
)

func TestProjectGrokWindowsAnchoredOpenRejectsSameObjectAncestorAlias(t *testing.T) {
	for _, directory := range []bool{false, true} {
		t.Run(map[bool]string{false: "manifest", true: "directory"}[directory], func(t *testing.T) {
			base := projectTestRoot(t)
			parent := filepath.Join(base, "parent")
			path := filepath.Join(parent, "leaf")
			if directory {
				if err := os.MkdirAll(path, 0700); err != nil {
					t.Fatal(err)
				}
			} else {
				projectTestWrite(t, path, `{"name":"demo"}`)
			}
			// Check symlink capability before introducing the deterministic swap.
			probe := filepath.Join(base, "probe")
			if err := os.Symlink(parent, probe); err != nil {
				t.Skipf("symlink unavailable: %v", err)
			}
			if err := os.Remove(probe); err != nil {
				t.Fatal(err)
			}
			before, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			file, err := openProjectWindows(path, directory, func() {
				if err := os.Rename(parent, parent+"-old"); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(parent+"-old", parent); err != nil {
					t.Fatal(err)
				}
			})
			if file != nil {
				_ = file.Close()
			}
			if err == nil {
				t.Fatal("same-object ancestor alias was admitted")
			}
			after, err := os.Stat(path)
			if err != nil || !os.SameFile(before, after) {
				t.Fatal("fixture failed to preserve exact same leaf object", err)
			}
		})
	}
}

func TestProjectGrokWindowsReadRecheckRefusesSameObjectAncestorAlias(t *testing.T) {
	base := projectTestRoot(t)
	parent := filepath.Join(base, "parent")
	path := filepath.Join(parent, "manifest.json")
	projectTestWrite(t, path, `{"name":"demo"}`)
	probe := filepath.Join(base, "probe")
	if err := os.Symlink(parent, probe); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if err := os.Remove(probe); err != nil {
		t.Fatal(err)
	}
	file, info, err := openProjectPath(path, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = file.Close() }()
	if err := os.Rename(parent, parent+"-old"); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(parent+"-old", parent); err != nil {
		t.Fatal(err)
	}
	if err := stableProjectPath(path, file, info); err == nil {
		t.Fatal("post-read same-object ancestor alias admitted")
	}
}
