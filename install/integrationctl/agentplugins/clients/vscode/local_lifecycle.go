package vscode

import (
	"context"
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
	if err := a.PreflightActivation(env, req); err != nil {
		return out, err
	}
	facts, _ := req.Plan.SelectedDelivery.LocalFacts()
	if req.Delivery.ActivePath != facts.Registration.Selector || req.Delivery.ArtifactDigest != facts.ProjectionDigest {
		return out, fmt.Errorf("local activation sealed artifact differs")
	}
	if req.Plan.InstallIntent == domain.InstallIntentPrepare {
		return out, nil
	}
	owned, err := ownedLocalObjects(facts, req.PreviousNativeObjects)
	if err != nil {
		return out, err
	}
	if !facts.NativeStop && len(facts.MCPServers) == 0 && len(facts.Skills) == 0 && !owned {
		return out, nil
	}
	var state RegistrationState
	if req.VerifyOnly {
		state, err = inspectRegistration(env.NativeConfig, facts, owned)
		if state == RegistrationMissing && err == nil {
			err = fmt.Errorf("local owned selector absent; repair required")
		}
	} else {
		action := vscodeprofile.Install
		if owned {
			action = vscodeprofile.Repair
		}
		state, out.NativeEffect, err = mutateProfile(ctx, env.NativeConfig, facts, action, owned)
	}
	if err != nil && !nativeconfig.IsCommittedCleanup(err) {
		return out, err
	}
	out.NativeObjects = []domain.NativeObjectOwnership{facts.Registration.Ownership(facts.SettingsPath)}
	out.UserActions = []string{"selected Local plugin registered; reload required; live activation unknown"}
	if state == RegistrationDisabled {
		out.UserActions = []string{"selected Local plugin is disabled; explicit native enablement required"}
	}
	// InstallationVerified would overstate the public enum: profile bytes prove
	// registration, while execution/trust/policy are still unknown.
	return out, err
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
	if !req.Confirmed {
		return out, nil
	}
	if owned {
		if _, _, err := mutateProfile(ctx, env.NativeConfig, facts, vscodeprofile.Remove, true); err != nil {
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
	if intent.Direction == domain.NativeIntentRemove {
		if !intent.RemoveOwnedEntry {
			return out, nil
		}
		_, out.NativeEffect, err = mutateProfile(ctx, nativeconfig.New(), facts, vscodeprofile.Remove, true)
		return out, err
	}
	if intent.Direction != domain.NativeIntentRegister || intent.RemoveOwnedEntry {
		return out, fmt.Errorf("local pending direction invalid")
	}
	// A crashed first install can have written the predeclared bool. Only this
	// durable pending authority permits verification; ordinary Install cannot adopt.
	state, err := inspectRegistration(nativeconfig.New(), facts, true)
	if err != nil {
		return out, err
	}
	if state == RegistrationMissing {
		return out, fmt.Errorf("local pending registration absent; retain uncertain intent")
	}
	out.NativeObjects = []domain.NativeObjectOwnership{facts.Registration.Ownership(facts.SettingsPath)}
	if state == RegistrationDisabled {
		out.UserActions = []string{"recorded Local registration is disabled"}
	}
	return out, nil
}
