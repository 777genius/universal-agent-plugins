package management

import (
	"context"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type applicationDelayedObservationStore struct {
	*applicationTestStore
	pauseReplay, pauseInspect                      atomic.Bool
	published, releaseFinal, captured, releaseRead chan struct{}
}

func (s *applicationDelayedObservationStore) ByRequest(id string) (*Observation, error) {
	record, err := s.applicationTestStore.ByRequest(id)
	if s.pauseReplay.CompareAndSwap(true, false) {
		close(s.captured)
		<-s.releaseRead
	}
	return record, err
}

func (s *applicationDelayedObservationStore) List() ([]Observation, error) {
	records, err := s.applicationTestStore.List()
	if s.pauseInspect.CompareAndSwap(true, false) {
		close(s.captured)
		<-s.releaseRead
	}
	return records, err
}

func (s *applicationDelayedObservationStore) Save(record Observation) error {
	err := s.applicationTestStore.Save(record)
	if record.Operation.State == Succeeded && err != nil {
		close(s.published)
		<-s.releaseFinal
	}
	return err
}

type applicationObservedPrepared struct {
	*applicationTestPrepared
	released chan struct{}
}

func (p *applicationObservedPrepared) Close() {
	p.applicationTestPrepared.Close()
	close(p.released)
}

func TestApplicationRejectsSnapshotsCapturedBeforeOutcomeFailure(t *testing.T) {
	for _, view := range []string{"replay", "inspect"} {
		for _, fallback := range []string{"", "before"} {
			t.Run(view+"/fallback_"+fallback, func(t *testing.T) {
				store := &applicationDelayedObservationStore{
					applicationTestStore: &applicationTestStore{records: map[string]Observation{}},
					published:            make(chan struct{}), releaseFinal: make(chan struct{}),
					captured: make(chan struct{}), releaseRead: make(chan struct{}),
				}
				releaseFinal := sync.OnceFunc(func() { close(store.releaseFinal) })
				releaseRead := sync.OnceFunc(func() { close(store.releaseRead) })
				defer releaseFinal()
				defer releaseRead()
				prepared := &applicationObservedPrepared{applicationTestPrepared: &applicationTestPrepared{}, released: make(chan struct{})}
				prepared.apply = func(context.Context, func(Progress)) (Outcome, error) {
					store.mu.Lock()
					store.failSaves = []string{"after", fallback}
					store.mu.Unlock()
					return applicationSuccess(), nil
				}
				backend := &applicationTestBackend{prepare: func(context.Context, ChangeRequest, string) (Prepared, error) { return prepared, nil }}
				lock := &applicationTestLock{}
				app, err := NewApplication(t.Context(), Options{Backend: backend, Store: store, Lock: lock, ContextID: "capture", EnvironmentID: "environment", IntentRoot: filepath.Join(t.TempDir(), "intent")})
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(app.Close)
				plan := prepareApplicationTest(t, app)
				const request = "delayed-observation-request-123456"
				op, err := app.Apply(t.Context(), plan.PlanId, request)
				if err != nil {
					t.Fatal(err)
				}
				awaitApplicationSignal(t, store.published)
				inFlight, err := app.Inspect(t.Context())
				if err != nil || len(inFlight.PendingOperations) != 1 || inFlight.PendingOperations[0].OperationId != op.OperationId {
					t.Fatal("inspection lost the current owner during final publication", inFlight, err)
				}
				returned := make(chan error, 1)
				if view == "replay" {
					store.pauseReplay.Store(true)
					go func() { _, err := app.Apply(t.Context(), plan.PlanId, request); returned <- err }()
				} else {
					store.pauseInspect.Store(true)
					go func() { _, err := app.Inspect(t.Context()); returned <- err }()
				}
				awaitApplicationSignal(t, store.captured)
				releaseFinal()
				awaitApplicationSignal(t, app.Stopping())
				awaitApplicationSignal(t, prepared.released)
				releaseRead()
				select {
				case err := <-returned:
					requireRefusal(t, err, "OBSERVATION_UNAVAILABLE")
				case <-time.After(5 * time.Second):
					t.Fatal("stopped observation did not return")
				}
				awaitApplicationSignal(t, app.Done())
				if prepared.calls.Load() != 1 || prepared.closed.Load() != 1 || backend.closed.Load() != 1 || lock.held.Load() {
					t.Fatal("failed outcome did not drain accepted and observing work")
				}
			})
		}
	}
}
