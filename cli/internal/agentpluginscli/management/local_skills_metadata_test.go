package management

import (
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestLocalRestoreReviewRetainsSelectedMetadataAfterRestart(t *testing.T) {
	root := t.TempDir()
	s := newTestServer(t, root, "before-restart", nil)
	expected := map[string]LocalSkillBackup{}
	for _, action := range []string{"upsert", "delete"} {
		record := retainedRecord(s, "same-skill-"+action, Queued)
		record.Operation.Action = LocalSkill
		record.Operation.LocalSkill = &LocalSkillIdentity{RootId: "user", SkillName: "mine"}
		outcome := knownSuccess()
		outcome.Result.LocalSkillBackup = &LocalSkillBackup{
			OperationId: record.Operation.OperationId, RootId: "user", SkillName: "mine",
			Path: filepath.Join(root, "backups", action), OriginalAction: LocalSkillBackupOriginalAction(action),
		}
		record.Operation = finishOperation(record.Operation, outcome, nil, false)
		point := record.Operation.Result.LocalSkillBackup
		if point.RecordedAt == nil || !point.RecordedAt.Equal(record.Operation.UpdatedAt) {
			t.Fatal("backup did not retain the actual completed observation time", point)
		}
		if outcome.Result.LocalSkillBackup.RecordedAt != nil {
			t.Fatal("recording metadata mutated the backend outcome")
		}
		expected[point.OperationId] = *point
		saveRecord(t, s, record)
	}
	// Old records had no backup metadata. Even this operation ID must not be
	// interpreted as proof of the original action; only its known time is used.
	legacy := retainedRecord(s, "legacy-delete-looking-id", Succeeded)
	legacy.Operation.Action = LocalSkill
	legacy.Operation.LocalSkill = &LocalSkillIdentity{RootId: "user", SkillName: "mine"}
	legacy.Operation.UpdatedAt = time.Now().UTC().Add(-time.Hour)
	legacy.Operation.Result = &OperationResult{Outcome: "completed", Mutated: true, LocalSkillBackup: &LocalSkillBackup{
		OperationId: legacy.Operation.OperationId, RootId: "user", SkillName: "mine", Path: filepath.Join(root, "backups", "legacy"),
	}}
	saveRecord(t, s, legacy)
	legacyPoint := *legacy.Operation.Result.LocalSkillBackup
	legacyPoint.RecordedAt = &legacy.Operation.UpdatedAt
	expected[legacyPoint.OperationId] = legacyPoint
	s.Close()

	prepared := &testPrepared{}
	next := newTestServer(t, root, "after-restart", &testBackend{makePlan: func(ChangeRequest, string) Prepared { return prepared }})
	var environment Environment
	requestJSON(t, next, "GET", "/api/v1/environment", nil, 200, &environment)
	if len(environment.LocalSkillBackups) != len(expected) {
		t.Fatal("restart lost same-name restore points", environment.LocalSkillBackups)
	}
	var selected Plan
	for _, point := range environment.LocalSkillBackups {
		if want, ok := expected[point.OperationId]; !ok || !reflect.DeepEqual(point, want) {
			t.Fatalf("restore point lost durable metadata: got %+v want %+v", point, want)
		}
		input := LocalSkillRequest{Action: Restore, RootId: point.RootId, SkillName: point.SkillName, RestoreOperationId: point.OperationId}
		selected = Plan{}
		requestJSON(t, next, "POST", "/api/v1/skills/local/plans", input, 200, &selected)
		if selected.RestorePoint == nil || !reflect.DeepEqual(*selected.RestorePoint, point) {
			t.Fatalf("review described a different point: got %+v want %+v", selected.RestorePoint, point)
		}
	}
	// Presentational metadata does not grant permission to apply stale authority.
	prepared.checkErr = ErrPlanStale
	var refusal APIError
	requestJSON(t, next, "POST", "/api/v1/plans/"+selected.PlanId+"/apply", ApplyRequest{RequestId: "stale-restore-metadata-123456"}, 409, &refusal)
	if refusal.Code != "PLAN_STALE" || prepared.calls.Load() != 0 {
		t.Fatal("restore metadata bypassed the retained backend check", refusal)
	}
}

func TestLocalRestorePointMetadataRequiresMatchingIdentity(t *testing.T) {
	op := Operation{Action: LocalSkill, OperationId: "original", LocalSkill: &LocalSkillIdentity{RootId: "user", SkillName: "mine"}, Result: &OperationResult{LocalSkillBackup: &LocalSkillBackup{
		OperationId: "original", RootId: "user", SkillName: "mine", Path: "/private/backup",
	}}}
	point := localSkillRestorePoint(op)
	if point == nil || point.RecordedAt != nil || point.OriginalAction != "" {
		t.Fatal("missing metadata was invented", point)
	}
	for _, modify := range []func(*LocalSkillBackup){
		func(p *LocalSkillBackup) { p.OperationId = "different-operation" },
		func(p *LocalSkillBackup) { p.RootId = "different-root" },
		func(p *LocalSkillBackup) { p.SkillName = "different-skill" },
	} {
		backup := *op.Result.LocalSkillBackup
		modify(&backup)
		mismatch := op
		mismatch.Result = &OperationResult{LocalSkillBackup: &backup}
		if got := localSkillRestorePoint(mismatch); got != nil {
			t.Fatal("unrelated metadata was presented as the selected restore point", got)
		}
	}
}
