package usecase

import (
	"fmt"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func (session *removeGroupSession) removeGroupNative() error {
	externalCompleted := 0
	for plannedIndex := range session.planned {
		item := &session.planned[plannedIndex]
		nativeAttempt := nativeLifecycleClient(item.input.Client.ClientID)
		if nativeAttempt {
			if err := session.service.beginNativeAttempt(session.installation.InstallationID, item.clientKey); err != nil {
				return err
			}
		}
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
		if nativeAttempt {
			if err := session.service.completeNativeRemoval(session.installation.InstallationID, item.clientKey, outcome.ExternalRemovalComplete); err != nil {
				session.markNativeDeactivationFailed(item, externalCompleted)
				return fmt.Errorf("persist native deactivation: %w", err)
			}
			state, err := session.service.StateStore.Load()
			if err != nil {
				return err
			}
			session.state = state
			session.installation = state.Installations[session.index]
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
