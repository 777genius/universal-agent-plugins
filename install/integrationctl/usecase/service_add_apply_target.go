package usecase

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type addApplyRuntime struct {
	operationID   string
	startedAt     string
	workspaceRoot string
	state         ports.StateFile
	applied       []appliedTargetInstall
	reportTargets []domain.TargetReport
}

func (s Service) applyAddedTarget(ctx context.Context, manifest domain.IntegrationManifest, resolved ports.ResolvedSource, policy domain.InstallPolicy, target plannedTargetInstall, runtime *addApplyRuntime) (bool, error) {
	if err := s.recordAddTargetPlanning(ctx, runtime.operationID, target); err != nil {
		return false, err
	}
	applyResult, err := target.Adapter.ApplyInstall(ctx, ports.ApplyInput{
		Plan:           target.Plan,
		Manifest:       manifest,
		ResolvedSource: &resolved,
		Policy:         policy,
		Inspect:        target.Inspect,
	})
	if err != nil {
		_ = s.Journal.AppendStep(ctx, runtime.operationID, domain.JournalStep{Target: string(target.TargetID), Action: "apply", Status: "failed"})
		return s.finishAddFailure(ctx, runtime, manifest, policy, runtime.applied, "install failed and rollback was incomplete; degraded state persisted", "install failed and applied targets were rolled back", err)
	}
	if err := s.Journal.AppendStep(ctx, runtime.operationID, domain.JournalStep{Target: string(target.TargetID), Action: "apply", Status: "ok"}); err != nil {
		return false, err
	}
	verified, err := s.verifyAddedTarget(ctx, manifest, policy, target, applyResult, runtime.workspaceRoot)
	if err != nil {
		_ = s.Journal.AppendStep(ctx, runtime.operationID, domain.JournalStep{Target: string(target.TargetID), Action: "verify", Status: "failed"})
		failureApplied := append(runtime.applied, appliedTargetInstall{Planned: target, Result: applyResult})
		return s.finishAddFailure(ctx, runtime, manifest, policy, failureApplied, "install verification failed and rollback was incomplete; degraded state persisted", "install verification failed and applied targets were rolled back", err)
	}
	if err := s.Journal.AppendStep(ctx, runtime.operationID, domain.JournalStep{Target: string(target.TargetID), Action: "verify", Status: "ok"}); err != nil {
		return false, err
	}
	runtime.applied = append(runtime.applied, appliedTargetInstall{Planned: target, Result: applyResult, Verify: verified})
	runtime.reportTargets = append(runtime.reportTargets, toAppliedTargetReport(manifest.IntegrationID, target.Delivery, target.Inspect, verified, target.Plan, applyResult))
	return false, nil
}

func (s Service) recordAddTargetPlanning(ctx context.Context, operationID string, target plannedTargetInstall) error {
	if err := s.Journal.AppendStep(ctx, operationID, domain.JournalStep{Target: string(target.TargetID), Action: "inspect", Status: "ok"}); err != nil {
		return err
	}
	return s.Journal.AppendStep(ctx, operationID, domain.JournalStep{Target: string(target.TargetID), Action: "plan", Status: "ok"})
}

func (s Service) verifyAddedTarget(ctx context.Context, manifest domain.IntegrationManifest, policy domain.InstallPolicy, target plannedTargetInstall, applyResult ports.ApplyResult, workspaceRoot string) (ports.InspectResult, error) {
	verifyRecord := provisionalRecordForAdd(manifest, policy, workspaceRoot, target, applyResult)
	return s.verifyPostApply(ctx, manifest.IntegrationID, policy, &verifyRecord, target.Adapter, "add")
}
