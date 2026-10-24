package management

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func retainedRecord(s *Server, id string, state OperationState) acceptance {
	now := time.Now().UTC()
	op := Operation{EnvironmentId: s.opts.EnvironmentID, ContextId: "previous-capture", Action: Install, OperationId: id, PlanId: "prior-plan-" + id, RequestId: "prior-request-" + id, State: state, Phase: "native", CreatedAt: now, UpdatedAt: now, Items: []OperationItem{}, Diagnostics: []Diagnostic{}}
	return acceptance{Version: 1, EnvironmentID: s.opts.EnvironmentID, IntentRoot: s.opts.IntentRoot, ContextID: op.ContextId, Fingerprint: digest(op.PlanId), Operation: op}
}
func saveRecord(t *testing.T, s *Server, record acceptance) {
	t.Helper()
	if err := s.store.save(record); err != nil {
		t.Fatal(err)
	}
}

func TestRestartRequiresFreshProofBeforeClearingUncertainty(t *testing.T) {
	var proof atomic.Bool
	backend := &testBackend{makePlan: func(input ChangeRequest, _ string) Prepared {
		outcome := knownSuccess()
		outcome.RecoverySettled = input.Action == Recover && proof.Load()
		return &testPrepared{apply: func(_ context.Context) (Outcome, error) { return outcome, nil }}
	}}
	s := newTestServer(t, "", "new-capture", backend)
	old := retainedRecord(s, "interrupted-original", Queued)
	saveRecord(t, s, old)
	op := applyTest(t, s, Plan{PlanId: old.Operation.PlanId}, old.Operation.RequestId)
	if op.State != Interrupted || op.Result != nil || op.ContextId != "previous-capture" {
		t.Fatal("restart invented a result or lost capture", op)
	}
	fresh := prepareTest(t, s, ChangeRequest{Action: Sync})
	var refused APIError
	requestJSON(t, s, "POST", "/api/v1/plans/"+fresh.PlanId+"/apply", ApplyRequest{RequestId: "new-mutation-123456"}, 409, &refused)
	if refused.Code != "RECOVERY_REQUIRED" {
		t.Fatal(refused)
	}
	recovery := prepareTest(t, s, ChangeRequest{Action: Recover, OperationId: old.Operation.OperationId})
	candidate := applyTest(t, s, recovery, "recovery-no-proof-123456")
	awaitTerminal(t, s, candidate.OperationId)
	retained, _ := s.store.byOperation(old.Operation.OperationId)
	if retained.Operation.ResolvedBy != "" {
		t.Fatal("return without proof cleared uncertainty")
	}
	second := retainedRecord(s, "intermediate-cleanup", Partial)
	second.Operation.Result = &OperationResult{Outcome: "partial", Mutated: true}
	saveRecord(t, s, second)
	local := retainedRecord(s, "local-interrupted", Interrupted)
	local.Operation.Action = LocalSkill
	local.Operation.LocalSkill = &LocalSkillIdentity{RootId: "user-skills", SkillName: "example"}
	saveRecord(t, s, local)
	proof.Store(true)
	recovery = prepareTest(t, s, ChangeRequest{Action: Recover})
	completed := applyTest(t, s, recovery, "recovery-with-proof-123456")
	awaitTerminal(t, s, completed.OperationId)
	for _, id := range []string{old.Operation.OperationId, second.Operation.OperationId} {
		record, err := s.store.byOperation(id)
		if err != nil || record.Operation.ResolvedBy != completed.OperationId {
			t.Fatalf("%s not linked by proof: %v", id, err)
		}
	}
	retained, _ = s.store.byOperation(old.Operation.OperationId)
	if retained.Operation.Result != nil || retained.Operation.State != Interrupted {
		t.Fatal("recovery rewrote original outcome")
	}
	var environment Environment
	requestJSON(t, s, "GET", "/api/v1/environment", nil, 200, &environment)
	if len(environment.PendingOperations) != 1 || environment.PendingOperations[0].OperationId != local.Operation.OperationId {
		t.Fatal("native proof hid local uncertainty", environment.PendingOperations)
	}
}

