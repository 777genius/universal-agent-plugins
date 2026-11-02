package usecase

import (
	"context"
	"errors"
	"fmt"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
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
	if current, err := session.commandSkillsAlreadyCurrent(); current || err != nil {
		return session.result, err
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
		return session.finishUnchangedProjection(delivery, selectionChanged, pendingActivation)
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

// commandSkillsAlreadyCurrent ends a refresh ahead of an update, with no
// change, when the installed command skills already match the current rules.
// Only command rules bring an update here; other host projection inputs, such
// as a config file the client created since, stay with RefreshProjection, as
// they do for a package without commands.
func (session *repairSession) commandSkillsAlreadyCurrent() (bool, error) {
	if !session.input.refreshChangedProjectionOnly {
		return false, nil
	}
	current, verified, err := session.service.commandSkillsCurrent(session.ctx, session.target.ActivePath, session.input.Envelope, session.plan)
	if err != nil || !verified || !current {
		return false, err
	}
	session.result.NoChange = true
	return true, nil
}

// finishUnchangedProjection settles a refresh whose staged digest equals the
// recorded one.
func (session *repairSession) finishUnchangedProjection(delivery domain.StagedDelivery, selectionChanged, pendingActivation bool) (AddResult, error) {
	if session.input.refreshChangedProjectionOnly {
		// The update that follows owns selection and activation; an
		// identical projection leaves nothing for this refresh to do.
		session.result.NoChange = true
		return session.result, nil
	}
	if session.hasEmptyLocalMaintenanceRoute(delivery, selectionChanged) {
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

// commandSkillsCurrent reports whether the installed tree at root already
// holds the command skills the current rules render. Only command rules
// decide; other host projection inputs do not. verified is false when the
// stager cannot tell, and callers then fall back to comparing a staged digest.
func (service Service) commandSkillsCurrent(ctx context.Context, root string, envelope domain.PackageEnvelope, plan domain.DeliveryPlan) (current, verified bool, err error) {
	verifier, ok := service.Stager.(ports.CommandSkillVerifier)
	if !ok {
		return false, false, nil
	}
	switch err := verifier.VerifyCommandSkills(ctx, root, envelope, plan); {
	case err == nil:
		return true, true, nil
	case errors.Is(err, ErrProjectionRefreshRequired):
		return false, true, nil
	default:
		return false, true, err
	}
}

func (session *repairSession) hasEmptyLocalMaintenanceRoute(delivery domain.StagedDelivery, selectionChanged bool) bool {
	facts, local := session.plan.SelectedDelivery.LocalFacts()
	return local && !selectionChanged && !facts.NativeStop && len(facts.MCPServers) == 0 && len(facts.Skills) == 0 && confirmedNativeProjection(session.client.NativeObjects, delivery.NativeObjects)
}

func (session *repairSession) activateRefreshedProjection(delivery domain.StagedDelivery) (AddResult, error) {
	previousObservation := session.plan.LocalEntryObservation.Clone()
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
	changed, updateErr := session.service.updateActivationResultWithObservation(session.installation.InstallationID, session.clientKey, outcome, activationErr, session.client.NativeObjects, previousObservation)
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
