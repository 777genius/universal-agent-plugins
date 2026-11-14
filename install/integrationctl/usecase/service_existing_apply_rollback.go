package usecase

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func (s Service) rollbackRemovedExisting(ctx context.Context, operationID string, record domain.InstallationRecord, removed []removedExistingTarget) ([]domain.TargetID, []string) {
	failed := make([]domain.TargetID, 0)
	warnings := make([]string, 0)
	for i := len(removed) - 1; i >= 0; i-- {
		failedTarget, warning, ok := s.rollbackRemovedExistingTarget(ctx, operationID, record, removed[i])
		if !ok {
			continue
		}
		failed = append(failed, failedTarget)
		warnings = append(warnings, warning)
	}
	return failed, warnings
}

func (s Service) rollbackRemovedExistingTarget(ctx context.Context, operationID string, record domain.InstallationRecord, item removedExistingTarget) (domain.TargetID, string, bool) {
	targetID := item.Planned.TargetID
	if item.Planned.Manifest == nil || item.Planned.Resolved == nil {
		return targetID, s.failRollbackStep(ctx, operationID, targetID, "rollback install context missing for "+string(targetID)), true
	}
	inspect, err := item.Planned.Adapter.Inspect(ctx, ports.InspectInput{IntegrationID: record.IntegrationID, Record: &record, Scope: record.Policy.Scope})
	if err != nil {
		return targetID, s.failRollbackStep(ctx, operationID, targetID, "rollback inspect failed for "+string(targetID)+": "+err.Error()), true
	}
	plan, err := item.Planned.Adapter.PlanInstall(ctx, ports.PlanInstallInput{
		Manifest: *item.Planned.Manifest,
		Policy:   record.Policy,
		Inspect:  inspect,
	})
	if err != nil {
		return targetID, s.failRollbackStep(ctx, operationID, targetID, "rollback plan failed for "+string(targetID)+": "+err.Error()), true
	}
	if _, err := s.validateEvidence(ctx, item.Planned.TargetID, plan.EvidenceKey); err != nil {
		return targetID, s.failRollbackStep(ctx, operationID, targetID, "rollback evidence validation failed for "+string(targetID)+": "+err.Error()), true
	}
	if _, err := item.Planned.Adapter.ApplyInstall(ctx, ports.ApplyInput{
		Plan:           plan,
		Manifest:       *item.Planned.Manifest,
		ResolvedSource: item.Planned.Resolved,
		Policy:         record.Policy,
		Inspect:        inspect,
		Record:         &record,
	}); err != nil {
		return targetID, s.failRollbackStep(ctx, operationID, targetID, "rollback apply failed for "+string(targetID)+": "+err.Error()), true
	}
	_ = s.Journal.AppendStep(ctx, operationID, domain.JournalStep{Target: string(targetID), Action: "rollback", Status: "ok"})
	return "", "", false
}

func (s Service) failRollbackStep(ctx context.Context, operationID string, targetID domain.TargetID, warning string) string {
	_ = s.Journal.AppendStep(ctx, operationID, domain.JournalStep{Target: string(targetID), Action: "rollback", Status: "failed"})
	return warning
}
