package usecase

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
)

func (s Service) finishRepairExistingFailure(ctx context.Context, runtime *existingRepairRuntime, target plannedExistingTarget, cause error, message string) (bool, error) {
	markTargetDegraded(&runtime.nextRecord, target.TargetID)
	applyManifestMetadata(&runtime.nextRecord, *target.Manifest, runtime.startedAt)
	runtime.state.Installations = upsertInstallation(runtime.state.Installations, runtime.nextRecord)
	if err := s.StateStore.Save(ctx, runtime.state); err != nil {
		return false, err
	}
	if err := s.Journal.AppendStep(ctx, runtime.operationID, domain.JournalStep{Target: "state", Action: "persist_degraded_state", Status: "ok"}); err != nil {
		return false, err
	}
	if err := s.Journal.Finish(ctx, runtime.operationID, "degraded"); err != nil {
		return false, err
	}
	return true, domain.NewError(domain.ErrRepairApply, message, cause)
}
