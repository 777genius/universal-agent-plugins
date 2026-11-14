package usecase

import (
	"context"
	"sort"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type existingUpdateOperation struct {
	operationID string
	startedAt   string
}

func (s Service) applyUpdateExisting(ctx context.Context, record domain.InstallationRecord, planned []plannedExistingTarget) (domain.Report, error) {
	if err := validateExistingUpdateTargets(planned); err != nil {
		return domain.Report{}, err
	}
	defer cleanupPlannedExisting(planned)
	operation := newExistingUpdateOperation(record.IntegrationID, s.now())
	unlock, err := s.LockManager.Acquire(ctx, "state")
	if err != nil {
		return domain.Report{}, domain.NewError(domain.ErrLockAcquire, "acquire integrationctl state lock", err)
	}
	defer func() { _ = unlock() }()

	if err := operation.start(ctx, s.Journal, record.IntegrationID); err != nil {
		return domain.Report{}, err
	}
	committed := false
	defer finishExistingUpdateOnFailure(ctx, s.Journal, operation, &committed)

	state, err := s.StateStore.Load(ctx)
	if err != nil {
		return domain.Report{}, err
	}
	runtime, err := loadExistingUpdateRuntime(state, record.IntegrationID, operation.operationID, operation.startedAt, len(planned))
	if err != nil {
		return domain.Report{}, err
	}
	for _, target := range planned {
		persisted, err := s.applyUpdatedExistingTarget(ctx, record, target, &runtime)
		if err != nil {
			committed = persisted
			return domain.Report{}, err
		}
	}

	if err := s.commitExistingUpdate(ctx, operation, runtime); err != nil {
		return domain.Report{}, err
	}
	committed = true
	return existingUpdateReport(operation.operationID, record.IntegrationID, runtime.reportTargets), nil
}

func newExistingUpdateOperation(integrationID string, now time.Time) existingUpdateOperation {
	startedAt := now.UTC().Format(time.RFC3339)
	return existingUpdateOperation{
		operationID: operationID(actionNamePrefix("update_version"), integrationID, now),
		startedAt:   startedAt,
	}
}

func (op existingUpdateOperation) record(integrationID string) domain.OperationRecord {
	return newExistingUpdateOperationRecord(op.operationID, integrationID, op.startedAt)
}

func (op existingUpdateOperation) start(ctx context.Context, journal ports.OperationJournal, integrationID string) error {
	return journal.Start(ctx, op.record(integrationID))
}

func (op existingUpdateOperation) finishFailed(ctx context.Context, journal ports.OperationJournal) error {
	return journal.Finish(ctx, op.operationID, "failed")
}

func finishExistingUpdateOnFailure(ctx context.Context, journal ports.OperationJournal, operation existingUpdateOperation, committed *bool) {
	if *committed {
		return
	}
	_ = operation.finishFailed(ctx, journal)
}

func newExistingUpdateOperationRecord(operationID, integrationID, startedAt string) domain.OperationRecord {
	return domain.OperationRecord{
		OperationID:   operationID,
		Type:          "update",
		IntegrationID: integrationID,
		Status:        "in_progress",
		StartedAt:     startedAt,
	}
}

func validateExistingUpdateTargets(planned []plannedExistingTarget) error {
	if len(planned) == 0 {
		return domain.NewError(domain.ErrMutationApply, "update requires at least one planned target", nil)
	}
	if err := ensureExistingTargetsNotBlocking(planned); err != nil {
		return err
	}
	return ensureExistingTargetsResolved(planned, "update")
}

func newExistingUpdateRuntime(operationID, startedAt string, state ports.StateFile, items domain.InstallationRecord, plannedCount int) existingUpdateRuntime {
	return existingUpdateRuntime{
		operationID:   operationID,
		startedAt:     startedAt,
		state:         state,
		nextRecord:    cloneInstallationRecord(items),
		reportTargets: make([]domain.TargetReport, 0, plannedCount),
	}
}

func loadExistingUpdateRuntime(state ports.StateFile, integrationID, operationID, startedAt string, plannedCount int) (existingUpdateRuntime, error) {
	items, found := findInstallationMutable(state.Installations, integrationID)
	if !found {
		return existingUpdateRuntime{}, domain.NewError(domain.ErrStateConflict, "integration disappeared from state during apply: "+integrationID, nil)
	}
	return newExistingUpdateRuntime(operationID, startedAt, state, items, plannedCount), nil
}

func finalizeExistingUpdateState(state ports.StateFile, nextRecord domain.InstallationRecord) ports.StateFile {
	state.Installations = upsertInstallation(state.Installations, nextRecord)
	return state
}

func (s Service) commitExistingUpdate(ctx context.Context, operation existingUpdateOperation, runtime existingUpdateRuntime) error {
	return s.persistExistingUpdateCommittedState(ctx, operation.operationID, finalizeExistingUpdateState(runtime.state, runtime.nextRecord))
}

func (s Service) persistExistingUpdateCommittedState(ctx context.Context, operationID string, state ports.StateFile) error {
	if err := s.StateStore.Save(ctx, state); err != nil {
		return err
	}
	if err := s.Journal.AppendStep(ctx, operationID, domain.JournalStep{Target: "state", Action: "persist_state", Status: "ok"}); err != nil {
		return err
	}
	return s.Journal.Finish(ctx, operationID, "committed")
}

func existingUpdateReport(operationID, integrationID string, targets []domain.TargetReport) domain.Report {
	sort.Slice(targets, func(i, j int) bool { return targets[i].TargetID < targets[j].TargetID })
	return domain.Report{
		OperationID: operationID,
		Summary:     summaryForExisting("update_version", integrationID),
		Targets:     targets,
	}
}
