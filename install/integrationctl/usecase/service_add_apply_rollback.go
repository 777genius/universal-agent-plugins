package usecase

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func (s Service) rollbackAppliedAdd(ctx context.Context, operationID string, manifest domain.IntegrationManifest, policy domain.InstallPolicy, workspaceRoot string, startedAt string, applied []appliedTargetInstall) ([]appliedTargetInstall, []string) {
	failed := make([]appliedTargetInstall, 0)
	warnings := make([]string, 0)
	for i := len(applied) - 1; i >= 0; i-- {
		failedItem, warning, ok := s.rollbackAppliedAddTarget(ctx, operationID, manifest, policy, workspaceRoot, startedAt, applied[i])
		if !ok {
			continue
		}
		failed = append(failed, failedItem)
		warnings = append(warnings, warning)
	}
	return failed, warnings
}

func (s Service) rollbackAppliedAddTarget(ctx context.Context, operationID string, manifest domain.IntegrationManifest, policy domain.InstallPolicy, workspaceRoot string, startedAt string, item appliedTargetInstall) (appliedTargetInstall, string, bool) {
	record := installationRecordFromApplied(manifest, policy, workspaceRoot, startedAt, []appliedTargetInstall{item})
	inspect, err := item.Planned.Adapter.Inspect(ctx, ports.InspectInput{IntegrationID: record.IntegrationID, Record: &record, Scope: policy.Scope})
	if err != nil {
		_ = s.Journal.AppendStep(ctx, operationID, domain.JournalStep{Target: string(item.Planned.TargetID), Action: "rollback", Status: "failed"})
		return item, "rollback inspect failed for " + string(item.Planned.TargetID) + ": " + err.Error(), true
	}
	plan, err := item.Planned.Adapter.PlanRemove(ctx, ports.PlanRemoveInput{Record: record, Inspect: inspect})
	if err != nil {
		_ = s.Journal.AppendStep(ctx, operationID, domain.JournalStep{Target: string(item.Planned.TargetID), Action: "rollback", Status: "failed"})
		return item, "rollback plan failed for " + string(item.Planned.TargetID) + ": " + err.Error(), true
	}
	if _, err := s.validateEvidence(ctx, item.Planned.TargetID, plan.EvidenceKey); err != nil {
		_ = s.Journal.AppendStep(ctx, operationID, domain.JournalStep{Target: string(item.Planned.TargetID), Action: "rollback", Status: "failed"})
		return item, "rollback evidence validation failed for " + string(item.Planned.TargetID) + ": " + err.Error(), true
	}
	if _, err := item.Planned.Adapter.ApplyRemove(ctx, ports.ApplyInput{
		Plan:    plan,
		Policy:  policy,
		Inspect: inspect,
		Record:  &record,
	}); err != nil {
		_ = s.Journal.AppendStep(ctx, operationID, domain.JournalStep{Target: string(item.Planned.TargetID), Action: "rollback", Status: "failed"})
		return item, "rollback apply failed for " + string(item.Planned.TargetID) + ": " + err.Error(), true
	}
	_ = s.Journal.AppendStep(ctx, operationID, domain.JournalStep{Target: string(item.Planned.TargetID), Action: "rollback", Status: "ok"})
	return appliedTargetInstall{}, "", false
}
