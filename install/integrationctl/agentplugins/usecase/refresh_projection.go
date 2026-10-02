package usecase

import (
	"fmt"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func (session *repairSession) refreshIntactProjection() (AddResult, error) {
	if session.verifyErr != nil {
		return session.result, fmt.Errorf("managed package is not intact; run repair before refreshing its projection: %w", session.verifyErr)
	}
	if session.input.DryRun {
		return session.result, nil
	}
	if !session.input.Confirmed {
		session.result.RequiresConfirmation = true
		return session.result, nil
	}
	delivery, err := session.stageRepairDelivery(true)
	if err != nil {
		return session.result, err
	}
	defer func() { session.service.discardSettledDelivery(session.input.OperationID, delivery) }()
	selectionChanged := !session.client.SelectedDelivery.SameSelection(session.plan.SelectedDelivery)
	pendingActivation := selectionChanged || session.client.Activation == domain.ActivationPrepared || session.client.Activation == domain.ActivationFailed || session.client.Verification == domain.VerificationFailed || !confirmedNativeProjection(session.client.NativeObjects, delivery.NativeObjects)
	// External receipt drift alone never creates a new directory transaction.
	changed := delivery.ArtifactDigest != session.expectedDigest
	if changed {
		if err := session.service.observeNativeIdentity(session.ctx, session.input.Client, session.plan, &session.client); err != nil {
			return session.result, fmt.Errorf("native identity changed before projection refresh: %w", err)
		}
	}
	if err := session.service.Stager.Verify(session.ctx, session.target.ActivePath, session.expectedDigest); err != nil {
		return session.result, fmt.Errorf("managed package changed after refresh preflight; rerun refresh: %w", err)
	}
	if !changed {
		facts, local := session.plan.SelectedDelivery.LocalFacts()
		if local && !selectionChanged && !facts.NativeStop && len(facts.MCPServers) == 0 && len(facts.Skills) == 0 && confirmedNativeProjection(session.client.NativeObjects, delivery.NativeObjects) {
			// Prepared/package-valid is the certain Local registration result,
			// not an outstanding activation. Verify the retained empty route
			// through the existing native maintenance path without a new attempt.
			return session.repairNative()
		}
		if selectionChanged {
			if err := session.persistRefreshedSelection(); err != nil {
				return session.result, err
			}
		}
		if pendingActivation {
			return session.activateRefreshedProjection(delivery)
		}
		session.result.NoChange = true
		return session.result, nil
	}
	pending := lifecycleOutcome(session.client)
	pending.Activation = domain.ActivationPrepared
	pending.Verification = domain.VerificationPackageValid
	result, err := session.commitRepairDirectory(delivery, session.expectedDigest, pending)
	if err != nil {
		return result, err
	}
	return session.activateRefreshedProjection(delivery)
}

func (session *repairSession) activateRefreshedProjection(delivery domain.StagedDelivery) (AddResult, error) {
	outcome, activationErr := session.service.activateWithNativeAttempt(session.ctx, session.installation.InstallationID, session.clientKey, domain.ActivationRequest{
		Client: session.input.Client, Plan: session.plan, Delivery: delivery,
		DeclaredName: session.input.Envelope.Manifest.Name, Replacing: true,
		Interactive: session.input.Interactive, BackendExecutable: session.input.BackendExecutable,
		PreviousNativeObjects: append([]domain.NativeObjectOwnership(nil), session.client.NativeObjects...),
	})
	outcome = preserveManagedAuthentication(outcome, session.client.Authentication)
	if activationErr != nil {
		outcome.Activation = domain.ActivationFailed
		outcome.Verification = domain.VerificationFailed
		if outcome.Policy == "" {
			outcome.Policy = session.client.Policy
		}
	}
	session.result.Activation = outcome
	changed, updateErr := session.service.updateActivationResult(session.installation.InstallationID, session.clientKey, outcome, activationErr, session.client.NativeObjects)
	session.result.Mutated = session.result.Mutated || changed
	if updateErr != nil {
		if activationErr != nil {
			return session.result, fmt.Errorf("activate refreshed projection: %w; persist failure state: %w", activationErr, updateErr)
		}
		return session.result, fmt.Errorf("persist refreshed projection activation: %w", updateErr)
	}
	if activationErr != nil {
		return session.result, fmt.Errorf("activate refreshed projection: %w", activationErr)
	}
	return session.result, nil
}

// Equal projected bytes can still represent a new explicitly reviewed selected
// delivery. Persist that exact decision before activation, using the same state
// journal rather than replacing an intact directory or adding a second journal.
func (session *repairSession) persistRefreshedSelection() error {
	client := session.client
	client.SelectedDelivery = session.plan.SelectedDelivery
	client.Activation = domain.ActivationPrepared
	client.Verification = domain.VerificationPackageValid
	installation := session.installation
	installation.Clients = cloneClientBindings(installation.Clients)
	installation.Clients[session.clientKey] = client
	session.state.Installations[session.index] = installation
	if err := session.service.persistLifecycleState(session.state); err != nil {
		return err
	}
	session.client, session.installation = client, installation
	session.result.Mutated = true
	return nil
}
