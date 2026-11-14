package usecase

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func TestValidateExistingUpdateTargetsRejectsEmptyPlans(t *testing.T) {
	t.Parallel()

	err := validateExistingUpdateTargets(nil)
	if err == nil || !strings.Contains(err.Error(), "update requires at least one planned target") {
		t.Fatalf("err = %v", err)
	}
}

func TestFinalizeExistingUpdateStateUpsertsRecord(t *testing.T) {
	t.Parallel()

	state := finalizeExistingUpdateState(ports.StateFile{}, domain.InstallationRecord{IntegrationID: "demo"})
	if len(state.Installations) != 1 || state.Installations[0].IntegrationID != "demo" {
		t.Fatalf("state = %+v", state)
	}
}

func TestExistingUpdateReportSortsTargets(t *testing.T) {
	t.Parallel()

	report := existingUpdateReport("op", "demo", []domain.TargetReport{
		{TargetID: "gemini"},
		{TargetID: "claude"},
	})
	if len(report.Targets) != 2 || report.Targets[0].TargetID != "claude" || report.Targets[1].TargetID != "gemini" {
		t.Fatalf("targets = %+v", report.Targets)
	}
}

func TestBuildExistingUpdateApplyInputCarriesMutationInputs(t *testing.T) {
	t.Parallel()

	input := buildExistingUpdateApplyInput(domain.InstallationRecord{IntegrationID: "demo"}, plannedExistingTarget{
		Plan: ports.AdapterPlan{ActionClass: "update"},
		Manifest: &domain.IntegrationManifest{
			Version: "1.2.3",
		},
		Resolved: &ports.ResolvedSource{Kind: "bundle"},
	})
	if input.Plan.ActionClass != "update" || input.Manifest.Version != "1.2.3" || input.ResolvedSource == nil || input.ResolvedSource.Kind != "bundle" || input.Record == nil || input.Record.IntegrationID != "demo" {
		t.Fatalf("input = %+v", input)
	}
}

func TestExistingUpdateVerifyRecordDelegatesProvisionalRecord(t *testing.T) {
	t.Parallel()

	record := domain.InstallationRecord{
		IntegrationID: "demo",
		Targets: map[domain.TargetID]domain.TargetInstallation{
			"gemini": {TargetID: "gemini"},
		},
	}
	target := plannedExistingTarget{
		TargetID: "gemini",
		Manifest: &domain.IntegrationManifest{
			Version: "2.0.0",
		},
		Resolved: &ports.ResolvedSource{Resolved: domain.ResolvedSourceRef{Value: "registry.example/demo@2.0.0"}},
	}
	got := existingUpdateVerifyRecord(record, target, ports.ApplyResult{})
	if got.ResolvedVersion != "2.0.0" || got.Targets["gemini"].TargetID != "gemini" {
		t.Fatalf("record = %+v", got)
	}
}

func TestDegradedExistingUpdateStateMarksTargetAndMetadata(t *testing.T) {
	t.Parallel()

	state := degradedExistingUpdateState(ports.StateFile{}, domain.InstallationRecord{
		IntegrationID: "demo",
		Targets:       map[domain.TargetID]domain.TargetInstallation{},
	}, plannedExistingTarget{
		TargetID: "gemini",
		Delivery: domain.Delivery{TargetID: "gemini"},
		Manifest: &domain.IntegrationManifest{
			Version:        "1.2.3",
			ResolvedRef:    domain.ResolvedSourceRef{Value: "registry.example/demo@1.2.3"},
			SourceDigest:   "source",
			ManifestDigest: "manifest",
		},
	}, "2026-04-10T20:00:00Z")
	if len(state.Installations) != 1 || state.Installations[0].ResolvedVersion != "1.2.3" || state.Installations[0].Targets["gemini"].State != domain.InstallDegraded {
		t.Fatalf("state = %+v", state)
	}
}

func TestExistingUpdateFailureErrorWrapsMutationApply(t *testing.T) {
	t.Parallel()

	err := existingUpdateFailureError("update failed after partial progress; degraded state persisted", errors.New("cause"))
	if err == nil || !strings.Contains(err.Error(), "update failed after partial progress; degraded state persisted") {
		t.Fatalf("err = %v", err)
	}
}

func TestNewExistingUpdateOperationRecordBuildsInProgressRecord(t *testing.T) {
	t.Parallel()

	record := newExistingUpdateOperationRecord("op", "demo", "2026-04-10T20:00:00Z")
	if record.OperationID != "op" || record.Type != "update" || record.IntegrationID != "demo" || record.Status != "in_progress" || record.StartedAt != "2026-04-10T20:00:00Z" {
		t.Fatalf("record = %+v", record)
	}
}

