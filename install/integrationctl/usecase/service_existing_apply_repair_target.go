package usecase

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type existingRepairRuntime struct {
	operationID   string
	startedAt     string
	state         ports.StateFile
	nextRecord    domain.InstallationRecord
	reportTargets []domain.TargetReport
}

func (s Service) applyRepairedExistingTarget(ctx context.Context, record domain.InstallationRecord, target plannedExistingTarget, runtime *existingRepairRuntime) (bool, error) {
	if err := s.appendExistingInspectPlanSteps(ctx, runtime.operationID, target.TargetID); err != nil {
		return false, err
	}
	applyResult, err := target.Adapter.Repair(ctx, ports.RepairInput{
		Record:         record,
		Inspect:        target.Inspect,
		Manifest:       target.Manifest,
		ResolvedSource: target.Resolved,
	})
	if err != nil {
		_ = s.Journal.AppendStep(ctx, runtime.operationID, domain.JournalStep{Target: string(target.TargetID), Action: "apply", Status: "failed"})
		return s.finishRepairExistingFailure(ctx, runtime, target, err, "repair failed after partial progress; degraded state persisted")
	}
	if err := s.Journal.AppendStep(ctx, runtime.operationID, domain.JournalStep{Target: string(target.TargetID), Action: "apply", Status: "ok"}); err != nil {
		return false, err
	}
	verified, err := s.verifyRepairedExistingTarget(ctx, record, target, applyResult)
	if err != nil {
		_ = s.Journal.AppendStep(ctx, runtime.operationID, domain.JournalStep{Target: string(target.TargetID), Action: "verify", Status: "failed"})
		return s.finishRepairExistingFailure(ctx, runtime, target, err, "repair verification failed after partial progress; degraded state persisted")
	}
	if err := s.Journal.AppendStep(ctx, runtime.operationID, domain.JournalStep{Target: string(target.TargetID), Action: "verify", Status: "ok"}); err != nil {
		return false, err
	}
	runtime.nextRecord.Targets[target.TargetID] = targetInstallationFromExisting(target, applyResult, verified)
	applyManifestMetadata(&runtime.nextRecord, *target.Manifest, runtime.startedAt)
	runtime.reportTargets = append(runtime.reportTargets, toAppliedTargetReport(record.IntegrationID, target.Delivery, target.Inspect, verified, target.Plan, applyResult))
	return false, nil
}

func (s Service) verifyRepairedExistingTarget(ctx context.Context, record domain.InstallationRecord, target plannedExistingTarget, applyResult ports.ApplyResult) (ports.InspectResult, error) {
	verifyRecord := provisionalRecordForExisting(record, target, applyResult)
	return s.verifyPostApply(ctx, record.IntegrationID, record.Policy, &verifyRecord, target.Adapter, "repair_drift")
}
