//go:build linux

package pluginkitairepo_test

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"golang.org/x/sys/unix"
)

func TestGeminiCycleHeldOrdinaryTreeRejectsLinksAncestorsSpecialEntriesAndLimits(t *testing.T) {
	for _, kind := range []string{"leaf-alias", "ancestor-alias", "hardlink", "fifo", "mode-drift", "extra", "deadline", "large"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "owned", "data")
			projectNativeRuntimeWrite(t, path, []byte("held"), 0600)
			ctx := context.Background()
			before, err := geminiCycleTree(ctx, root)
			if err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "leaf-alias":
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink("foreign", path); err != nil {
					t.Fatal(err)
				}
			case "ancestor-alias":
				if err := os.Rename(filepath.Dir(path), filepath.Join(root, "other")); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink("other", filepath.Dir(path)); err != nil {
					t.Fatal(err)
				}
			case "hardlink":
				if err := os.Link(path, filepath.Join(root, "linked")); err != nil {
					t.Fatal(err)
				}
			case "fifo":
				if err := unix.Mkfifo(filepath.Join(root, "pipe"), 0600); err != nil {
					t.Fatal(err)
				}
			case "mode-drift":
				if err := os.Chmod(path, 0640); err != nil {
					t.Fatal(err)
				}
			case "extra":
				projectNativeRuntimeWrite(t, filepath.Join(root, "sibling"), []byte("unknown"), 0600)
			case "deadline":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			case "large":
				file, err := os.Create(filepath.Join(root, "large"))
				if err != nil {
					t.Fatal(err)
				}
				resize := file.Truncate((64 << 20) + 1)
				closeErr := file.Close()
				if resize != nil || closeErr != nil {
					t.Fatal("sparse bound fixture")
				}
			}
			after, err := geminiCycleTree(ctx, root)
			if kind == "mode-drift" || kind == "extra" {
				if err != nil || reflect.DeepEqual(before, after) {
					t.Fatal("tree mutation hidden")
				}
			} else if err == nil || after != nil {
				t.Fatal("unsafe tree produced accepted partial inventory")
			}
		})
	}
}
func TestGeminiCycleHeldDirectoryReplacementAndCheckedCloseCannotBecomeBaseline(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "held")
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	edges, err := geminiCycleOpen(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(path, path+"-old"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	if geminiCycleEdges(edges) == nil {
		t.Fatal("linked replacement accepted")
	}
	if geminiCycleClose(edges) != nil {
		t.Fatal("normal held closes")
	}
	if geminiCycleEdges(edges) == nil || geminiCycleClose(edges) == nil {
		t.Fatal("closed descriptor/second close hidden")
	}
}
