package agentpluginscli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/processlock"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/transaction"
)

type doctorObservedStore struct {
	transaction.StateStore
	loaded     chan struct{}
	beforeLoad func()
}

func (store doctorObservedStore) Load() (domain.StateFileV2, error) {
	if store.beforeLoad != nil {
		store.beforeLoad()
	}
	select {
	case store.loaded <- struct{}{}:
	default:
	}
	return store.StateStore.Load()
}

type doctorReadOnlyLockFunc func(context.Context) (ports.UnlockFunc, error)

func (lock doctorReadOnlyLockFunc) Acquire(ctx context.Context) (ports.UnlockFunc, error) {
	return lock(ctx)
}

func (lock doctorReadOnlyLockFunc) AcquireReadOnly(ctx context.Context) (ports.UnlockFunc, error) {
	return lock(ctx)
}

func TestDoctorSnapshotWaitsForActiveMutation(t *testing.T) {
	fixture := newCLIFixture(t, nil)
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	unlock, err := fixture.app.Lifecycle.Lock.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = unlock() }()
	// An active journal deliberately cannot decode. An unlocked doctor fails
	// while the protected observation must wait for transaction cleanup.
	if err := os.MkdirAll(fixture.operations, 0o700); err != nil {
		t.Fatal(err)
	}
	journal := filepath.Join(fixture.operations, "active.json")
	if err := os.WriteFile(journal, []byte("in-progress"), 0o600); err != nil {
		t.Fatal(err)
	}
	loaded := make(chan struct{}, 1)
	fixture.app.StateStore = doctorObservedStore{StateStore: fixture.app.StateStore, loaded: loaded}
	contended := make(chan struct{})
	var once sync.Once
	original := fixture.app.Lifecycle.Lock
	fixture.app.Lifecycle.Lock = doctorReadOnlyLockFunc(func(ctx context.Context) (ports.UnlockFunc, error) {
		release, err := original.(interface {
			AcquireReadOnly(context.Context) (ports.UnlockFunc, error)
		}).AcquireReadOnly(ctx)
		if errors.Is(err, processlock.ErrActive) {
			once.Do(func() { close(contended) })
		}
		return release, err
	})
	finished := make(chan error, 1)
	go func() {
		report, _, _, err := collectDoctorSnapshot(ctx, fixture.app, nil, nil)
		if err == nil && report.OpenOperationCount != 0 {
			err = errors.New("doctor exposed active transaction as abandoned")
		}
		finished <- err
	}()
	select {
	case <-contended:
	case err := <-finished:
		t.Fatalf("doctor read active journal without waiting: %v", err)
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	select {
	case <-loaded:
		t.Fatal("doctor loaded state before acquiring the mutation lock")
	default:
	}
	if err := os.Remove(journal); err != nil {
		t.Fatal(err)
	}
	if err := unlock(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-finished:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
}

func TestDoctorLockCancellationAndFailures(t *testing.T) {
	t.Run("parent deadline bounds contention", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(t.Context(), 30*time.Millisecond)
		defer cancel()
		_, err := acquireDoctorLock(ctx, doctorReadOnlyLockFunc(func(context.Context) (ports.UnlockFunc, error) {
			return nil, processlock.ErrActive
		}))
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("lock error = %v", err)
		}
	})
	t.Run("canceled during successful acquisition releases", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		releaseErr := errors.New("release failed")
		calls := 0
		_, err := acquireDoctorLock(ctx, doctorReadOnlyLockFunc(func(context.Context) (ports.UnlockFunc, error) {
			cancel()
			return func() error { calls++; return releaseErr }, nil
		}))
		if !errors.Is(err, context.Canceled) || !errors.Is(err, releaseErr) || calls != 1 {
			t.Fatalf("lock error = %v; releases = %d", err, calls)
		}
	})
	t.Run("ordinary error is not retried", func(t *testing.T) {
		want := errors.New("unsafe lock")
		calls := 0
		_, err := acquireDoctorLock(t.Context(), doctorReadOnlyLockFunc(func(context.Context) (ports.UnlockFunc, error) {
			calls++
			return nil, want
		}))
		if !errors.Is(err, want) || calls != 1 {
			t.Fatalf("lock error = %v; acquisitions = %d", err, calls)
		}
	})
	t.Run("missing release fails closed", func(t *testing.T) {
		_, err := acquireDoctorLock(t.Context(), doctorReadOnlyLockFunc(func(context.Context) (ports.UnlockFunc, error) {
			return nil, nil
		}))
		if err == nil || !strings.Contains(err.Error(), "no release function") {
			t.Fatalf("lock error = %v", err)
		}
	})
}

func TestDoctorSnapshotJoinsReadAndReleaseErrors(t *testing.T) {
	fixture := newCLIFixture(t, nil)
	readErr, releaseErr := errors.New("corrupt state"), errors.New("unlock failure")
	fixture.app.StateStore = scopedFloorTestStore{err: readErr}
	var released atomic.Int32
	fixture.app.Lifecycle.Lock = doctorReadOnlyLockFunc(func(context.Context) (ports.UnlockFunc, error) {
		return func() error { released.Add(1); return releaseErr }, nil
	})
	_, _, _, err := collectDoctorSnapshot(t.Context(), fixture.app, nil, nil)
	if !errors.Is(err, readErr) || !errors.Is(err, releaseErr) || released.Load() != 1 {
		t.Fatalf("snapshot error = %v; releases = %d", err, released.Load())
	}
}

