package vscode

import (
	"context"
	"errors"
	"fmt"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/vscodeprofile"
)

func (*LocalAdapter) AutomaticallyActivates(_ clients.Env, req domain.ActivationRequest) bool {
	facts, ok := req.Plan.SelectedDelivery.LocalFacts()
	return req.Plan.InstallIntent == domain.InstallIntentAutomatic && ok && (facts.NativeStop || len(facts.MCPServers) != 0 || len(facts.Skills) != 0 || req.Plan.SelectedDelivery.OwnsProfileEntry(req.PreviousNativeObjects))
}

func (*LocalAdapter) VerifierAvailable(_ domain.DetectedClient, plan domain.DeliveryPlan, _ string) bool {
	return !plan.SelectedDelivery.IsZero()
}

func (*LocalAdapter) PreflightActivation(env clients.Env, req domain.ActivationRequest) error {
	facts, err := recordedLocalFacts(req.Plan.SelectedDelivery)
	if err != nil {
		return err
	}
	if err := req.Plan.SelectedDelivery.ValidatePlan(req.Plan, facts.CanonicalDigest); err != nil {
		return err
	}
	if req.Client.ClientID != domain.ClientVSCode || req.Client.ConfigRoot != facts.ProfileRoot || req.Plan.NativeRegistryRoot != facts.ProfileRoot {
		return fmt.Errorf("local activation profile differs")
	}
	if err := env.NativeConfig.RequireFileIO(); err != nil {
		return err
	}
	// Preflight carries no previous receipt on generic dry-run. Dispatch below
	// validates ownership against recorded objects and uses lock/CAS for effects.
	_, err = env.NativeConfig.ReadExactFile(facts.SettingsPath)
	return err
}

func (a *LocalAdapter) Activate(ctx context.Context, env clients.Env, req domain.ActivationRequest) (domain.ActivationOutcome, error) {
	out := domain.ActivationOutcome{Activation: domain.ActivationPrepared, Authentication: req.Plan.Authentication, Policy: req.Plan.Policy, Verification: domain.VerificationPackageValid, NativeEffect: domain.NativeEffectUnchanged}
	if err := ctx.Err(); err != nil {
		return out, err
	}
	if err := a.PreflightActivation(env, req); err != nil {
		return out, err
	}
	facts, _ := req.Plan.SelectedDelivery.LocalFacts()
	if !req.Replacing && req.Plan.LocalEntryObservation != nil && !localSameBasis(req.Plan.SelectedDelivery, req.Plan.LocalEntryObservation) {
		return out, fmt.Errorf("local ordinary activation cannot rebase a stale observation")
	}

	if req.Delivery.ActivePath != facts.Registration.Selector || req.Delivery.ArtifactDigest != facts.ProjectionDigest {
		return out, fmt.Errorf("local activation sealed artifact differs")
	}
	if _, err := localRecordedReceipt(req.Plan.SelectedDelivery, req.PreviousNativeObjects, req.Plan.LocalEntryObservation); err != nil {
		return out, err
	}
	owned, err := ownedLocalObjects(facts, req.PreviousNativeObjects)
	if err != nil {
		return out, err
	}
	if req.Plan.InstallIntent == domain.InstallIntentPrepare {
		return out, nil
	}
	if !facts.NativeStop && len(facts.MCPServers) == 0 && len(facts.Skills) == 0 && !owned {
		return out, nil
	}
	result, effect, err := activateLocalProfile(ctx, env.NativeConfig, req, owned)
	out.NativeEffect = effect
	if err != nil && !nativeconfig.IsCommittedCleanup(err) {
		return out, err
	}
	return finishLocalActivation(out, req, result, err)
}

func finishLocalActivation(out domain.ActivationOutcome, req domain.ActivationRequest, result vscodeprofile.Result, cleanupErr error) (domain.ActivationOutcome, error) {
	basis := req.Plan.SelectedDelivery
	if req.VerifyOnly && req.Plan.LocalEntryObservation != nil {
		basis = req.Plan.LocalEntryObservation.Facts().RevisionBasis
	}
	var err error
	out.LocalEntryObservation, err = localObservation(basis, result.Receipt)
	if err != nil {
		return out, err
	}
	facts, _ := req.Plan.SelectedDelivery.LocalFacts()
	out.NativeObjects = []domain.NativeObjectOwnership{facts.Registration.Ownership(facts.SettingsPath)}
	out.UserActions = []string{"selected Local plugin registered; reload required; live activation unknown"}
	if result.Disabled {
		out.UserActions = []string{"selected Local plugin is disabled; explicit native enablement required"}
	}
	return out, cleanupErr
}

func activateLocalProfile(ctx context.Context, kernel nativeconfig.Kernel, req domain.ActivationRequest, owned bool) (vscodeprofile.Result, domain.NativeEffectState, error) {
	if req.VerifyOnly {
		result, err := inspectObservedRegistration(kernel, req.Plan.SelectedDelivery, req.PreviousNativeObjects, req.Plan.LocalEntryObservation)
		if errors.Is(err, vscodeprofile.ErrRecordedEntryAbsent) && localSameBasis(req.Plan.SelectedDelivery, req.Plan.LocalEntryObservation) {
			absent, makeErr := domain.NewLocalEntryAbsence(req.Plan.SelectedDelivery)
			if makeErr != nil {
				return result, domain.NativeEffectUnchanged, makeErr
			}
			return result, domain.NativeEffectUnchanged, absent
		}
		return result, domain.NativeEffectUnchanged, err
	}
	action := vscodeprofile.Install
	if owned && req.Replacing {
		action = vscodeprofile.Repair
	}
	if owned && req.Plan.LocalEntryObservation != nil && !localSameBasis(req.Plan.SelectedDelivery, req.Plan.LocalEntryObservation) {
		action = vscodeprofile.Update
	}
	return mutateProfile(ctx, kernel, req.Plan.SelectedDelivery, action, req.PreviousNativeObjects, req.Plan.LocalEntryObservation)
}

