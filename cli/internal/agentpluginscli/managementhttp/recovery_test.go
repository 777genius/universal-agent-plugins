package managementhttp

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/management"
)

func digest(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func retainedRecord(s *testServer, id string, state management.OperationState) management.Observation {
	now := time.Now().UTC()
	op := management.Operation{EnvironmentId: s.opts.EnvironmentID, ContextId: "previous-capture", Action: management.Install, OperationId: id, PlanId: "prior-plan-" + id, RequestId: "prior-request-" + id, State: state, Phase: "native", CreatedAt: now, UpdatedAt: now, Items: []management.OperationItem{}, Diagnostics: []management.Diagnostic{}}
	return management.Observation{Version: 1, EnvironmentID: s.opts.EnvironmentID, IntentRoot: s.opts.IntentRoot, ContextID: op.ContextId, Fingerprint: digest(op.PlanId), Operation: op}
}
func saveRecord(t *testing.T, s *testServer, record management.Observation) {
	t.Helper()
	if err := s.store.Save(record); err != nil {
		t.Fatal(err)
	}
}

func TestRestartRequiresFreshProofBeforeClearingUncertainty(t *testing.T) {
	var proof atomic.Bool
	backend := &testBackend{makePlan: func(input management.ChangeRequest, _ string) management.Prepared {
		outcome := knownSuccess()
		outcome.RecoverySettled = input.Action == management.Recover && proof.Load()
		return &testPrepared{apply: func(_ context.Context) (management.Outcome, error) { return outcome, nil }}
	}}
	s := newTestServer(t, "", "new-capture", backend)
	old := retainedRecord(s, "interrupted-original", management.Queued)
	saveRecord(t, s, old)
	op := applyTest(t, s, management.Plan{PlanId: old.Operation.PlanId}, old.Operation.RequestId)
	if op.State != management.Interrupted || op.Result != nil || op.ContextId != "previous-capture" {
		t.Fatal("restart invented a result or lost capture", op)
	}
	fresh := prepareTest(t, s, management.ChangeRequest{Action: management.Sync})
	var refused management.APIError
	requestJSON(t, s, "POST", "/api/v1/plans/"+fresh.PlanId+"/apply", management.ApplyRequest{RequestId: "new-mutation-123456"}, 409, &refused)
	if refused.Code != "RECOVERY_REQUIRED" {
		t.Fatal(refused)
	}
	recovery := prepareTest(t, s, management.ChangeRequest{Action: management.Recover, OperationId: old.Operation.OperationId})
	candidate := applyTest(t, s, recovery, "recovery-no-proof-123456")
	awaitTerminal(t, s, candidate.OperationId)
	retained, _ := s.store.ByOperation(old.Operation.OperationId)
	if retained.Operation.ResolvedBy != "" {
		t.Fatal("return without proof cleared uncertainty")
	}
	second := retainedRecord(s, "intermediate-cleanup", management.Partial)
	second.Operation.Result = &management.OperationResult{Outcome: "partial", Mutated: true}
	saveRecord(t, s, second)
	local := retainedRecord(s, "local-interrupted", management.Interrupted)
	local.Operation.Action = management.LocalSkill
	local.Operation.LocalSkill = &management.LocalSkillIdentity{RootId: "user-skills", SkillName: "example"}
	saveRecord(t, s, local)
	proof.Store(true)
	recovery = prepareTest(t, s, management.ChangeRequest{Action: management.Recover})
	completed := applyTest(t, s, recovery, "recovery-with-proof-123456")
	awaitTerminal(t, s, completed.OperationId)
	for _, id := range []string{old.Operation.OperationId, second.Operation.OperationId} {
		record, err := s.store.ByOperation(id)
		if err != nil || record.Operation.ResolvedBy != completed.OperationId {
			t.Fatalf("%s not linked by proof: %v", id, err)
		}
	}
	retained, _ = s.store.ByOperation(old.Operation.OperationId)
	if retained.Operation.Result != nil || retained.Operation.State != management.Interrupted {
		t.Fatal("recovery rewrote original outcome")
	}
	var environment management.Environment
	requestJSON(t, s, "GET", "/api/v1/environment", nil, 200, &environment)
	if len(environment.PendingOperations) != 1 || environment.PendingOperations[0].OperationId != local.Operation.OperationId {
		t.Fatal("native proof hid local uncertainty", environment.PendingOperations)
	}
}

func TestRecoveryCannotChangeIntentRootOrCategory(t *testing.T) {
	s := newTestServer(t, "", "capture", nil)
	record := retainedRecord(s, "foreign-intent", management.Interrupted)
	record.IntentRoot = filepath.Join(t.TempDir(), "other-intent")
	saveRecord(t, s, record)
	var refusal management.APIError
	requestJSON(t, s, "POST", "/api/v1/plans", management.ChangeRequest{Action: management.Recover, OperationId: record.Operation.OperationId}, 409, &refusal)
	if refusal.Code != "RECOVERY_CONTEXT_CHANGED" {
		t.Fatal(refusal)
	}
	record.IntentRoot = s.opts.IntentRoot
	record.Operation.Action = management.LocalSkill
	record.Operation.LocalSkill = &management.LocalSkillIdentity{RootId: "user", SkillName: "skill"}
	saveRecord(t, s, record)
	requestJSON(t, s, "POST", "/api/v1/plans", management.ChangeRequest{Action: management.Recover, OperationId: record.Operation.OperationId}, 409, &refusal)
	if refusal.Code != "LOCAL_RECOVERY_REQUIRED" {
		t.Fatal(refusal)
	}
}

func TestLocalRestoreLinksOnlyItsExactRetainedIdentity(t *testing.T) {
	var restoreID string
	backend := &testBackend{makePlan: func(management.ChangeRequest, string) management.Prepared {
		outcome := knownSuccess()
		outcome.ReconciledOperationIDs = []string{restoreID}
		return &testPrepared{apply: func(context.Context) (management.Outcome, error) { return outcome, nil }}
	}}
	s := newTestServer(t, "", "capture", backend)
	old := retainedRecord(s, "local-before", management.Interrupted)
	old.Operation.Action = management.LocalSkill
	old.Operation.LocalSkill = &management.LocalSkillIdentity{RootId: "user", SkillName: "mine"}
	saveRecord(t, s, old)
	restoreID = old.Operation.OperationId
	input := management.LocalSkillRequest{Action: management.Restore, RootId: "user", SkillName: "other", RestoreOperationId: restoreID}
	var refusal management.APIError
	requestJSON(t, s, "POST", "/api/v1/skills/local/plans", input, 409, &refusal)
	if refusal.Code != "RECOVERY_CONTEXT_MISMATCH" {
		t.Fatal(refusal)
	}
	input.SkillName = "mine"
	var plan management.Plan
	requestJSON(t, s, "POST", "/api/v1/skills/local/plans", input, 200, &plan)
	op := applyTest(t, s, plan, "local-restore-123456")
	awaitTerminal(t, s, op.OperationId)
	record, err := s.store.ByOperation(restoreID)
	if err != nil || record.Operation.ResolvedBy != op.OperationId {
		t.Fatal("local restore was not linked", err)
	}
}

func TestMalformedHistoryRefusesAcceptanceBeforeEffects(t *testing.T) {
	p := &testPrepared{}
	s := newTestServer(t, "", "capture", &testBackend{makePlan: func(management.ChangeRequest, string) management.Prepared { return p }})
	review := prepareTest(t, s, management.ChangeRequest{Action: management.Sync})
	directory := filepath.Join(s.root, "management", digest(s.opts.EnvironmentID))
	if err := os.MkdirAll(directory, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, strings.Repeat("a", 64)+".json"), []byte("corrupt"), 0600); err != nil {
		t.Fatal(err)
	}
	var refusal management.APIError
	requestJSON(t, s, "POST", "/api/v1/plans/"+review.PlanId+"/apply", management.ApplyRequest{RequestId: "corrupt-history-123456"}, 503, &refusal)
	if refusal.Code != "OBSERVATION_UNAVAILABLE" || p.calls.Load() != 0 {
		t.Fatal("corrupt history allowed effects", refusal)
	}
}

func TestSuccessfulLocalBackupRemainsVisibleAfterHostRestart(t *testing.T) {
	root := t.TempDir()
	s := newTestServer(t, root, "capture-before", nil)
	old := retainedRecord(s, "successful-delete", management.Succeeded)
	old.Operation.Action = management.LocalSkill
	old.Operation.LocalSkill = &management.LocalSkillIdentity{RootId: "user", SkillName: "mine"}
	old.Operation.Result = &management.OperationResult{Outcome: "completed", Mutated: true, LocalSkillBackup: &management.LocalSkillBackup{OperationId: old.Operation.OperationId, RootId: "user", SkillName: "mine", Path: filepath.Join(root, "private-backup")}}
	saveRecord(t, s, old)
	s.Close()
	next := newTestServer(t, root, "capture-after", nil)
	var environment management.Environment
	requestJSON(t, next, "GET", "/api/v1/environment", nil, 200, &environment)
	if len(environment.PendingOperations) != 0 || len(environment.LocalSkillBackups) != 1 || environment.LocalSkillBackups[0].OperationId != old.Operation.OperationId {
		t.Fatal("successful undo point disappeared across restart", environment)
	}
}
