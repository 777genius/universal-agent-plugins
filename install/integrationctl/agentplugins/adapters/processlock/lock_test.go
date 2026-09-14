package processlock

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

func TestLockIsExclusiveAndReleasedWithProcessHandle(t *testing.T) {
	t.Parallel()
	lock := Lock{Path: filepath.Join(t.TempDir(), "mutation.lock")}
	release, err := lock.Acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := lock.Acquire(context.Background()); !errors.Is(err, ErrActive) {
		t.Fatalf("second process handle conflict = %v", err)
	}
	if err := release(); err != nil {
		t.Fatal(err)
	}
	releaseAgain, err := lock.Acquire(context.Background())
	if err != nil {
		t.Fatalf("lock remained stale after release: %v", err)
	}
	if err := releaseAgain(); err != nil {
		t.Fatal(err)
	}
}

func TestLockRejectsSymlinkTarget(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	target := filepath.Join(root, "target")
	if err := writeEmpty(target); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "mutation.lock")
	if err := symlink(target, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, err := (Lock{Path: link}).Acquire(context.Background()); err == nil {
		t.Fatal("symlink mutation lock was accepted")
	}
}
