package usecase

import (
	"context"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
)

func (s Service) finishAddFailure(ctx context.Context, runtime *addApplyRuntime, manifest domain.IntegrationManifest, policy domain.InstallPolicy, applied []appliedTargetInstall, degradedMessage string, rolledBackMessage string, cause error) (bool, error) {
	rollbackFailed, rollbackWarnings := s.rollbackAppliedAdd(ctx, runtime.operationID, manifest, policy, runtime.workspaceRoot, runtime.startedAt, applied)
	if len(rollbackFailed) > 0 {
		runtime.state.Installations = upsertInstallation(runtime.state.Installations, degradedRecordFromApplied(manifest, policy, runtime.workspaceRoot, runtime.startedAt, rollbackFailed))
		if saveErr := s.StateStore.Save(ctx, runtime.state); saveErr != nil {
			return false, saveErr
		}
		if stepErr := s.Journal.AppendStep(ctx, runtime.operationID, domain.JournalStep{Target: "state", Action: "persist_degraded_state", Status: "ok"}); stepErr != nil {
			return false, stepErr
		}
		if finishErr := s.Journal.Finish(ctx, runtime.operationID, "degraded"); finishErr != nil {
			return false, finishErr
		}
		return true, domain.NewError(domain.ErrMutationApply, appendRollbackWarnings(degradedMessage, rollbackWarnings), cause)
	}
	if finishErr := s.Journal.Finish(ctx, runtime.operationID, "rolled_back"); finishErr != nil {
		return false, finishErr
	}
	return true, domain.NewError(domain.ErrMutationApply, appendRollbackWarnings(rolledBackMessage, rollbackWarnings), cause)
}

func appendRollbackWarnings(message string, warnings []string) string {
	if len(warnings) == 0 {
		return message
	}
	return message + ": " + strings.Join(warnings, "; ")
}
