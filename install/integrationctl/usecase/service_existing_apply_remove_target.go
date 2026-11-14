package usecase

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type existingRemoveRuntime struct {
	operationID   string
	startedAt     string
	state         ports.StateFile
	nextRecord    domain.InstallationRecord
	removed       []removedExistingTarget
	reportTargets []domain.TargetReport
}

func (s Service) applyRemovedExistingTarget(ctx context.Context, record domain.InstallationRecord, target plannedExistingTarget, runtime *existingRemoveRuntime) (bool, error) {
	if err := s.appendExistingInspectPlanSteps(ctx, runtime.operationID, target.TargetID); err != nil {
		return false, err
	}
	applyResult, err := target.Adapter.ApplyRemove(ctx, ports.ApplyInput{
		Plan:    target.Plan,
		Policy:  record.Policy,
		Inspect: target.Inspect,
		Record:  &record,
	})
	if err != nil {
		_ = s.Journal.AppendStep(ctx, runtime.operationID, domain.JournalStep{Target: string(target.TargetID), Action: "apply", Status: "failed"})
		return s.finishRemoveExistingTargetFailure(ctx, runtime, record, target.TargetID, runtime.removed, err, "remove failed and removed targets were rolled back", "remove failed and rollback was incomplete; degraded state persisted")
	}
	if err := s.Journal.AppendStep(ctx, runtime.operationID, domain.JournalStep{Target: string(target.TargetID), Action: "apply", Status: "ok"}); err != nil {
		return false, err
	}
	verified, err := s.verifyRemovedExistingTarget(ctx, record, target)
	if err != nil {
		_ = s.Journal.AppendStep(ctx, runtime.operationID, domain.JournalStep{Target: string(target.TargetID), Action: "verify", Status: "failed"})
		failureRemoved := appendRemovedExisting(runtime.removed, target, applyResult)
		return s.finishRemoveExistingTargetFailure(ctx, runtime, record, target.TargetID, failureRemoved, err, "remove verification failed and removed targets were rolled back", "remove verification failed and rollback was incomplete; degraded state persisted")
	}
	if err := s.Journal.AppendStep(ctx, runtime.operationID, domain.JournalStep{Target: string(target.TargetID), Action: "verify", Status: "ok"}); err != nil {
		return false, err
	}
	runtime.removed = appendRemovedExisting(runtime.removed, target, applyResult)
	delete(runtime.nextRecord.Targets, target.TargetID)
	runtime.reportTargets = append(runtime.reportTargets, toAppliedTargetReport(record.IntegrationID, target.Delivery, target.Inspect, verified, target.Plan, applyResult))
	return false, nil
}

func (s Service) verifyRemovedExistingTarget(ctx context.Context, record domain.InstallationRecord, target plannedExistingTarget) (ports.InspectResult, error) {
	return s.verifyPostApply(ctx, record.IntegrationID, record.Policy, &record, target.Adapter, "remove_orphaned_target")
}
