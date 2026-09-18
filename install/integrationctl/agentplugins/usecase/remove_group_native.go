package usecase

import (
	"fmt"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func (session *removeGroupSession) removeGroupNative() error {
	externalCompleted := 0
	for plannedIndex := range session.planned {
		item := &session.planned[plannedIndex]
		outcome, err := session.service.Activator.Deactivate(session.ctx, domain.DeactivationRequest{
			Client: item.input.Client, DeclaredName: session.installation.DeclaredName,
			CurrentActivation: item.client.Activation, Interactive: item.input.Interactive, ExternalUninstalled: item.input.ExternalUninstalled,
			Confirmed: true, PhysicalArtifactID: item.client.PhysicalArtifact, BackendExecutable: item.input.BackendExecutable,
			ManagedArtifactPath: item.client.TargetLocator,
			NativeObjects:       append([]domain.NativeObjectOwnership(nil), item.client.NativeObjects...),
		})
		for _, resultIndex := range item.resultIndexes {
			session.result.Targets[resultIndex].Deactivation = outcome
		}
		if err != nil {
			session.markNativeDeactivationFailed(item, externalCompleted)
			return fmt.Errorf("external deactivation failed; managed materialization was retained for repair: %w", err)
		}
		if !outcome.ArtifactRemovalAllowed {
			session.result.Phase = GroupPhaseManagedUnchanged
			if externalCompleted > 0 {
				session.result.Phase = GroupPhaseExternalPartialFailure
			}
			return fmt.Errorf("client did not authorize managed artifact removal")
		}
		for _, resultIndex := range item.resultIndexes {
			session.result.Targets[resultIndex].GroupPhase = GroupTargetExternalCompleted
		}
		externalCompleted += len(item.resultIndexes)
	}
	session.externalCompleted = externalCompleted
	return nil
}

func (session *removeGroupSession) markNativeDeactivationFailed(item *plannedRemoval, externalCompleted int) {
	for _, resultIndex := range item.resultIndexes {
		session.result.Targets[resultIndex].GroupPhase = GroupTargetExternalFailed
	}
	if externalCompleted > 0 {
		session.result.Phase = GroupPhaseExternalPartialFailure
		return
	}
	session.result.Phase = GroupPhaseManagedUnchanged
}
