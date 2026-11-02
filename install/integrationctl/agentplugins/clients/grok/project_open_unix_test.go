//go:build !windows

package grok

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/unix"
)

func TestProjectGrokAnchoredOpenRefusesLeafAndAncestorSwaps(t *testing.T) {
	for _, directory := range []bool{false, true} {
		for _, kind := range []string{"fifo", "symlink", "ancestor"} {
			t.Run(map[bool]string{false: "manifest", true: "directory"}[directory]+"/"+kind, func(t *testing.T) {
				root := projectTestRoot(t)
				parent := filepath.Join(root, "parent")
				path := filepath.Join(parent, "leaf")
				if directory {
					if err := os.MkdirAll(path, 0700); err != nil {
						t.Fatal(err)
					}
				} else {
					projectTestWrite(t, path, `{"name":"demo"}`)
				}
				target := projectTestRoot(t)
				projectTestWrite(t, filepath.Join(target, "leaf"), "foreign")
				hook := func() {
					if kind == "ancestor" {
						if err := os.Rename(parent, parent+"-old"); err != nil {
							t.Fatal(err)
						}
						if err := os.Symlink(target, parent); err != nil {
							t.Fatal(err)
						}
						return
					}
					if err := os.RemoveAll(path); err != nil {
						t.Fatal(err)
					}
					if kind == "fifo" {
						if err := unix.Mkfifo(path, 0600); err != nil {
							t.Fatal(err)
						}
					} else if err := os.Symlink(target, path); err != nil {
						t.Fatal(err)
					}
				}
				file, _, err := openProjectPath(path, directory, hook)
				if file != nil {
					_ = file.Close()
				}
				if err == nil {
					t.Fatal("typed nofollow handle admitted path swap")
				}
			})
		}
	}
}
