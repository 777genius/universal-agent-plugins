package locks

import (
	"context"
	"testing"
)

func TestFileLockAcquireExclusive(t *testing.T) {
	t.Parallel()
	lock := FileLock{BaseDir: t.TempDir()}
	unlock, err := lock.Acquire(context.Background(), "state")
	if err != nil {
		t.Fatalf("acquire first: %v", err)
	}
	defer func() { _ = unlock() }()
	if _, err := lock.Acquire(context.Background(), "state"); err == nil {
		t.Fatal("expected second acquire to fail")
	}
}
