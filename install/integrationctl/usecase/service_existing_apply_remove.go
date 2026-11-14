package usecase

import (
	"context"
	"sort"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func (s Service) applyRemoveExisting(ctx context.Context, record domain.InstallationRecord, planned []plannedExistingTarget) (domain.Report, error) {
	if err := validateExistingRemoveTargets(planned); err != nil {
		return domain.Report{}, err
	}
	defer cleanupPlannedExisting(planned)
	operationID := operationID(actionNamePrefix("remove_orphaned_target"), record.IntegrationID, s.now())
	unlock, err := s.LockManager.Acquire(ctx, "state")
	if err != nil {
		return domain.Report{}, domain.NewError(domain.ErrLockAcquire, "acquire integrationctl state lock", err)
	}
	defer func() { _ = unlock() }()

	startedAt := s.now().UTC().Format(time.RFC3339)
	if err := s.Journal.Start(ctx, domain.OperationRecord{
		OperationID:   operationID,
		Type:          "remove",
		IntegrationID: record.IntegrationID,
		Status:        "in_progress",
		StartedAt:     startedAt,
	}); err != nil {
		return domain.Report{}, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = s.Journal.Finish(ctx, operationID, "failed")
		}
	}()

	state, err := s.StateStore.Load(ctx)
	if err != nil {
		return domain.Report{}, err
	}
	items, found := findInstallationMutable(state.Installations, record.IntegrationID)
	if !found {
		return domain.Report{}, domain.NewError(domain.ErrStateConflict, "integration disappeared from state during apply: "+record.IntegrationID, nil)
	}
	runtime := newExistingRemoveRuntime(operationID, startedAt, state, items, len(planned))
	for _, target := range planned {
		persisted, err := s.applyRemovedExistingTarget(ctx, record, target, &runtime)
		if err != nil {
			committed = persisted
			return domain.Report{}, err
		}
	}
	runtime.state = finalizeExistingRemoveState(runtime.state, runtime.nextRecord, startedAt)
	if err := s.StateStore.Save(ctx, runtime.state); err != nil {
		return domain.Report{}, err
	}
	if err := s.Journal.AppendStep(ctx, operationID, domain.JournalStep{Target: "state", Action: "persist_state", Status: "ok"}); err != nil {
		return domain.Report{}, err
	}
	if err := s.Journal.Finish(ctx, operationID, "committed"); err != nil {
		return domain.Report{}, err
	}
	committed = true
	return existingRemoveReport(operationID, record.IntegrationID, runtime.reportTargets), nil
}

func validateExistingRemoveTargets(planned []plannedExistingTarget) error {
	if len(planned) == 0 {
		return domain.NewError(domain.ErrMutationApply, "remove requires at least one planned target", nil)
	}
	return ensureExistingTargetsNotBlocking(planned)
}

func newExistingRemoveRuntime(operationID, startedAt string, state ports.StateFile, items domain.InstallationRecord, plannedCount int) existingRemoveRuntime {
	return existingRemoveRuntime{
		operationID:   operationID,
		startedAt:     startedAt,
		state:         state,
		nextRecord:    cloneInstallationRecord(items),
		removed:       make([]removedExistingTarget, 0, plannedCount),
		reportTargets: make([]domain.TargetReport, 0, plannedCount),
	}
}

func finalizeExistingRemoveState(state ports.StateFile, nextRecord domain.InstallationRecord, startedAt string) ports.StateFile {
	if len(nextRecord.Targets) == 0 {
		state.Installations = removeInstallation(state.Installations, nextRecord.IntegrationID)
		return state
	}
	nextRecord.LastCheckedAt = startedAt
	nextRecord.LastUpdatedAt = startedAt
	state.Installations = upsertInstallation(state.Installations, nextRecord)
	return state
}

func existingRemoveReport(operationID, integrationID string, targets []domain.TargetReport) domain.Report {
	sort.Slice(targets, func(i, j int) bool { return targets[i].TargetID < targets[j].TargetID })
	return domain.Report{
		OperationID: operationID,
		Summary:     summaryForExisting("remove_orphaned_target", integrationID),
		Targets:     targets,
	}
}
