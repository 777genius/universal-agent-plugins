package usecase

import (
	"context"
	"fmt"
	"reflect"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
)

func lifecycleConverged(client domain.ClientBinding) bool { return client.LifecycleConverged() }

// resume retries only the external client lifecycle against an existing,
// digest-verified managed package. It deliberately creates no directory
// transaction or receipt, so repeating add cannot replace or duplicate the
// materialized artifact.
func (service Service) resume(
	ctx context.Context,
	input AddInput,
	result AddResult,
	installationID, clientBindingID string,
	client domain.ClientBinding,
) (AddResult, error) {
	previousObservation := client.LocalEntryObservation.Clone()
	if input.DryRun {
		return result, nil
	}
	if err := service.verifyManagedTarget(ctx, input.Client, input.Scope, client, "resume"); err != nil {
		return result, err
	}
	result.Activation = lifecycleOutcome(client)
	if !input.Confirmed {
		result.RequiresConfirmation = true
		return result, nil
	}
	delivery, complete, settled, err := service.resumeNativeDelivery(ctx, input, result.Plan, installationID, client)
	if err != nil {
		return result, err
	}
	if client.NativeActivationAttempt != "" || client.PendingNativeIntent != nil {
		return result, fmt.Errorf("native activation attempt %s is unresolved; inspect the owned client state before retry", unresolvedNativeAttemptID(client))
	}
	if complete && !result.Plan.SelectedDelivery.IsZero() {
		result.Plan, err = service.readOnlyObservationPlan(client, result.Plan)
		if err != nil {
			return result, err
		}
	}
	request := domain.ActivationRequest{
		Client: input.Client, Plan: result.Plan, Delivery: delivery, PackageUnchanged: true,
		DeclaredName: input.Envelope.Manifest.Name, Replacing: true,
		Interactive: input.Interactive, BackendExecutable: input.BackendExecutable,
		PreviousNativeObjects: append([]domain.NativeObjectOwnership(nil), client.NativeObjects...),
		VerifyOnly:            complete, ActivationComplete: input.ActivationComplete,
	}
	var outcome domain.ActivationOutcome
	var activationErr error
	if complete && !result.Plan.SelectedDelivery.IsZero() {
		outcome, activationErr = service.activateReadOnlyWithObservation(ctx, client, request)
	} else {
		outcome, activationErr = service.activateWithNativeAttempt(ctx, installationID, clientBindingID, request)
	}
	outcome = service.resumeActivationOutcome(input, result.Plan, client, outcome, activationErr)
	result.Activation = outcome
	changed, updateErr := service.updateActivationResultWithObservation(installationID, clientBindingID, outcome, activationErr, client.NativeObjects, previousObservation)
	result.Mutated = changed
	if updateErr != nil {
		if activationErr != nil {
			return result, fmt.Errorf("resume client activation: %w; persist activation state: %w", activationErr, updateErr)
		}
		return result, updateErr
	}
	if activationErr != nil {
		return result, activationErr
	}
	// A settled manual registration is a stable no-change outcome: this request
	// cannot finish it, and a later one the adapter can act on will.
	result.NoChange = !changed && (complete && (service.verifiedRegistrationUnchanged(input, result.Plan, client, outcome) || settled) || service.verifiedPreparedObservationUnchanged(input, result.Plan, client, delivery, outcome, complete))
	return result, nil
}

func unresolvedNativeAttemptID(client domain.ClientBinding) string {
	if client.NativeActivationAttempt != "" {
		return client.NativeActivationAttempt
	}
	if client.PendingNativeIntent != nil {
		return client.PendingNativeIntent.AttemptID
	}
	return ""
}

func (service Service) resumeNativeDelivery(ctx context.Context, input AddInput, plan domain.DeliveryPlan, installationID string, client domain.ClientBinding) (domain.StagedDelivery, bool, bool, error) {
	state, err := service.StateStore.Load()
	if err != nil {
		return domain.StagedDelivery{}, false, false, err
	}
	var installation domain.Installation
	for _, item := range state.Installations {
		if item.InstallationID == installationID {
			installation = item
			break
		}
	}
	return service.activeNativeDelivery(ctx, input, plan, installation, client)
}

func (service Service) resumeActivationOutcome(input AddInput, plan domain.DeliveryPlan, client domain.ClientBinding, outcome domain.ActivationOutcome, activationErr error) domain.ActivationOutcome {
	// Authentication completion is a separate phase. Client installation/list
	// evidence must never silently complete it.
	if activationErr == nil && !service.clientVerifierAvailable(input, plan) && client.Activation == domain.ActivationActive && client.Verification == domain.VerificationInstalled {
		outcome.Activation = client.Activation
		outcome.Verification = client.Verification
	}
	if (client.Authentication == domain.AuthenticationPending || client.Authentication == domain.AuthenticationNotChecked) && input.AuthComplete {
		outcome.Authentication = domain.AuthenticationComplete
		outcome.AuthenticationAttested = true
	} else if client.Authentication != "" {
		outcome.Authentication = client.Authentication
	}
	if activationErr != nil && outcome.Activation == "" {
		effect, objects := outcome.NativeEffect, outcome.NativeObjects
		outcome = domain.ActivationOutcome{
			Activation: domain.ActivationFailed, Authentication: plan.Authentication,
			Policy: domain.PolicyAllowed, Verification: domain.VerificationFailed,
			NativeEffect: effect, NativeObjects: objects,
		}
	}
	return outcome
}

