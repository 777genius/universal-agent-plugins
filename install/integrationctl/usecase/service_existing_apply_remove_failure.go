package usecase

import (
	"context"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func (s Service) finishRemoveExistingTargetFailure(ctx context.Context, runtime *existingRemoveRuntime, record domain.InstallationRecord, failedTarget domain.TargetID, removed []removedExistingTarget, cause error, rolledBackMessage string, degradedMessage string) (bool, error) {
	failureErr, done, failureState, finishErr := s.finishRemoveExistingFailure(ctx, runtime.operationID, runtime.state, record, runtime.startedAt, failedTarget, removed, cause, rolledBackMessage, degradedMessage)
	if finishErr != nil {
		return false, finishErr
	}
	if done {
		runtime.state = failureState
		return true, failureErr
	}
	return false, nil
}

func (s Service) finishRemoveExistingFailure(ctx context.Context, operationID string, state ports.StateFile, record domain.InstallationRecord, startedAt string, failedTarget domain.TargetID, removed []removedExistingTarget, cause error, rolledBackMessage string, degradedMessage string) (error, bool, ports.StateFile, error) {
	rollbackFailed, rollbackWarnings := s.rollbackRemovedExisting(ctx, operationID, record, removed)
	if len(rollbackFailed) > 0 {
		state.Installations = upsertInstallation(state.Installations, degradedRecordForRemoveFailure(record, startedAt, failedTarget, rollbackFailed))
		if err := s.StateStore.Save(ctx, state); err != nil {
			return nil, false, state, err
		}
		if err := s.Journal.AppendStep(ctx, operationID, domain.JournalStep{Target: "state", Action: "persist_degraded_state", Status: "ok"}); err != nil {
			return nil, false, state, err
		}
		if err := s.Journal.Finish(ctx, operationID, "degraded"); err != nil {
			return nil, false, state, err
		}
		return domain.NewError(domain.ErrMutationApply, failureMessage(degradedMessage, rollbackWarnings), cause), true, state, nil
	}
	if err := s.Journal.Finish(ctx, operationID, "rolled_back"); err != nil {
		return nil, false, state, err
	}
	return domain.NewError(domain.ErrMutationApply, failureMessage(rolledBackMessage, rollbackWarnings), cause), true, state, nil
}

func appendRemovedExisting(removed []removedExistingTarget, target plannedExistingTarget, result ports.ApplyResult) []removedExistingTarget {
	return append(removed, removedExistingTarget{Planned: target, Result: result})
}

func failureMessage(base string, warnings []string) string {
	if len(warnings) == 0 {
		return base
	}
	return base + ": " + strings.Join(warnings, "; ")
}

func degradedRecordForRemoveFailure(record domain.InstallationRecord, startedAt string, failedTarget domain.TargetID, rollbackFailed []domain.TargetID) domain.InstallationRecord {
	next := cloneInstallationRecord(record)
	next.LastCheckedAt = startedAt
	next.LastUpdatedAt = startedAt

	if len(rollbackFailed) > 0 {
		for targetID, target := range next.Targets {
			target.State = domain.InstallDegraded
			next.Targets[targetID] = target
		}
		return next
	}

	target, ok := next.Targets[failedTarget]
	if ok {
		target.State = domain.InstallDegraded
		next.Targets[failedTarget] = target
	}
	return next
}
