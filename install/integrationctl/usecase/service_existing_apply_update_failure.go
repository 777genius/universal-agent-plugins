package usecase

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func (s Service) finishUpdateExistingFailure(ctx context.Context, runtime *existingUpdateRuntime, target plannedExistingTarget, cause error, message string) (bool, error) {
	runtime.state = degradedExistingUpdateState(runtime.state, runtime.nextRecord, target, runtime.startedAt)
	if err := s.persistExistingUpdateDegradedState(ctx, runtime.operationID, runtime.state); err != nil {
		return false, err
	}
	return true, existingUpdateFailureError(message, cause)
}

func degradedExistingUpdateState(state ports.StateFile, nextRecord domain.InstallationRecord, target plannedExistingTarget, startedAt string) ports.StateFile {
	markPlannedTargetDegraded(&nextRecord, target)
	applyManifestMetadata(&nextRecord, *target.Manifest, startedAt)
	state.Installations = upsertInstallation(state.Installations, nextRecord)
	return state
}

func (s Service) persistExistingUpdateDegradedState(ctx context.Context, operationID string, state ports.StateFile) error {
	if err := s.StateStore.Save(ctx, state); err != nil {
		return err
	}
	if err := s.Journal.AppendStep(ctx, operationID, domain.JournalStep{Target: "state", Action: "persist_degraded_state", Status: "ok"}); err != nil {
		return err
	}
	return s.Journal.Finish(ctx, operationID, "degraded")
}

func existingUpdateFailureError(message string, cause error) error {
	return domain.NewError(domain.ErrMutationApply, message, cause)
}
