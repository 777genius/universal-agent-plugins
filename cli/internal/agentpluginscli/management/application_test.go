package management

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type applicationTestStore struct {
	mu             sync.Mutex
	records        map[string]Observation
	failSave       string
	lookupCount    int
	appearOnSecond *Observation
}

func (s *applicationTestStore) ByRequest(id string) (*Observation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lookupCount++
	if s.lookupCount == 2 && s.appearOnSecond != nil {
		record := cloneObservation(*s.appearOnSecond)
		s.records[record.Operation.RequestId] = record
	}
	record, ok := s.records[id]
	if !ok {
		return nil, nil
	}
	record = cloneObservation(record)
	return &record, nil
}
func (s *applicationTestStore) ByOperation(id string) (*Observation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, record := range s.records {
		if record.Operation.OperationId == id {
			out := cloneObservation(record)
			return &out, nil
		}
	}
	return nil, nil
}
func (s *applicationTestStore) List() ([]Observation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Observation, 0, len(s.records))
	for _, record := range s.records {
		out = append(out, cloneObservation(record))
	}
	return out, nil
}
func (s *applicationTestStore) Save(record Observation) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	mode := s.failSave
	s.failSave = ""
	if mode == "before" {
		return errors.New("save refused before publication")
	}
	s.records[record.Operation.RequestId] = cloneObservation(record)
	if mode == "after" {
		return errors.New("publication visible but durability confirmation failed")
	}
	return nil
}
func (s *applicationTestStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.records, id)
	return nil
}

var errApplicationTestActive = errors.New("lock active")

type applicationTestLock struct{ held atomic.Bool }

func (l *applicationTestLock) Acquire(ctx context.Context) (func() error, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !l.held.CompareAndSwap(false, true) {
		return nil, errApplicationTestActive
	}
	var once sync.Once
	return func() error { once.Do(func() { l.held.Store(false) }); return nil }, nil
}
func (*applicationTestLock) IsActive(err error) bool { return errors.Is(err, errApplicationTestActive) }

type applicationTestBackend struct {
	prepare func(context.Context, ChangeRequest, string) (Prepared, error)
	session func(context.Context) (Session, error)
	forget  func(Operation) error
	closed  atomic.Int32
}

func (b *applicationTestBackend) Session(ctx context.Context) (Session, error) {
	if b.session != nil {
		return b.session(ctx)
	}
	return Session{}, nil
}
func (*applicationTestBackend) Catalog(context.Context, string) (Catalog, error) {
	return Catalog{}, nil
}
func (*applicationTestBackend) CatalogDetail(context.Context, string) (CatalogEntry, error) {
	return CatalogEntry{}, nil
}
func (*applicationTestBackend) SkillsCatalog(context.Context, SourceRequest) (SkillCatalog, error) {
	return SkillCatalog{}, nil
}
func (*applicationTestBackend) Inspect(context.Context) (Environment, error) {
	return Environment{}, nil
}
func (*applicationTestBackend) CheckUpdates(context.Context, UpdateCheckRequest) (UpdateCheck, error) {
	return UpdateCheck{}, nil
}
func (b *applicationTestBackend) Prepare(ctx context.Context, input ChangeRequest, id string) (Prepared, error) {
	if b.prepare != nil {
		return b.prepare(ctx, input, id)
	}
	return &applicationTestPrepared{}, nil
}
func (b *applicationTestBackend) PrepareLocalSkill(ctx context.Context, _ LocalSkillRequest, id string) (Prepared, error) {
	return b.Prepare(ctx, ChangeRequest{Action: LocalSkill}, id)
}
func (b *applicationTestBackend) ForgetObservation(_ context.Context, op Operation) error {
	if b.forget != nil {
		return b.forget(op)
	}
	return nil
}
func (b *applicationTestBackend) Close() { b.closed.Add(1) }

type applicationTestPrepared struct {
	review        Plan
	check         func(context.Context) error
	apply         func(context.Context, func(Progress)) (Outcome, error)
	calls, closed atomic.Int32
}