func (*LocalAdapter) Deactivate(ctx context.Context, env clients.Env, req domain.DeactivationRequest) (domain.DeactivationOutcome, error) {
	out := domain.DeactivationOutcome{Activation: domain.ActivationNotRequired, ArtifactRemovalAllowed: true}
	facts, err := recordedLocalFacts(req.SelectedDelivery)
	if err != nil {
		return out, err
	}
	if req.Client.ConfigRoot != facts.ProfileRoot || req.ManagedArtifactPath != facts.Registration.Selector {
		return out, fmt.Errorf("local removal recorded profile/target differs")
	}
	owned, err := ownedLocalObjects(facts, req.NativeObjects)
	if err != nil {
		return out, err
	}
	if req.RemoveOwnedEntry != owned {
		return out, fmt.Errorf("local removal lacks matching reverse authority")
	}
	if _, err := localRecordedReceipt(req.SelectedDelivery, req.NativeObjects, req.LocalEntryObservation); err != nil {
		return out, err
	}
	if req.LocalEntryObservation != nil && !localSameBasis(req.SelectedDelivery, req.LocalEntryObservation) {
		return out, fmt.Errorf("local removal cannot rebase a stale observation")
	}
	if !req.Confirmed {
		return out, nil
	}
	if owned {
		if _, _, err := mutateProfile(ctx, env.NativeConfig, req.SelectedDelivery, vscodeprofile.Remove, req.NativeObjects, req.LocalEntryObservation); err != nil {
			if !nativeconfig.IsCommittedCleanup(err) {
				return out, err
			}
			out.UserActions = append(out.UserActions, "Local owned removal committed; native writer lock cleanup degraded")
		}
	}
	out.ExternalRemovalComplete = true
	return out, nil
}

// ReconcileNativeIntent implements the existing structurally matched selected
// recovery contract. The facade owns attempt/binding/global scope validation
// and package verification. No constructor/environment facts redirect recovery.
func (*LocalAdapter) ReconcileNativeIntent(ctx context.Context, intent domain.PendingNativeIntent) (domain.ActivationOutcome, error) {
	out := domain.ActivationOutcome{Activation: domain.ActivationPrepared, Authentication: domain.AuthenticationNotChecked, Policy: domain.PolicyAllowed, Verification: domain.VerificationPackageValid, NativeEffect: domain.NativeEffectUnchanged}
	facts, err := recordedLocalFacts(intent.Delivery)
	if err != nil {
		return out, err
	}
	if intent.AttemptID == "" || facts.ProjectionDigest == "" {
		return out, fmt.Errorf("local recovery requires exact recorded attempt/projection")
	}
	// The facade validates the durable intent against the exact binding before
	// this port runs. Only its predeclared selector supplies first-register authority.
	objects := []domain.NativeObjectOwnership{facts.Registration.Ownership(facts.SettingsPath)}
	if intent.Direction == domain.NativeIntentRemove {
		if !intent.RemoveOwnedEntry {
			if intent.LocalEntryObservation != nil {
				return out, fmt.Errorf("unowned removal cannot carry an observation")
			}
			return out, nil
		}
		_, out.NativeEffect, err = mutateProfile(ctx, nativeconfig.New(), intent.Delivery, vscodeprofile.Remove, objects, intent.LocalEntryObservation)
		return out, err
	}
	if intent.Direction != domain.NativeIntentRegister || intent.RemoveOwnedEntry {
		return out, fmt.Errorf("local pending direction invalid")
	}
	return reconcileLocalRegistration(ctx, out, intent, objects)
}

func reconcileLocalRegistration(ctx context.Context, out domain.ActivationOutcome, intent domain.PendingNativeIntent, objects []domain.NativeObjectOwnership) (domain.ActivationOutcome, error) {
	facts, _ := intent.Delivery.LocalFacts()
	if err := ctx.Err(); err != nil {
		return out, err
	}
	result, err := inspectObservedRegistration(nativeconfig.New(), intent.Delivery, objects, intent.LocalEntryObservation)
	if err != nil {
		return out, err
	}
	// Recovery observes a present entry; it cannot resend registration or restore
	// absence. A revision transition must verify old bytes before real Plan(Update).
	if intent.LocalEntryObservation != nil && !localSameBasis(intent.Delivery, intent.LocalEntryObservation) {
		result, err = vscodeprofile.Plan(vscodeprofile.Request{Settings: result.Settings, Identity: profileIdentity(facts), Action: vscodeprofile.Update, Previous: result.Receipt})
		if err != nil {
			return out, err
		}
		if result.Changed {
			return out, fmt.Errorf("local pending verification cannot resend registration")
		}
	}
	out.LocalEntryObservation, err = localObservation(intent.Delivery, result.Receipt)
	if err != nil {
		return out, err
	}
	out.NativeObjects = objects
	if result.Disabled {
		out.UserActions = []string{"recorded Local registration is disabled"}
	}
	return out, nil
}
