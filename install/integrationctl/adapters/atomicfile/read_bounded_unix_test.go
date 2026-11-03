//go:build !windows

package atomicfile

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// The child timeout is a failure guard, not a timing assertion. The replacement
// happens exactly between inspection and opening, with no background race/sleep.
func TestBoundedReaderFIFOReplacement(t *testing.T) {
	const childKey = "UAP_BOUNDED_READ_FIFO_CHILD"
	if os.Getenv(childKey) != "1" {
		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		defer cancel()
		command := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestBoundedReaderFIFOReplacement$")
		command.Env = append(os.Environ(), childKey+"=1")
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("nonblocking read child failed (%v, context %v): %s", err, ctx.Err(), output)
		}
		return
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "metadata.json")
	if err := os.WriteFile(path, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := readBoundedRegularFile(path, 16, func(name string) (*os.File, error) {
		if err := os.Remove(name); err != nil {
			t.Fatal(err)
		}
		if err := unix.Mkfifo(name, 0o600); err != nil {
			t.Fatal(err)
		}
		return openRegularNoFollow(name)
	})
	if !errors.Is(err, ErrUnsafeFile) {
		t.Fatalf("substituted FIFO: %v", err)
	}
	if _, err := ReadBoundedRegularFile(path, 16); !errors.Is(err, ErrUnsafeFile) {
		t.Fatalf("existing FIFO: %v", err)
	}
	if err := os.Symlink("metadata.json", filepath.Join(dir, "uap.toml")); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if file, err := OpenRootReadFile(root, "uap.toml"); !errors.Is(err, ErrUnsafeFile) {
		if file != nil {
			_ = file.Close()
		}
		t.Fatalf("rooted FIFO link: %v", err)
	}
}
