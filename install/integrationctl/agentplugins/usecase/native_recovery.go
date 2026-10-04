package usecase

import (
	"context"
	"fmt"
	"reflect"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// NativeIntentReconciler is the selected adapter's narrow recovery operation.
// It must re-read the recorded physical settings and reconcile only the pending
// selector, refusing drift. It must not restore a document backup, rediscover a
// profile, or infer ownership from identical bytes without this pending intent.
// The future facade composes this explicitly; no production Local writer exists.
type NativeIntentReconciler interface {
	ReconcileNativeIntent(context.Context, domain.PendingNativeIntent) (domain.ActivationOutcome, error)
}

// RecoverNativeIntents uses the same mutation lock, journal and binding state as
// installation. Its scope is one observed attempt, not every pending profile.
// A stale observation, missing reconciler or changed package refuses before the
// native callback. Persisted delivery facts are the only recovery input.
func (service Service) RecoverNativeIntent(ctx context.Context, installationID, bindingID, attemptID string, reconciler NativeIntentReconciler) error {
	if reconciler == nil || service.StateStore == nil || service.Stager == nil {
		return fmt.Errorf("native recovery requires state, stager and selected reconciler")
	}
	binding, err := service.pendingNativeBinding(installationID, bindingID, attemptID)
	if err != nil {
		return err
	}
	service, err = service.freezeNativeRecoveryProfile(ctx, installationID, binding)
	if err != nil {
		return err
	}
	release, err := service.beginMutation(ctx, false, true)
	if err != nil {
		return err
	}
	defer func() { _ = release() }()
	binding, err = service.pendingNativeBinding(installationID, bindingID, attemptID)
	if err != nil {
		return err
	}
	previousObservation := binding.LocalEntryObservation.Clone()
	intent := *binding.PendingNativeIntent.Clone()
	intent.LocalEntryObservation = intent.LocalEntryObservation.Clone()
	if err := service.Stager.Verify(ctx, binding.TargetLocator, managedDigest(binding)); err != nil {
		return fmt.Errorf("verify native recovery package: %w", err)
	}
	if err := service.checkProfiles(ctx); err != nil {
		return err
	}
	outcome, err := reconciler.ReconcileNativeIntent(ctx, intent)
	if err != nil {
		return err
	}
	current, loadErr := service.activationBinding(installationID, bindingID)
	if loadErr != nil {
		return loadErr
	}
	if !reflect.DeepEqual(binding, current) {
		return fmt.Errorf("native recovery callback changed frozen predecessor")
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err := validateNativeReconciliation(intent, outcome); err != nil {
		return err
	}
	if intent.Direction == domain.NativeIntentRemove {
		return service.completeNativeRemoval(installationID, bindingID, true)
	}
	return service.acknowledgeNativeRecovery(installationID, bindingID, binding, outcome, previousObservation)
}

func (service Service) pendingNativeBinding(installationID, bindingID, attemptID string) (domain.ClientBinding, error) {
	state, err := service.StateStore.Load()
	if err != nil {
		return domain.ClientBinding{}, err
	}
	for _, installation := range state.Installations {
		if installation.InstallationID != installationID {
			continue
		}
		binding, ok := installation.Clients[bindingID]
		if !ok || binding.PendingNativeIntent == nil || binding.NativeActivationAttempt != attemptID {
			return domain.ClientBinding{}, fmt.Errorf("native recovery observation is stale or has no owned pending intent")
		}
		if err := binding.PendingNativeIntent.Validate(binding); err != nil {
			return domain.ClientBinding{}, err
		}
		facts, _ := binding.SelectedDelivery.LocalFacts()
		if managedDigest(binding) != facts.ProjectionDigest {
			return domain.ClientBinding{}, fmt.Errorf("native recovery projection differs from managed directory receipt")
		}
		return binding, nil
	}
	return domain.ClientBinding{}, fmt.Errorf("native recovery installation is absent")
}

func validateNativeReconciliation(intent domain.PendingNativeIntent, outcome domain.ActivationOutcome) error {
	if outcome.NativeEffect != domain.NativeEffectCommitted && outcome.NativeEffect != domain.NativeEffectUnchanged {
		return fmt.Errorf("native recovery did not establish a certain owned effect")
	}
	if intent.Direction == domain.NativeIntentRemove {
		if len(outcome.NativeObjects) != 0 {
			return fmt.Errorf("native removal recovery retained external ownership")
		}
		return nil
	}
	facts, _ := intent.Delivery.LocalFacts()
	expected := facts.Registration.Ownership(facts.SettingsPath)
	if len(outcome.NativeObjects) != 1 || !reflect.DeepEqual(outcome.NativeObjects[0], expected) {
		return fmt.Errorf("native recovery returned ownership outside the predeclared selector/value")
	}
	return nil
}

func (service Service) freezeNativeRecoveryProfile(ctx context.Context, installationID string, binding domain.ClientBinding) (Service, error) {
	client := domain.DetectedClient{ClientID: domain.ClientID(binding.ClientID), ConfigRoot: binding.NativeProfileRoot, ProfileAuthority: domain.CloneProfileAuthority(binding.ProfileAuthority), ProfileNamespace: binding.ProfileNamespace}
	if client.ProfileAuthority != nil {
		client.ConfigRoot = client.ProfileAuthority.Facts().CanonicalRoot
	}
	returnService, _, err := service.freezeProfiles(ctx, installationID, []domain.DetectedClient{client}, false)
	if err != nil {
		return service, err
	}
	return returnService, nil
}

func (service Service) acknowledgeNativeRecovery(installationID, bindingID string, binding domain.ClientBinding, outcome domain.ActivationOutcome, previousObservation *domain.LocalEntryObservation) error {
	// The certain pending effect is now acknowledged, even when recovery only
	// read the entry written before death. Unchanged retry policy would discard
	// this newly established receipt and retain only pre-attempt ownership.
	if outcome.LocalEntryObservation == nil && previousObservation == nil {
		outcome.NativeEffect = domain.NativeEffectCommitted
	}
	// This register intent and its exact outcome selector were validated above.
	// A verified unchanged receipt can acknowledge that predeclared selector even
	// on first registration, where no previous external ownership was confirmed.
	// Ordinary unchanged callers continue to pass only their frozen prior objects.
	acknowledgedObjects := binding.NativeObjects
	if outcome.LocalEntryObservation != nil && outcome.NativeEffect == domain.NativeEffectUnchanged {
		acknowledgedObjects = append([]domain.NativeObjectOwnership(nil), outcome.NativeObjects...)
	}
	_, err := service.updateActivationResultWithObservation(installationID, bindingID, outcome, nil, acknowledgedObjects, previousObservation)
	return err
}
