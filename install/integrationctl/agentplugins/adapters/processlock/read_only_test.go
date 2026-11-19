package processlock

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestReadOnlyLockPreservesWriterMetadataAndExclusion(t *testing.T) {
	root := t.TempDir()
	lock := Lock{Path: filepath.Join(root, "mutation.lock")}
	writer, err := lock.Acquire(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := lock.AcquireReadOnly(t.Context()); !errors.Is(err, ErrActive) {
		t.Fatalf("reader ignored writer: %v", err)
	}
	if err := writer(); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(lock.Path)
	if err != nil {
		t.Fatal(err)
	}
	mode, err := os.Stat(root)
	if err != nil {
		t.Fatal(err)
	}
	reader, err := lock.AcquireReadOnly(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = reader() }()
	if _, err := lock.Acquire(t.Context()); !errors.Is(err, ErrActive) {
		t.Fatalf("writer ignored reader: %v", err)
	}
	if err := reader(); err != nil {
		t.Fatal(err)
	}
	if err := reader(); err != nil {
		t.Fatal("release was not idempotent", err)
	}
	after, readErr := os.ReadFile(lock.Path)
	current, statErr := os.Stat(root)
	if readErr != nil || statErr != nil || !reflect.DeepEqual(before, after) || current.Mode() != mode.Mode() {
		t.Fatalf("read-only lock changed metadata or parent: %v %v", readErr, statErr)
	}
}

func TestReadOnlyLockDoesNotCreateMissingPathsAndDetectsFirstWriter(t *testing.T) {
	for _, createWriter := range []bool{false, true} {
		t.Run(map[bool]string{false: "absent", true: "first-writer"}[createWriter], func(t *testing.T) {
			parent := filepath.Join(t.TempDir(), "missing")
			lock := Lock{Path: filepath.Join(parent, "mutation.lock")}
			reader, err := lock.AcquireReadOnly(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			if _, err := os.Lstat(parent); !os.IsNotExist(err) {
				t.Fatalf("reader created parent: %v", err)
			}
			if createWriter {
				writer, err := lock.Acquire(t.Context())
				if err != nil {
					t.Fatal(err)
				}
				defer func() { _ = writer() }()
			}
			first, second := reader(), reader()
			if createWriter && (!errors.Is(first, ErrActive) || !errors.Is(second, ErrActive)) || !createWriter && (first != nil || second != nil) {
				t.Fatalf("absent lock validation = %v, %v", first, second)
			}
		})
	}
}

func TestReadOnlyLockRejectsUnsafePathsAndCancellation(t *testing.T) {
	root := t.TempDir()
	for _, path := range []string{"", root} {
		if _, err := (Lock{Path: path}).AcquireReadOnly(t.Context()); err == nil {
			t.Fatalf("invalid lock path accepted: %q", path)
		}
	}
	target := filepath.Join(root, "foreign")
	if err := writeEmpty(target); err != nil {
		t.Fatal(err)
	}
	t.Run("symlink", func(t *testing.T) {
		link := filepath.Join(root, "mutation.lock")
		if err := symlink(target, link); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
		if _, err := (Lock{Path: link}).AcquireReadOnly(t.Context()); err == nil {
			t.Fatal("symlink lock accepted")
		}
	})
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := (Lock{Path: target}).AcquireReadOnly(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled read-only lock = %v", err)
	}
}

func TestReadOnlyLockPreservesOriginalModes(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "mutation.lock")
	if err := os.WriteFile(path, []byte("held metadata\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(root, 0755); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(root)
	if err != nil {
		t.Fatal(err)
	}
	fileBefore, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	release, err := (Lock{Path: path}).AcquireReadOnly(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if err := release(); err != nil {
		t.Fatal(err)
	}
	after, err := os.Stat(root)
	if err != nil {
		t.Fatal(err)
	}
	fileAfter, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(path)
	if err != nil || string(body) != "held metadata\n" || before.Mode() != after.Mode() || fileBefore.Mode() != fileAfter.Mode() {
		t.Fatalf("read-only acquisition changed bytes or modes: %v", err)
	}
}

func TestReadOnlyLockRejectsReplacedIdentityOnRelease(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mutation.lock")
	if err := writeEmpty(path); err != nil {
		t.Fatal(err)
	}
	release, err := (Lock{Path: path}).AcquireReadOnly(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = release() }()
	if err := os.Rename(path, path+".held"); err != nil {
		t.Skipf("host cannot replace an open lock: %v", err)
	}
	if err := os.WriteFile(path, []byte("replacement bytes\n"), 0600); err != nil {
		t.Fatal(err)
	}
	first, second := release(), release()
	if first == nil || first != second || errors.Is(first, ErrActive) {
		t.Fatalf("replaced identity accepted/retried: %v %v", first, second)
	}
	body, err := os.ReadFile(path)
	if err != nil || string(body) != "replacement bytes\n" {
		t.Fatalf("release changed replacement: %v", err)
	}
}