func (p *applicationTestPrepared) Review() Plan {
	if p.review.Actions != nil {
		return p.review
	}
	return Plan{Confirmable: true, Actions: []PlanAction{{Action: "install"}}}
}
func (p *applicationTestPrepared) Check(ctx context.Context) error {
	if p.check != nil {
		return p.check(ctx)
	}
	return nil
}
func (p *applicationTestPrepared) Apply(ctx context.Context, progress func(Progress)) (Outcome, error) {
	p.calls.Add(1)
	if p.apply != nil {
		return p.apply(ctx, progress)
	}
	return applicationSuccess(), nil
}
func (*applicationTestPrepared) OperationIDs() []string { return []string{"retained-installer-id"} }
func (p *applicationTestPrepared) Close()               { p.closed.Add(1) }
func applicationSuccess() Outcome {
	return Outcome{State: Succeeded, Result: OperationResult{Outcome: "completed", Items: []OperationItem{}, NextActions: []NextAction{}}}
}
func newApplicationTest(t *testing.T, backend *applicationTestBackend) (*Application, *applicationTestStore, *applicationTestLock) {
	t.Helper()
	if backend == nil {
		backend = &applicationTestBackend{}
	}
	store := &applicationTestStore{records: map[string]Observation{}}
	lock := &applicationTestLock{}
	app, err := NewApplication(t.Context(), Options{Backend: backend, Store: store, Lock: lock, ContextID: "capture", EnvironmentID: "environment", IntentRoot: filepath.Join(t.TempDir(), "intent")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(app.Close)
	return app, store, lock
}
func prepareApplicationTest(t *testing.T, app *Application) Plan {
	t.Helper()
	plan, err := app.Prepare(t.Context(), ChangeRequest{Action: Sync})
	if err != nil {
		t.Fatal(err)
	}
	return plan
}
func awaitApplicationTest(t *testing.T, app *Application, id string) Operation {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		op, err := app.Operation(t.Context(), id)
		if err != nil {
			t.Fatal(err)
		}
		if terminal(op.State) {
			return op
		}
		select {
		case <-deadline:
			t.Fatal("operation did not finish")
		case <-time.After(time.Millisecond):
		}
	}
}
func requireRefusal(t *testing.T, err error, code string) {
	t.Helper()
	var refusal *Refusal
	if !errors.As(err, &refusal) || refusal.Code != code {
		t.Fatalf("wanted %s, got %v", code, err)
	}
}

func TestApplicationAcceptanceSaveFailureDoesNotExecute(t *testing.T) {
	for _, mode := range []string{"before", "after"} {
		t.Run(mode, func(t *testing.T) {
			prepared := &applicationTestPrepared{}
			app, store, _ := newApplicationTest(t, &applicationTestBackend{prepare: func(context.Context, ChangeRequest, string) (Prepared, error) { return prepared, nil }})
			plan := prepareApplicationTest(t, app)
			store.failSave = mode
			const request = "acceptance-failure-123456"
			_, err := app.Apply(t.Context(), plan.PlanId, request)
			requireRefusal(t, err, "ACCEPTANCE_UNAVAILABLE")
			if prepared.calls.Load() != 0 || prepared.closed.Load() != 0 {
				t.Fatal("failed save transferred or executed authority")
			}
			record, err := store.ByRequest(request)
			if err != nil {
				t.Fatal(err)
			}
			if mode == "before" {
				if record != nil {
					t.Fatal("save before publication consumed identity")
				}
				op, err := app.Apply(t.Context(), plan.PlanId, request)
				if err != nil {
					t.Fatal(err)
				}
				if awaitApplicationTest(t, app, op.OperationId).State != Succeeded || prepared.calls.Load() != 1 {
					t.Fatal("fresh retry did not execute exactly once")
				}
			} else {
				if record == nil {
					t.Fatal("ambiguous publication lost identity")
				}
				op, err := app.Apply(t.Context(), plan.PlanId, request)
				if err != nil {
					t.Fatal(err)
				}
				if op.State != Interrupted || op.Result != nil || op.OperationId != record.Operation.OperationId || prepared.calls.Load() != 0 {
					t.Fatal("ambiguous acceptance replayed effects", op)
				}
				_, err = app.Apply(t.Context(), plan.PlanId, "unrelated-request-123456")
				requireRefusal(t, err, "RECOVERY_REQUIRED")
			}
		})
	}
}

func TestApplicationSecondReplayObservesConcurrentAcceptanceWithoutBlocking(t *testing.T) {
	prepared := &applicationTestPrepared{}
	app, store, lock := newApplicationTest(t, &applicationTestBackend{prepare: func(context.Context, ChangeRequest, string) (Prepared, error) { return prepared, nil }})
	plan := prepareApplicationTest(t, app)
	const request = "concurrent-acceptance-123456"
	prior := Observation{Version: 1, EnvironmentID: "environment", ContextID: "prior-capture", IntentRoot: app.opts.IntentRoot, Fingerprint: digest(plan.PlanId), Operation: Operation{EnvironmentId: "environment", ContextId: "prior-capture", OperationId: "other-process-operation", PlanId: plan.PlanId, RequestId: request, State: Queued}}
	store.appearOnSecond = &prior
	done := make(chan Operation, 1)
	failure := make(chan error, 1)
	go func() {
		op, err := app.Apply(t.Context(), plan.PlanId, request)
		if err != nil {
			failure <- err
			return
		}
		done <- op
	}()
	select {
	case err := <-failure:
		t.Fatal(err)
	case <-time.After(5 * time.Second):
		t.Fatal("nested observation blocked on held mutation lock")
	case op := <-done:
		if op.OperationId != prior.Operation.OperationId || op.State != Queued || prepared.calls.Load() != 0 {
			t.Fatal("second replay started duplicate work", op)
		}
	}
	if lock.held.Load() {
		t.Fatal("replay did not release acquisition")
	}
	op, err := app.Apply(t.Context(), plan.PlanId, request)
	if err != nil || op.State != Interrupted || prepared.calls.Load() != 0 {
		t.Fatal("abandoned observation executed", op, err)
	}
}

func TestApplicationRetainsIndependentReviewAndOperationSnapshots(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	backendReview := Plan{Confirmable: true, Actions: []PlanAction{{Action: "reviewed"}}, Warnings: []Diagnostic{{Code: "original"}}}
	prepared := &applicationTestPrepared{review: backendReview, apply: func(ctx context.Context, progress func(Progress)) (Outcome, error) {
		progress(Progress{Items: []OperationItem{{Alias: "retained-item"}}, Completed: 0, Total: 1})
		close(started)
		select {
		case <-release:
		case <-ctx.Done():
			return Outcome{}, ctx.Err()
		}
		result := applicationSuccess()
		result.Result.LocalSkillBackup = &LocalSkillBackup{OperationId: "backup", RootId: "root", SkillName: "mine"}
		result.Result.NextActions = []NextAction{{Kind: "display", Reason: "retained-action"}}
		return result, nil
	}}
	app, _, _ := newApplicationTest(t, &applicationTestBackend{prepare: func(context.Context, ChangeRequest, string) (Prepared, error) { return prepared, nil }})
	plan, err := app.PrepareLocalSkill(t.Context(), LocalSkillRequest{Action: Upsert, RootId: "root", SkillName: "mine"})
	if err != nil {
		t.Fatal(err)
	}
	plan.Actions[0].Action = "caller mutation"
	plan.Warnings[0].Code = "caller mutation"
	backendReview.Actions[0].Action = "backend mutation"
	backendReview.Warnings[0].Code = "backend mutation"
	app.mu.Lock()
	retained := clonePlan(app.plans[plan.PlanId].review)
	app.mu.Unlock()
	if retained.Actions[0].Action != "reviewed" || retained.Warnings[0].Code != "original" {
		t.Fatal("review shares mutable caller/backend data", retained)
	}
	accepted, err := app.Apply(t.Context(), plan.PlanId, "snapshot-request-123456")
	if err != nil {
		t.Fatal(err)
	}
	accepted.LocalSkill.RootId = "caller mutation"
	<-started
	snapshot, err := app.Operation(t.Context(), accepted.OperationId)
	if err != nil {
		t.Fatal(err)
	}
	snapshot.LocalSkill.SkillName = "caller mutation"
	snapshot.Items[0].Alias = "caller mutation"
	next, err := app.Operation(t.Context(), accepted.OperationId)
	if err != nil || next.LocalSkill.RootId != "root" || next.LocalSkill.SkillName != "mine" || next.Items[0].Alias != "retained-item" {
		t.Fatal("running snapshot escaped", next, err)
	}
	close(release)
	completed := awaitApplicationTest(t, app, accepted.OperationId)
	if completed.Result == nil || completed.Result.LocalSkillBackup == nil || completed.Result.LocalSkillBackup.RecordedAt == nil {
		t.Fatal("completion missing backup metadata", completed)
	}
	completed.Result.NextActions[0].Reason = "caller mutation"
	completed.Result.LocalSkillBackup.RootId = "caller mutation"
	*completed.Result.LocalSkillBackup.RecordedAt = time.Time{}
	again, err := app.Operation(t.Context(), accepted.OperationId)
	if err != nil || again.Result.NextActions[0].Reason != "retained-action" || again.Result.LocalSkillBackup.RootId != "root" || again.Result.LocalSkillBackup.RecordedAt.IsZero() {
		t.Fatal("completed snapshot escaped", again, err)
	}
}

func TestApplicationCloseCancelsAdmittedWorkAndOwnsBackendOnce(t *testing.T) {
	for _, phase := range []string{"read", "prepare", "accepted"} {
		t.Run(phase, func(t *testing.T) {
			started, release, finished := make(chan struct{}), make(chan struct{}), make(chan struct{})
			prepared := &applicationTestPrepared{}
			backend := &applicationTestBackend{}
			block := func(ctx context.Context) { close(started); <-ctx.Done(); <-release }
			switch phase {
			case "read":
				backend.session = func(ctx context.Context) (Session, error) { block(ctx); return Session{}, ctx.Err() }
			case "prepare":
				backend.prepare = func(ctx context.Context, _ ChangeRequest, _ string) (Prepared, error) {
					block(ctx)
					return prepared, nil
				}
			case "accepted":
				backend.prepare = func(context.Context, ChangeRequest, string) (Prepared, error) { return prepared, nil }
				prepared.apply = func(ctx context.Context, _ func(Progress)) (Outcome, error) { block(ctx); return Outcome{}, ctx.Err() }
			}
			app, _, lock := newApplicationTest(t, backend)
			var operationID string
			if phase == "accepted" {
				plan := prepareApplicationTest(t, app)
				op, err := app.Apply(t.Context(), plan.PlanId, "closing-request-123456")
				if err != nil {
					t.Fatal(err)
				}
				operationID = op.OperationId
			} else {
				go func() {
					defer close(finished)
					if phase == "read" {
						_, _ = app.Session(t.Context())
					} else {
						_, _ = app.Prepare(t.Context(), ChangeRequest{Action: Sync})
					}
				}()
			}
			<-started
			closeReturned := make(chan struct{})
			go func() { app.Close(); close(closeReturned) }()
			<-app.Stopping()
			if backend.closed.Load() != 0 {
				t.Fatal("backend closed before admitted work exited")
			}
			select {
			case <-closeReturned:
				t.Fatal("Close returned before work completed")
			default:
			}
			_, err := app.Prepare(t.Context(), ChangeRequest{Action: Sync})
			requireRefusal(t, err, "SESSION_EXPIRED")
			close(release)
			select {
			case <-closeReturned:
			case <-time.After(5 * time.Second):
				t.Fatal("Close deadlocked")
			}
			app.Close()
			if backend.closed.Load() != 1 || lock.held.Load() {
				t.Fatal("shutdown lost exclusive ownership")
			}
			if phase != "read" && prepared.closed.Load() != 1 {
				t.Fatal("prepared authority was not closed exactly once", prepared.closed.Load())
			}
			if phase == "prepare" && prepared.calls.Load() != 0 {
				t.Fatal("late preparation executed")
			}
			if phase == "accepted" {
				record, err := app.opts.Store.ByOperation(operationID)
				if err != nil || record == nil || record.Operation.State != Interrupted {
					t.Fatal("shutdown lost accepted uncertainty", record, err)
				}
			} else {
				<-finished
			}
		})
	}
}

func TestCleanupRefusalRetainsHistoryWithoutBlockingBelowBudget(t *testing.T) {
	var forgetCalls atomic.Int32
	var refuse atomic.Bool
	refuse.Store(true)
	backend := &applicationTestBackend{forget: func(Operation) error {
		forgetCalls.Add(1)
		if refuse.Load() {
			return errors.New("private backup changed")
		}
		return nil
	}}
	app, store, lock := newApplicationTest(t, backend)
	old := Observation{Version: 1, EnvironmentID: "environment", ContextID: "prior", Operation: Operation{RequestId: "expired-observation-request", OperationId: "expired-backup", State: Succeeded, LocalSkill: &LocalSkillIdentity{RootId: "user", SkillName: "mine"}, Result: &OperationResult{Outcome: "completed"}, UpdatedAt: time.Now().Add(-31 * 24 * time.Hour)}}
	if err := store.Save(old); err != nil {
		t.Fatal(err)
	}
	unlock, err := lock.Acquire(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = unlock() }()
	if err := app.reserveHistory(time.Now()); err != nil {
		t.Fatal("cleanup refusal blocked below budget", err)
	}
	if kept, _ := store.ByRequest(old.Operation.RequestId); kept == nil || forgetCalls.Load() != 1 {
		t.Fatal("unverified private undo lost observation")
	}
	refuse.Store(false)
	if err := app.reserveHistory(time.Now()); err != nil {
		t.Fatal(err)
	}
	if kept, _ := store.ByRequest(old.Operation.RequestId); kept != nil {
		t.Fatal("verified cleanup retained observation")
	}
}