func TestRecoveryCannotChangeIntentRootOrCategory(t *testing.T) {
	s := newTestServer(t, "", "capture", nil)
	record := retainedRecord(s, "foreign-intent", Interrupted)
	record.IntentRoot = filepath.Join(t.TempDir(), "other-intent")
	saveRecord(t, s, record)
	var refusal APIError
	requestJSON(t, s, "POST", "/api/v1/plans", ChangeRequest{Action: Recover, OperationId: record.Operation.OperationId}, 409, &refusal)
	if refusal.Code != "RECOVERY_CONTEXT_CHANGED" {
		t.Fatal(refusal)
	}
	record.IntentRoot = s.opts.IntentRoot
	record.Operation.Action = LocalSkill
	record.Operation.LocalSkill = &LocalSkillIdentity{RootId: "user", SkillName: "skill"}
	saveRecord(t, s, record)
	requestJSON(t, s, "POST", "/api/v1/plans", ChangeRequest{Action: Recover, OperationId: record.Operation.OperationId}, 409, &refusal)
	if refusal.Code != "LOCAL_RECOVERY_REQUIRED" {
		t.Fatal(refusal)
	}
}

func TestLocalRestoreLinksOnlyItsExactRetainedIdentity(t *testing.T) {
	var restoreID string
	backend := &testBackend{makePlan: func(ChangeRequest, string) Prepared {
		outcome := knownSuccess()
		outcome.ReconciledOperationIDs = []string{restoreID}
		return &testPrepared{apply: func(context.Context) (Outcome, error) { return outcome, nil }}
	}}
	s := newTestServer(t, "", "capture", backend)
	old := retainedRecord(s, "local-before", Interrupted)
	old.Operation.Action = LocalSkill
	old.Operation.LocalSkill = &LocalSkillIdentity{RootId: "user", SkillName: "mine"}
	saveRecord(t, s, old)
	restoreID = old.Operation.OperationId
	input := LocalSkillRequest{Action: Restore, RootId: "user", SkillName: "other", RestoreOperationId: restoreID}
	var refusal APIError
	requestJSON(t, s, "POST", "/api/v1/skills/local/plans", input, 409, &refusal)
	if refusal.Code != "RECOVERY_CONTEXT_MISMATCH" {
		t.Fatal(refusal)
	}
	input.SkillName = "mine"
	var plan Plan
	requestJSON(t, s, "POST", "/api/v1/skills/local/plans", input, 200, &plan)
	op := applyTest(t, s, plan, "local-restore-123456")
	awaitTerminal(t, s, op.OperationId)
	record, err := s.store.byOperation(restoreID)
	if err != nil || record.Operation.ResolvedBy != op.OperationId {
		t.Fatal("local restore was not linked", err)
	}
}

func TestCancelledPartialEffectsRemainPendingAndNeverAgeOut(t *testing.T) {
	now := time.Now().UTC()
	for _, result := range []*OperationResult{
		{Outcome: "partial", Mutated: true},
		{Outcome: "unchanged", NextActions: []NextAction{{Kind: "recover", Reason: "cleanup required"}}},
	} {
		op := Operation{State: Cancelled, Result: result, UpdatedAt: now.Add(-90 * 24 * time.Hour)}
		if !pendingOperation(op) || prunable(op, now) {
			t.Fatal("cancelled unresolved work released fence")
		}
		op.ResolvedBy = "reviewed-recovery"
		if pendingOperation(op) || !prunable(op, now) {
			t.Fatal("resolved work did not become eligible for pruning")
		}
	}
}

func TestMalformedHistoryRefusesAcceptanceBeforeEffects(t *testing.T) {
	p := &testPrepared{}
	s := newTestServer(t, "", "capture", &testBackend{makePlan: func(ChangeRequest, string) Prepared { return p }})
	review := prepareTest(t, s, ChangeRequest{Action: Sync})
	if err := s.store.ensureDirectory(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(s.store.root, strings.Repeat("a", 64)+".json"), []byte("corrupt"), 0600); err != nil {
		t.Fatal(err)
	}
	var refusal APIError
	requestJSON(t, s, "POST", "/api/v1/plans/"+review.PlanId+"/apply", ApplyRequest{RequestId: "corrupt-history-123456"}, 503, &refusal)
	if refusal.Code != "OBSERVATION_UNAVAILABLE" || p.calls.Load() != 0 {
		t.Fatal("corrupt history allowed effects", refusal)
	}
}

func TestNilResultCannotClaimKnownNoEffects(t *testing.T) {
	op := Operation{State: Queued}
	for _, outcome := range []Outcome{{State: Succeeded}, {State: Failed}, {State: Cancelled}, {State: Running}} {
		result := finishOperation(op, outcome, nil, false)
		if result.State != Interrupted || result.Result != nil {
			t.Fatal("missing result treated as known", result)
		}
	}
}
