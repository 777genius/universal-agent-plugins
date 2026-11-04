package legacylock

import (
	"context"
	"errors"
	"testing"

	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type recordingLockManager struct {
	key          string
	releaseCalls int
	err          error
}

func (m *recordingLockManager) Acquire(_ context.Context, key string) (legacyports.UnlockFunc, error) {
	m.key = key
	if m.err != nil {
		return nil, m.err
	}
	return func() error {
		m.releaseCalls++
		return nil
	}, nil
}

func TestStateLockPreservesLegacyNamespaceAndRelease(t *testing.T) {
	manager := &recordingLockManager{}
	release, err := (StateLock{Manager: manager}).Acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if manager.key != "state" {
		t.Fatalf("lock namespace = %q, want state", manager.key)
	}
	if err := release(); err != nil {
		t.Fatal(err)
	}
	if manager.releaseCalls != 1 {
		t.Fatalf("release calls = %d, want 1", manager.releaseCalls)
	}
}

func TestStateLockFailsClosedOnErrors(t *testing.T) {
	if _, err := (StateLock{}).Acquire(context.Background()); err == nil {
		t.Fatal("nil manager acquired legacy state lock")
	}
	want := errors.New("legacy lock unavailable")
	manager := &recordingLockManager{err: want}
	if _, err := (StateLock{Manager: manager}).Acquire(context.Background()); !errors.Is(err, want) {
		t.Fatalf("lock error = %v, want %v", err, want)
	}
}
