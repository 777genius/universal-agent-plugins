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
	release, err := service.beginMutation(ctx, false, true)
	if err != nil {
		return err
	}
	defer func() { _ = release() }()
	binding, err := service.pendingNativeBinding(installationID, bindingID, attemptID)
	if err != nil {
		return err
	}
	intent := *binding.PendingNativeIntent
	if err := service.Stager.Verify(ctx, binding.TargetLocator, managedDigest(binding)); err != nil {
		return fmt.Errorf("verify native recovery package: %w", err)
	}
	outcome, err := reconciler.ReconcileNativeIntent(ctx, intent)
	if err != nil {
		return err
	}
	if err := validateNativeReconciliation(intent, outcome); err != nil {
		return err
	}
	if intent.Direction == domain.NativeIntentRemove {
		return service.completeNativeRemoval(installationID, bindingID, true)
	}
	// The certain pending effect is now acknowledged, even when recovery only
	// read the entry written before death. Unchanged retry policy would discard
	// this newly established receipt and retain only pre-attempt ownership.
	outcome.NativeEffect = domain.NativeEffectCommitted
	_, err = service.updateActivationResult(installationID, bindingID, outcome, nil, binding.NativeObjects)
	return err
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
