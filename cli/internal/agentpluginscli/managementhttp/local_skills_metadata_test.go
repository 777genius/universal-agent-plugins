package managementhttp

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/management"
)

func TestLocalRestoreReviewRetainsSelectedMetadataAfterRestart(t *testing.T) {
	root := t.TempDir()
	var original *management.Outcome
	var action string
	backend := &testBackend{makePlan: func(_ management.ChangeRequest, id string) management.Prepared {
		outcome := knownSuccess()
		outcome.Result.LocalSkillBackup = &management.LocalSkillBackup{
			OperationId: id, RootId: "user", SkillName: "mine",
			Path: filepath.Join(root, "backups", action), OriginalAction: management.LocalSkillBackupOriginalAction(action),
		}
		original = &outcome
		return &testPrepared{apply: func(context.Context) (management.Outcome, error) { return outcome, nil }}
	}}
	s := newTestServer(t, root, "before-restart", backend)
	expected := map[string]management.LocalSkillBackup{}
	for _, nextAction := range []string{"upsert", "delete"} {
		action = nextAction
		var review management.Plan
		requestJSON(t, s, "POST", "/api/v1/skills/local/plans", management.LocalSkillRequest{
			Action: management.LocalSkillRequestAction(action), RootId: "user", SkillName: "mine",
		}, 200, &review)
		accepted := applyTest(t, s, review, "metadata-request-"+action)
		operation := awaitTerminal(t, s, accepted.OperationId)
		if operation.State != management.Succeeded || operation.Result == nil || operation.Result.LocalSkillBackup == nil {
			t.Fatal("local operation did not retain its backup", operation)
		}
		point := operation.Result.LocalSkillBackup
		if point.RecordedAt == nil || !point.RecordedAt.Equal(operation.UpdatedAt) {
			t.Fatal("backup did not retain the actual completed observation time", point)
		}
		if original.Result.LocalSkillBackup.RecordedAt != nil {
			t.Fatal("recording metadata mutated the backend outcome")
		}
		expected[point.OperationId] = *point
	}
	// Old records had no backup metadata. Even this operation ID must not be
	// interpreted as proof of the original action; only its known time is used.
	legacy := retainedRecord(s, "legacy-delete-looking-id", management.Succeeded)
	legacy.Operation.Action = management.LocalSkill
	legacy.Operation.LocalSkill = &management.LocalSkillIdentity{RootId: "user", SkillName: "mine"}
	legacy.Operation.UpdatedAt = time.Now().UTC().Add(-time.Hour)
	legacy.Operation.Result = &management.OperationResult{Outcome: "completed", Mutated: true, LocalSkillBackup: &management.LocalSkillBackup{
		OperationId: legacy.Operation.OperationId, RootId: "user", SkillName: "mine", Path: filepath.Join(root, "backups", "legacy"),
	}}
	saveRecord(t, s, legacy)
	legacyPoint := *legacy.Operation.Result.LocalSkillBackup
	legacyPoint.RecordedAt = &legacy.Operation.UpdatedAt
	expected[legacyPoint.OperationId] = legacyPoint
	s.Close()

	prepared := &testPrepared{}
	next := newTestServer(t, root, "after-restart", &testBackend{makePlan: func(management.ChangeRequest, string) management.Prepared { return prepared }})
	var environment management.Environment
	requestJSON(t, next, "GET", "/api/v1/environment", nil, 200, &environment)
	if len(environment.LocalSkillBackups) != len(expected) {
		t.Fatal("restart lost same-name restore points", environment.LocalSkillBackups)
	}
	var selected management.Plan
	for _, point := range environment.LocalSkillBackups {
		if want, ok := expected[point.OperationId]; !ok || !reflect.DeepEqual(point, want) {
			t.Fatalf("restore point lost durable metadata: got %+v want %+v", point, want)
		}
		input := management.LocalSkillRequest{Action: management.Restore, RootId: point.RootId, SkillName: point.SkillName, RestoreOperationId: point.OperationId}
		selected = management.Plan{}
		requestJSON(t, next, "POST", "/api/v1/skills/local/plans", input, 200, &selected)
		if selected.RestorePoint == nil || !reflect.DeepEqual(*selected.RestorePoint, point) {
			t.Fatalf("review described a different point: got %+v want %+v", selected.RestorePoint, point)
		}
	}
	// Presentational metadata does not grant permission to apply stale authority.
	prepared.checkErr = management.ErrPlanStale
	var refusal management.APIError
	requestJSON(t, next, "POST", "/api/v1/plans/"+selected.PlanId+"/apply", management.ApplyRequest{RequestId: "stale-restore-metadata-123456"}, 409, &refusal)
	if refusal.Code != "PLAN_STALE" || prepared.calls.Load() != 0 {
		t.Fatal("restore metadata bypassed the retained backend check", refusal)
	}
}