// Unchecked authentication is not an outstanding action by itself. Report
// an unchanged verified registration without manufacturing an auth attestation.
func (service Service) verifiedRegistrationUnchanged(input AddInput, plan domain.DeliveryPlan, client domain.ClientBinding, outcome domain.ActivationOutcome) bool {
	return service.clientVerifierAvailable(input, plan) &&
		client.Materialization == domain.MaterializationMaterialized &&
		outcome.Activation == domain.ActivationActive && outcome.Verification == domain.VerificationInstalled &&
		outcome.Authentication == domain.AuthenticationNotChecked &&
		(plan.Authentication == domain.AuthenticationNotChecked || plan.Authentication == domain.AuthenticationNotRequired) &&
		!input.AuthComplete && !outcome.AuthenticationAttested && len(outcome.UserActions) == 0
}

// verifiedPreparedObservationUnchanged classifies a repeated project-only,
// read-only observation, not lifecycle convergence. The verified existing
// delivery is reused; every subsequent request still reobserves the client.
func (service Service) verifiedPreparedObservationUnchanged(input AddInput, plan domain.DeliveryPlan, client domain.ClientBinding, delivery domain.StagedDelivery, outcome domain.ActivationOutcome, nativeComplete bool) bool {
	if plan.SelectedDelivery.EffectiveTraitsForScope(input.Client.ClientID, input.Scope).LifecycleKind == domain.LifecycleNativeConfig {
		return nativeComplete && service.verifiedPreparedNativeConfigUnchanged(input, plan, client, delivery, outcome)
	}
	return input.Scope == domain.ScopeProject && !nativeLifecycleClient(input.Client.ClientID, plan.SelectedDelivery, input.Scope) &&
		service.clientVerifierAvailable(input, plan) && client.Materialization == domain.MaterializationMaterialized &&
		client.Activation == domain.ActivationPrepared && client.Verification == domain.VerificationInstalled &&
		outcome.Activation == client.Activation && outcome.Verification == client.Verification &&
		outcome.Authentication == client.Authentication && outcome.Policy == client.Policy &&
		!input.ActivationComplete && !input.AuthComplete && !outcome.ActivationAttested && !outcome.AuthenticationAttested &&
		(outcome.NativeEffect == "" || outcome.NativeEffect == domain.NativeEffectUnchanged) && len(outcome.NativeObjects) == 0 &&
		preparedDeliveryUnchanged(client, delivery)
}

// A complete native projection can retain Prepared while exact file verification
// succeeds. This classifies only that operation; it does not converge lifecycle.
func (service Service) verifiedPreparedNativeConfigUnchanged(input AddInput, plan domain.DeliveryPlan, client domain.ClientBinding, delivery domain.StagedDelivery, outcome domain.ActivationOutcome) bool {
	return input.Scope == domain.ScopeProject && plan.Scope == domain.ScopeProject && client.Scope == string(domain.ScopeProject) &&
		plan.ClientID == input.Client.ClientID && client.ClientID == string(input.Client.ClientID) && delivery.ClientID == input.Client.ClientID &&
		plan.ActivePath != "" && plan.ActivePath == client.TargetLocator &&
		plan.SelectedDelivery.IsZero() && client.SelectedDelivery.IsZero() && service.clientVerifierAvailable(input, plan) &&
		len(client.NativeObjects) > 0 && reflect.DeepEqual(outcome.NativeObjects, client.NativeObjects) &&
		preparedDeliveryUnchanged(client, delivery) && preparedNativeConfigLifecycleUnchanged(input, client, outcome)
}

func preparedNativeConfigLifecycleUnchanged(input AddInput, client domain.ClientBinding, outcome domain.ActivationOutcome) bool {
	return outcome.Policy == domain.PolicyAllowed &&
		client.Materialization == domain.MaterializationMaterialized && client.Activation == domain.ActivationPrepared && client.Verification == domain.VerificationInstalled &&
		outcome.Activation == client.Activation && outcome.Verification == client.Verification && outcome.Authentication == client.Authentication && outcome.Policy == client.Policy &&
		client.NativeActivationAttempt == "" && client.PendingNativeIntent == nil && !input.ActivationComplete && !input.AuthComplete &&
		!outcome.ActivationAttested && !outcome.AuthenticationAttested && !outcome.AuthoritativeObservation && outcome.LocalEntryObservation == nil &&
		outcome.NativeEffect == domain.NativeEffectUnchanged
}

func preparedDeliveryUnchanged(client domain.ClientBinding, delivery domain.StagedDelivery) bool {
	return delivery.ActivePath == client.TargetLocator && delivery.ArtifactDigest != "" &&
		delivery.ArtifactDigest == managedDigest(client) && reflect.DeepEqual(delivery.NativeObjects, client.NativeObjects)
}

// clientVerifierAvailable asks the activator whether an exact client-side
// verifier can observe this plan. An activator without the capability reports
// no verifier, so prior evidence is re-derived rather than retained.
func (service Service) clientVerifierAvailable(input AddInput, plan domain.DeliveryPlan) bool {
	classifier, ok := service.Activator.(ports.ActivationVerifierClassifier)
	if !ok {
		return false
	}
	return classifier.VerifierAvailable(input.Client, plan, input.BackendExecutable)
}
