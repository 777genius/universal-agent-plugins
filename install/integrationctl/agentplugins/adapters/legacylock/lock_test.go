package legacylock

import (
	"context"
	"errors"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/locks"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type recordingLockManager struct {
	key          string
	releaseCalls int
	err          error
	returnNil    bool
}

func (m *recordingLockManager) Acquire(_ context.Context, key string) (legacyports.UnlockFunc, error) {
	m.key = key
	if m.err != nil {
		return nil, m.err
	}
	if m.returnNil {
		return nil, nil
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
	if _, err := (StateLock{Manager: &recordingLockManager{returnNil: true}}).Acquire(context.Background()); err == nil {
		t.Fatal("nil release from legacy lock manager was accepted")
	}
}

func TestStateLockSharesPhysicalLegacyFileLock(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	legacy := locks.FileLock{BaseDir: t.TempDir()}
	releaseLegacy, err := legacy.Acquire(ctx, "state")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := (StateLock{Manager: legacy}).Acquire(ctx); err == nil {
		_ = releaseLegacy()
		t.Fatal("new lifecycle did not contend with the existing legacy state lock")
	}
	if err := releaseLegacy(); err != nil {
		t.Fatal(err)
	}
	releaseNew, err := (StateLock{Manager: legacy}).Acquire(ctx)
	if err != nil {
		t.Fatalf("new lifecycle failed to acquire legacy lock after release: %v", err)
	}
	if _, err := legacy.Acquire(ctx, "state"); err == nil {
		_ = releaseNew()
		t.Fatal("old lifecycle did not contend with the new lifecycle's lock")
	}
	if err := releaseNew(); err != nil {
		t.Fatal(err)
	}
}
