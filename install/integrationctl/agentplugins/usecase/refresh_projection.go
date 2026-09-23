package usecase

import (
	"context"
	"fmt"
	"reflect"

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
	defer func() { _ = session.service.Stager.Discard(context.Background(), delivery) }()
	pendingActivation := session.client.Activation == domain.ActivationPrepared || session.client.Activation == domain.ActivationFailed || session.client.Verification == domain.VerificationFailed
	// A failed activation retains the prior external ownership. That difference
	// from the staged desired ownership must not create another directory receipt.
	changed := delivery.ArtifactDigest != session.expectedDigest || (!pendingActivation && !reflect.DeepEqual(delivery.NativeObjects, session.client.NativeObjects))
	if changed {
		if err := session.service.observeNativeIdentity(session.ctx, session.input.Client, session.plan, &session.client); err != nil {
			return session.result, fmt.Errorf("native identity changed before projection refresh: %w", err)
		}
	}
	if err := session.service.Stager.Verify(session.ctx, session.target.ActivePath, session.expectedDigest); err != nil {
		return session.result, fmt.Errorf("managed package changed after refresh preflight; rerun refresh: %w", err)
	}
	if !changed {
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
	outcome, activationErr := session.service.Activator.Activate(session.ctx, domain.ActivationRequest{
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
	var changed bool
	var updateErr error
	if activationErr != nil {
		changed, updateErr = session.service.updateActivationResult(session.installation.InstallationID, session.clientKey, outcome, activationErr, session.client.NativeObjects)
	} else {
		// A retry may have retained the old external receipts after a failed
		// activation. Promote the staged desired ownership only once activation
		// succeeds, while keeping the committed package digest throughout.
		desired := append([]domain.NativeObjectOwnership(nil), delivery.NativeObjects...)
		changed, updateErr = session.service.updateLifecycleAndNativeObjects(session.installation.InstallationID, session.clientKey, outcome, &desired, false)
	}
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
