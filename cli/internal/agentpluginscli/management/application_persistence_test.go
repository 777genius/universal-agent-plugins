package management

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestApplicationPreservesInstallerIdentityEncoding(t *testing.T) {
	for _, tc := range []struct {
		name string
		ids  []string
		want string
	}{
		{"nil", nil, `"installerIds":null`},
		{"empty", []string{}, `"installerIds":[]`},
		{"populated", []string{"installer"}, `"installerIds":["installer"]`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			prepared := &applicationTestPrepared{operationIDs: func() []string { return tc.ids }}
			app, store, _ := newApplicationTest(t, &applicationTestBackend{prepare: func(context.Context, ChangeRequest, string) (Prepared, error) { return prepared, nil }})
			plan := prepareApplicationTest(t, app)
			op, err := app.Apply(t.Context(), plan.PlanId, "encoding-request-123456")
			if err != nil {
				t.Fatal(err)
			}
			if len(tc.ids) > 0 {
				tc.ids[0] = "caller mutation"
			}
			awaitApplicationTest(t, app, op.OperationId)
			record, err := store.ByRequest(op.RequestId)
			if err != nil || record == nil {
				t.Fatal("accepted identity missing", record, err)
			}
			body, err := json.Marshal(record)
			if err != nil || !bytes.Contains(body, []byte(tc.want)) {
				t.Fatalf("version-1 installer identities changed: %s, %v", body, err)
			}
		})
	}
}

func TestApplicationFinalSaveFailureRetainsUncertainty(t *testing.T) {
	for _, mode := range []string{"before", "after"} {
		t.Run(mode, func(t *testing.T) {
			prepared := &applicationTestPrepared{}
			backend := &applicationTestBackend{prepare: func(context.Context, ChangeRequest, string) (Prepared, error) {
				return prepared, nil
			}}
			app, store, _ := newApplicationTest(t, backend)
			prepared.apply = func(context.Context, func(Progress)) (Outcome, error) {
				store.mu.Lock()
				store.failSave = mode
				store.mu.Unlock()
				return applicationSuccess(), nil
			}
			plan := prepareApplicationTest(t, app)
			op, err := app.Apply(t.Context(), plan.PlanId, "final-save-request-123456")
			if err != nil {
				t.Fatal(err)
			}
			awaitApplicationSignal(t, app.Done())
			_, err = app.Operation(t.Context(), op.OperationId)
			requireRefusal(t, err, "SESSION_EXPIRED")
			opts := app.opts
			opts.ContextID = "reopened-capture"
			opts.Backend = &applicationTestBackend{}
			reopened, err := NewApplication(t.Context(), opts)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(reopened.Close)
			observed, err := reopened.Operation(t.Context(), op.OperationId)
			if err != nil {
				t.Fatal(err)
			}
			if observed.State != Interrupted || observed.Result != nil || !hasApplicationDiagnostic(observed, "OUTCOME_NOT_PERSISTED") {
				t.Fatal("failed final save lost uncertainty", observed)
			}
			replayed, err := reopened.Apply(t.Context(), plan.PlanId, op.RequestId)
			if err != nil || replayed.State != Interrupted || replayed.OperationId != op.OperationId || prepared.calls.Load() != 1 {
				t.Fatal("uncertain outcome replayed effects", replayed, err)
			}
			next := prepareApplicationTest(t, reopened)
			_, err = reopened.Apply(t.Context(), next.PlanId, "next-mutation-request-123456")
			requireRefusal(t, err, "RECOVERY_REQUIRED")
		})
	}
}

func TestApplicationStopsWhenUncertainOutcomeCannotBeSaved(t *testing.T) {
	for _, first := range []string{"before", "after"} {
		for _, second := range []string{"before", "after"} {
			t.Run(first+"/"+second, func(t *testing.T) {
				prepared := &applicationTestPrepared{}
				backend := &applicationTestBackend{prepare: func(context.Context, ChangeRequest, string) (Prepared, error) { return prepared, nil }}
				app, store, lock := newApplicationTest(t, backend)
				prepared.apply = func(context.Context, func(Progress)) (Outcome, error) {
					store.mu.Lock()
					store.failSaves = []string{first, second}
					store.mu.Unlock()
					return applicationSuccess(), nil
				}
				plan := prepareApplicationTest(t, app)
				op, err := app.Apply(t.Context(), plan.PlanId, "unsaved-outcome-request-123456")
				if err != nil {
					t.Fatal(err)
				}
				awaitApplicationSignal(t, app.Stopping())
				_, err = app.Operation(t.Context(), op.OperationId)
				requireRefusal(t, err, "SESSION_EXPIRED")
				_, err = app.Prepare(t.Context(), ChangeRequest{Action: Sync})
				requireRefusal(t, err, "SESSION_EXPIRED")
				awaitApplicationSignal(t, app.Done())
				if prepared.calls.Load() != 1 || prepared.closed.Load() != 1 || backend.closed.Load() != 1 || lock.held.Load() {
					t.Fatal("uncertain outcome did not drain its sole owner")
				}
				record, err := store.ByRequest(op.RequestId)
				if err != nil || record == nil || record.Operation.OperationId != op.OperationId {
					t.Fatal("failed saves lost the accepted identity", record, err)
				}
			})
		}
	}
}

func TestApplicationCloseDuringRetentionCancelsBeforeAcceptance(t *testing.T) {
	started, releaseCleanup := make(chan struct{}), make(chan struct{})
	defer close(releaseCleanup)
	prepared := &applicationTestPrepared{}
	backend := &applicationTestBackend{
		prepare: func(context.Context, ChangeRequest, string) (Prepared, error) { return prepared, nil },
		forget: func(ctx context.Context, _ Operation) error {
			close(started)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-releaseCleanup:
				return errors.New("test released cleanup after failed cancellation")
			}
		},
	}
	app, store, lock := newApplicationTest(t, backend)
	old := Observation{Version: 1, EnvironmentID: "environment", ContextID: "prior", Operation: Operation{RequestId: "expired-observation-request", OperationId: "expired-backup", State: Succeeded, LocalSkill: &LocalSkillIdentity{RootId: "user", SkillName: "mine"}, Result: &OperationResult{Outcome: "completed"}, UpdatedAt: time.Now().Add(-31 * 24 * time.Hour)}}
	if err := store.Save(old); err != nil {
		t.Fatal(err)
	}
	plan := prepareApplicationTest(t, app)
	finished := make(chan error, 1)
	const request = "retention-shutdown-request-123456"
	go func() {
		_, err := app.Apply(t.Context(), plan.PlanId, request)
		finished <- err
	}()
	awaitApplicationSignal(t, started)
	go app.Close()
	awaitApplicationSignal(t, app.Stopping())
	select {
	case err := <-finished:
		if !errors.Is(err, ErrPlanStale) {
			t.Fatal("shutdown admitted work after retention", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("canceled retention did not finish")
	}
	awaitApplicationSignal(t, app.Done())
	if record, err := store.ByRequest(request); err != nil || record != nil {
		t.Fatal("shutdown published acceptance", record, err)
	}
	if prepared.calls.Load() != 0 || prepared.closed.Load() != 1 || backend.closed.Load() != 1 || lock.held.Load() {
		t.Fatal("shutdown lost unaccepted authority ownership")
	}
}

func awaitApplicationSignal(t *testing.T, signal <-chan struct{}) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(5 * time.Second):
		t.Fatal("application lifecycle signal did not arrive")
	}
}

func hasApplicationDiagnostic(op Operation, code string) bool {
	for _, diagnostic := range op.Diagnostics {
		if diagnostic.Code == code {
			return true
		}
	}
	return false
}