func TestDoctorLockRejectsMutatingOnlyCapability(t *testing.T) {
	calls := 0
	_, err := acquireDoctorLock(t.Context(), migrationLockFunc(func(context.Context) (ports.UnlockFunc, error) {
		calls++
		return func() error { return nil }, nil
	}))
	if err == nil || calls != 0 {
		t.Fatalf("doctor fell back to a mutating lock: %v calls=%d", err, calls)
	}
}

func TestDoctorSnapshotLeavesMissingLockAndParentAbsent(t *testing.T) {
	fixture := newCLIFixture(t, nil)
	lock := fixture.app.Lifecycle.Lock.(processlock.Lock)
	if _, err := os.Lstat(filepath.Dir(lock.Path)); !os.IsNotExist(err) {
		t.Fatalf("fixture lock parent exists: %v", err)
	}
	if _, _, _, err := collectDoctorSnapshot(t.Context(), fixture.app, nil, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Dir(lock.Path)); !os.IsNotExist(err) {
		t.Fatalf("doctor created lock parent: %v", err)
	}
}

func TestDoctorSnapshotRetriesWholeReadForFirstWriter(t *testing.T) {
	fixture := newCLIFixture(t, nil)
	lock := fixture.app.Lifecycle.Lock.(processlock.Lock)
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	journal := filepath.Join(fixture.operations, "first-writer.json")
	var once sync.Once
	var writer ports.UnlockFunc
	var loads atomic.Int32
	fixture.app.StateStore = doctorObservedStore{StateStore: fixture.app.StateStore, beforeLoad: func() {
		loads.Add(1)
		once.Do(func() {
			var err error
			writer, err = lock.Acquire(ctx)
			if err != nil {
				t.Error(err)
				return
			}
			release := writer
			t.Cleanup(func() { _ = release() })
			if err := os.MkdirAll(fixture.operations, 0700); err != nil {
				t.Error(err)
				return
			}
			if err := os.WriteFile(journal, []byte("in-progress"), 0600); err != nil {
				t.Error(err)
			}
		})
	}}
	contended := make(chan struct{})
	var notified sync.Once
	fixture.app.Lifecycle.Lock = doctorReadOnlyLockFunc(func(ctx context.Context) (ports.UnlockFunc, error) {
		release, err := lock.AcquireReadOnly(ctx)
		if errors.Is(err, processlock.ErrActive) {
			notified.Do(func() { close(contended) })
		}
		return release, err
	})
	finished := make(chan error, 1)
	go func() {
		report, _, _, err := collectDoctorSnapshot(ctx, fixture.app, nil, nil)
		if err == nil && report.OpenOperationCount != 0 {
			err = errors.New("mixed first-writer snapshot accepted")
		}
		finished <- err
	}()
	select {
	case <-contended:
	case err := <-finished:
		t.Fatalf("snapshot did not retry first writer: %v", err)
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	if loads.Load() != 1 {
		t.Fatal("state reread while writer held lock", loads.Load())
	}
	if err := os.Remove(journal); err != nil {
		t.Fatal(err)
	}
	if err := writer(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-finished:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	if loads.Load() < 2 {
		t.Fatal("partial snapshot reused after first writer")
	}
}

func TestDoctorSnapshotParentCancellationAfterReadWins(t *testing.T) {
	fixture := newCLIFixture(t, nil)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	fixture.app.StateStore = doctorObservedStore{StateStore: fixture.app.StateStore, beforeLoad: cancel}
	_, _, _, err := collectDoctorSnapshot(ctx, fixture.app, nil, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled snapshot was reported as complete: %v", err)
	}
}

func TestDoctorSnapshotAcquisitionBudgetDoesNotBoundCoherentRead(t *testing.T) {
	for _, invalidated := range []bool{false, true} {
		name := "coherent"
		if invalidated {
			name = "invalidated"
		}
		t.Run(name, func(t *testing.T) {
			fixture := newCLIFixture(t, nil)
			ctx, cancel := context.WithTimeout(t.Context(), 12*time.Second)
			defer cancel()
			acquisitions := 0
			fixture.app.Lifecycle.Lock = doctorReadOnlyLockFunc(func(context.Context) (ports.UnlockFunc, error) {
				acquisitions++
				return func() error {
					if invalidated {
						return processlock.ErrActive
					}
					return nil
				}, nil
			})
			fixture.app.StateStore = doctorObservedStore{StateStore: fixture.app.StateStore, beforeLoad: func() {
				// Exceed only the existing five-second acquisition/retry budget.
				time.Sleep(5100 * time.Millisecond)
			}}
			_, _, _, err := collectDoctorSnapshot(ctx, fixture.app, nil, nil)
			if acquisitions != 1 || !invalidated && err != nil || invalidated && !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("acquisition deadline changed snapshot policy: invalidated=%v acquisitions=%d err=%v", invalidated, acquisitions, err)
			}
		})
	}
}
