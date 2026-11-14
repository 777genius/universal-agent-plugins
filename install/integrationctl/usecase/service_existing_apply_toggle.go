package usecase

import (
	"context"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
)

func (s Service) applyToggleExisting(ctx context.Context, record domain.InstallationRecord, action string, planned []plannedExistingTarget) (domain.Report, error) {
	target, err := validateSingleExistingToggleTarget(planned)
	if err != nil {
		return domain.Report{}, err
	}
	defer cleanupPlannedExisting(planned)
	if err := ensureExistingTogglePlanAllowed(target); err != nil {
		return domain.Report{}, err
	}

	operationID := operationID(actionNamePrefix(action), record.IntegrationID, s.now())
	unlock, err := s.LockManager.Acquire(ctx, "state")
	if err != nil {
		return domain.Report{}, domain.NewError(domain.ErrLockAcquire, "acquire integrationctl state lock", err)
	}
	defer func() { _ = unlock() }()

	startedAt := s.now().UTC().Format(time.RFC3339)
	if err := s.Journal.Start(ctx, domain.OperationRecord{
		OperationID:   operationID,
		Type:          actionNamePrefix(action),
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

	if err := s.appendExistingPlanJournal(ctx, operationID, target); err != nil {
		return domain.Report{}, err
	}

	applyResult, err := s.applyExistingToggleMutation(ctx, action, record, target)
	if err != nil {
		_ = s.Journal.AppendStep(ctx, operationID, domain.JournalStep{Target: string(target.TargetID), Action: "apply", Status: "failed"})
		return domain.Report{}, err
	}
	if err := s.Journal.AppendStep(ctx, operationID, domain.JournalStep{Target: string(target.TargetID), Action: "apply", Status: "ok"}); err != nil {
		return domain.Report{}, err
	}

	verifyRecord := provisionalRecordForExisting(record, target, applyResult)
	verified, err := s.verifyPostApply(ctx, record.IntegrationID, record.Policy, &verifyRecord, target.Adapter, action)
	if err != nil {
		_ = s.Journal.AppendStep(ctx, operationID, domain.JournalStep{Target: string(target.TargetID), Action: "verify", Status: "failed"})
		return domain.Report{}, err
	}
	if err := s.Journal.AppendStep(ctx, operationID, domain.JournalStep{Target: string(target.TargetID), Action: "verify", Status: "ok"}); err != nil {
		return domain.Report{}, err
	}

	if err := s.persistExistingToggleState(ctx, record, action, target, applyResult, verified, startedAt); err != nil {
		return domain.Report{}, err
	}
	if err := s.Journal.AppendStep(ctx, operationID, domain.JournalStep{Target: string(target.TargetID), Action: "persist_state", Status: "ok"}); err != nil {
		return domain.Report{}, err
	}
	if err := s.Journal.Finish(ctx, operationID, "committed"); err != nil {
		return domain.Report{}, err
	}
	committed = true

	return domain.Report{
		OperationID: operationID,
		Summary:     summaryForExisting(action, record.IntegrationID),
		Targets:     []domain.TargetReport{newExistingToggleTargetReport(record, target, verified, applyResult)},
	}, nil
}
