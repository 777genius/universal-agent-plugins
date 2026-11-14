package usecase

import (
	"context"
	"sort"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func (s Service) applyRepairExisting(ctx context.Context, record domain.InstallationRecord, planned []plannedExistingTarget) (domain.Report, error) {
	if err := validateExistingRepairTargets(planned); err != nil {
		return domain.Report{}, err
	}
	defer cleanupPlannedExisting(planned)
	operationID := operationID(actionNamePrefix("repair_drift"), record.IntegrationID, s.now())
	unlock, err := s.LockManager.Acquire(ctx, "state")
	if err != nil {
		return domain.Report{}, domain.NewError(domain.ErrLockAcquire, "acquire integrationctl state lock", err)
	}
	defer func() { _ = unlock() }()

	startedAt := s.now().UTC().Format(time.RFC3339)
	if err := s.Journal.Start(ctx, domain.OperationRecord{
		OperationID:   operationID,
		Type:          "repair",
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
	runtime := newExistingRepairRuntime(operationID, startedAt, state, items, len(planned))
	for _, target := range planned {
		persisted, err := s.applyRepairedExistingTarget(ctx, record, target, &runtime)
		if err != nil {
			committed = persisted
			return domain.Report{}, err
		}
	}

	runtime.state = finalizeExistingRepairState(runtime.state, runtime.nextRecord)
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
	return existingRepairReport(operationID, record.IntegrationID, runtime.reportTargets), nil
}

func validateExistingRepairTargets(planned []plannedExistingTarget) error {
	if len(planned) == 0 {
		return domain.NewError(domain.ErrMutationApply, "repair requires at least one planned target", nil)
	}
	if err := ensureExistingTargetsNotBlocking(planned); err != nil {
		return err
	}
	return ensureExistingTargetsResolved(planned, "repair")
}

func newExistingRepairRuntime(operationID, startedAt string, state ports.StateFile, items domain.InstallationRecord, plannedCount int) existingRepairRuntime {
	return existingRepairRuntime{
		operationID:   operationID,
		startedAt:     startedAt,
		state:         state,
		nextRecord:    cloneInstallationRecord(items),
		reportTargets: make([]domain.TargetReport, 0, plannedCount),
	}
}

func finalizeExistingRepairState(state ports.StateFile, nextRecord domain.InstallationRecord) ports.StateFile {
	state.Installations = upsertInstallation(state.Installations, nextRecord)
	return state
}

func existingRepairReport(operationID, integrationID string, targets []domain.TargetReport) domain.Report {
	sort.Slice(targets, func(i, j int) bool { return targets[i].TargetID < targets[j].TargetID })
	return domain.Report{
		OperationID: operationID,
		Summary:     summaryForExisting("repair_drift", integrationID),
		Targets:     targets,
	}
}