func TestLoadExistingUpdateRuntimeRejectsMissingInstallation(t *testing.T) {
	t.Parallel()

	_, err := loadExistingUpdateRuntime(ports.StateFile{}, "demo", "op", "2026-04-10T20:00:00Z", 1)
	if err == nil || !strings.Contains(err.Error(), "integration disappeared from state during apply: demo") {
		t.Fatalf("err = %v", err)
	}
}

func TestLoadExistingUpdateRuntimeBuildsRuntime(t *testing.T) {
	t.Parallel()

	runtime, err := loadExistingUpdateRuntime(ports.StateFile{
		Installations: []domain.InstallationRecord{{IntegrationID: "demo"}},
	}, "demo", "op", "2026-04-10T20:00:00Z", 2)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.operationID != "op" || runtime.startedAt != "2026-04-10T20:00:00Z" || cap(runtime.reportTargets) != 2 || runtime.nextRecord.IntegrationID != "demo" {
		t.Fatalf("runtime = %+v", runtime)
	}
}

func TestNewExistingUpdateOperationBuildsOperationAndRecord(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 10, 20, 0, 0, 0, time.FixedZone("EEST", 3*60*60))
	op := newExistingUpdateOperation("demo", now)
	record := op.record("demo")
	if op.operationID == "" || op.startedAt != "2026-04-10T17:00:00Z" {
		t.Fatalf("operation = %+v", op)
	}
	if record.OperationID != op.operationID || record.Type != "update" || record.IntegrationID != "demo" || record.StartedAt != op.startedAt {
		t.Fatalf("record = %+v", record)
	}
}

func TestExistingUpdateOperationStartAndFinishFailedUseJournal(t *testing.T) {
	t.Parallel()

	journal := &stubOperationJournal{}
	op := existingUpdateOperation{operationID: "op", startedAt: "2026-04-10T17:00:00Z"}
	if err := op.start(context.Background(), journal, "demo"); err != nil {
		t.Fatal(err)
	}
	if err := op.finishFailed(context.Background(), journal); err != nil {
		t.Fatal(err)
	}
	if len(journal.started) != 1 || journal.started[0].OperationID != "op" || journal.started[0].IntegrationID != "demo" {
		t.Fatalf("started = %+v", journal.started)
	}
	if len(journal.finished) != 1 || journal.finished[0] != "op:failed" {
		t.Fatalf("finished = %+v", journal.finished)
	}
}

type stubOperationJournal struct {
	started  []domain.OperationRecord
	finished []string
}

func (s *stubOperationJournal) Start(_ context.Context, record domain.OperationRecord) error {
	s.started = append(s.started, record)
	return nil
}

func (s *stubOperationJournal) AppendStep(context.Context, string, domain.JournalStep) error {
	return nil
}

func (s *stubOperationJournal) Finish(_ context.Context, operationID string, status string) error {
	s.finished = append(s.finished, operationID+":"+status)
	return nil
}

func (s *stubOperationJournal) ListOpen(context.Context) ([]domain.OperationRecord, error) {
	return nil, nil
}

func TestCommitExistingUpdatePersistsFinalizedState(t *testing.T) {
	t.Parallel()

	store := &stubUpdateStateStore{}
	journal := &stubOperationJournal{}
	svc := Service{
		StateStore: store,
		Journal:    journal,
	}
	err := svc.commitExistingUpdate(context.Background(), existingUpdateOperation{operationID: "op"}, existingUpdateRuntime{
		state:      ports.StateFile{},
		nextRecord: domain.InstallationRecord{IntegrationID: "demo"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(store.saved) != 1 || len(store.saved[0].Installations) != 1 || store.saved[0].Installations[0].IntegrationID != "demo" {
		t.Fatalf("saved = %+v", store.saved)
	}
	if len(journal.finished) != 1 || journal.finished[0] != "op:committed" {
		t.Fatalf("finished = %+v", journal.finished)
	}
}

type stubUpdateStateStore struct {
	saved []ports.StateFile
}

func (s *stubUpdateStateStore) Load(context.Context) (ports.StateFile, error) {
	return ports.StateFile{}, nil
}

func (s *stubUpdateStateStore) Save(_ context.Context, state ports.StateFile) error {
	s.saved = append(s.saved, state)
	return nil
}
