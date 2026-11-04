package management

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

type applicationPausedAcceptanceStore struct {
	*applicationTestStore
	before           bool
	entered, release chan struct{}
}

func (s *applicationPausedAcceptanceStore) Save(record Observation) error {
	if record.Operation.State != Queued {
		return s.applicationTestStore.Save(record)
	}
	if s.before {
		close(s.entered)
		<-s.release
	}
	err := s.applicationTestStore.Save(record)
	if !s.before {
		close(s.entered)
		<-s.release
	}
	return err
}

func TestApplicationShutdownDuringAcceptancePreservesIdentityWithoutExecution(t *testing.T) {
	for _, tc := range []struct {
		name   string
		before bool
	}{{"before publication", true}, {"after publication", false}} {
		t.Run(tc.name, func(t *testing.T) {
			store := &applicationPausedAcceptanceStore{
				applicationTestStore: &applicationTestStore{records: map[string]Observation{}},
				before:               tc.before, entered: make(chan struct{}), release: make(chan struct{}),
			}
			release := sync.OnceFunc(func() { close(store.release) })
			defer release()
			prepared := &applicationTestPrepared{}
			backend := &applicationTestBackend{prepare: func(context.Context, ChangeRequest, string) (Prepared, error) { return prepared, nil }}
			lock := &applicationTestLock{}
			app, err := NewApplication(t.Context(), Options{Backend: backend, Store: store, Lock: lock, ContextID: "capture", EnvironmentID: "environment", IntentRoot: filepath.Join(t.TempDir(), "intent")})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(app.Close)
			plan := prepareApplicationTest(t, app)
			const request = "acceptance-shutdown-request-123456"
			returned := make(chan error, 1)
			go func() { _, err := app.Apply(t.Context(), plan.PlanId, request); returned <- err }()
			awaitApplicationSignal(t, store.entered)
			go app.Close()
			awaitApplicationSignal(t, app.Stopping())
			release()
			select {
			case err := <-returned:
				requireRefusal(t, err, "SESSION_EXPIRED")
			case <-time.After(5 * time.Second):
				t.Fatal("acceptance did not finish after shutdown")
			}
			awaitApplicationSignal(t, app.Done())
			record, err := store.ByRequest(request)
			if err != nil || record == nil || record.Operation.State != Queued {
				t.Fatal("shutdown lost durable acceptance", record, err)
			}
			if prepared.calls.Load() != 0 || prepared.closed.Load() != 1 || backend.closed.Load() != 1 || lock.held.Load() {
				t.Fatal("shutdown transferred or lost unexecuted authority")
			}
			opts := app.opts
			opts.ContextID, opts.Backend = "reopened-capture", &applicationTestBackend{}
			reopened, err := NewApplication(t.Context(), opts)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(reopened.Close)
			replayed, err := reopened.Apply(t.Context(), plan.PlanId, request)
			if err != nil || replayed.State != Interrupted || replayed.Result != nil || replayed.OperationId != record.Operation.OperationId || prepared.calls.Load() != 0 {
				t.Fatal("reopen lost or executed the accepted identity", replayed, err)
			}
		})
	}
}
