package installer

import (
	"reflect"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

// A successful user group file operation must not be confused with native
// client activation. This is a reporting-only exception for an existing
// prepared-package route, never a native trust or activation attestation.
func completedUserPreparedUpdate(prepared *PreparedOperation, got usecase.GroupResult, state domain.StateFileV2) bool {
	if !validUserPreparedGroupShape(prepared, got) {
		return false
	}
	size := len(prepared.plan.Targets)
	installation, err := completedGroupInstallation(state, got.InstallationID)
	if err != nil {
		return false
	}
	targets := make(map[string]usecase.AddResult, size)
	for _, target := range got.Targets {
		id := string(target.Plan.ClientID)
		if id == "" || target.InstallationID != got.InstallationID ||
			target.GroupPhase != usecase.GroupTargetExternalCompleted || target.Failure != nil {
			return false
		}
		if _, duplicate := targets[id]; duplicate {
			return false
		}
		targets[id] = target
	}
	hasPrepared := false
	for i, frozen := range prepared.plan.Targets {
		target, ok := targets[frozen.ClientID]
		if !ok || !completedUserPreparedTarget(prepared, frozen, i, target, installation) {
			return false
		}
		hasPrepared = hasPrepared || target.Activation.Activation == domain.ActivationPrepared
		delete(targets, frozen.ClientID)
	}
	return hasPrepared && len(targets) == 0
}


func validUserPreparedGroupShape(prepared *PreparedOperation, got usecase.GroupResult) bool {
	if prepared.req.Operation != OpUpdate || (prepared.req.Scope != "" && prepared.req.Scope != domain.ScopeUser) ||
		prepared.req.ProjectRoot != "" || got.Phase != usecase.GroupPhaseCompleted ||
		got.InstallationID == "" || got.InstallationID != prepared.req.InstallationID ||
		got.InstallationID != prepared.plan.InstallationID {
		return false
	}
	size := len(prepared.plan.Targets)
	return size == 2 && len(got.Targets) == size && len(prepared.clients) == size && len(prepared.req.Targets) == size
}

func completedUserPreparedTarget(prepared *PreparedOperation, frozen PlanTarget, i int, target usecase.AddResult, installation domain.Installation) bool {
	if frozen.ClientID == "" || frozen.ClientID != prepared.req.Targets[i].ClientID ||
		frozen.ClientID != string(prepared.clients[i].ClientID) ||
		frozen.ConfigRoot != prepared.clients[i].ConfigRoot {
		return false
	}
	plan := target.Plan
	context, err := domain.NewTargetContext(domain.ScopeUser, "", frozen.ConfigRoot)
	if err != nil || plan.Scope != domain.ScopeUser || plan.TargetContext != context ||
		plan.ActivePath == "" || plan.ActivePath != frozen.TargetPath || plan.PhysicalArtifactID == "" ||
		frozen.BindingID == "" ||
		frozen.BindingID != domain.ComputeClientBindingID(prepared.plan.InstallationID, frozen.ClientID, string(domain.ScopeUser), plan.ActivePath) ||
		!reflect.DeepEqual(frozen.SelectedDelivery, plan.SelectedDelivery) ||
		!domain.SameProfileAuthority(frozen.ProfileAuthority, plan.ProfileAuthority()) {
		return false
	}
	if !completedUserPreparedBinding(frozen, plan, target.Activation, installation) {
		return false
	}
	if target.Activation.Activation == domain.ActivationPrepared {
		return publishedPreparedUserDelivery(plan)
	}
	return true
}

func completedUserPreparedBinding(frozen PlanTarget, plan domain.DeliveryPlan, outcome domain.ActivationOutcome, installation domain.Installation) bool {
	binding, found := installation.Clients[frozen.BindingID]
	if !found || binding.ClientBindingID != frozen.BindingID || binding.ClientID != frozen.ClientID ||
		binding.Scope != string(domain.ScopeUser) || binding.TargetLocator != plan.ActivePath ||
		binding.PhysicalArtifact != plan.PhysicalArtifactID ||
		binding.ProfileNamespace != plan.ProfileNamespace() ||
		!domain.SameProfileAuthority(binding.ProfileAuthority, plan.ProfileAuthority()) ||
		!reflect.DeepEqual(binding.SelectedDelivery, plan.SelectedDelivery) ||
		binding.NativeActivationAttempt != "" || binding.PendingNativeIntent != nil {
		return false
	}
	// The selected Local delivery binds its exact physical profile. Historical
	// CLI clients derive native scope from declared traits instead.
	nativeRoot := ""
	if authority := plan.ProfileAuthority(); authority != nil {
		nativeRoot = authority.Facts().CanonicalRoot
	} else if facts, ok := plan.SelectedDelivery.LocalFacts(); ok {
		nativeRoot = facts.ProfileRoot
	} else if plan.SelectedDelivery.EffectiveTraitsForScope(plan.ClientID, domain.ScopeUser).BindsNativeProfileRoot {
		nativeRoot = plan.NativeRegistryRoot
	}
	if binding.NativeProfileRoot != nativeRoot ||
		!completedUserPreparedOutcome(plan, binding, outcome) || !completedProjectLifecycle(binding, outcome) {
		return false
	}
	receipt, foundReceipt := installation.DataReceipts[binding.DataReceiptID]
	if binding.DataReceiptID == "" || !foundReceipt || receipt.DataReceiptID != binding.DataReceiptID ||
		receipt.Scope != string(domain.ScopeUser) || receipt.Locator == "" {
		return false
	}
	for key, other := range installation.Clients {
		if other.ClientID == frozen.ClientID && key != frozen.BindingID {
			return false
		}
	}
	return true
}

func publishedPreparedUserDelivery(plan domain.DeliveryPlan) bool {
	definition, supported := domain.ClientDefinitionFor(plan.ClientID)
	facts, selected := plan.SelectedDelivery.LocalFacts()
	if !supported || !selected || facts.ProjectionDigest == "" ||
		plan.SelectedDelivery.ValidatePlan(plan, facts.CanonicalDigest) != nil {
		return false
	}
	traits := plan.SelectedDelivery.EffectiveTraitsForScope(plan.ClientID, domain.ScopeUser)
	return definition.Capabilities.PackageMode == domain.PackagePrepared &&
		traits.LifecycleKind == domain.LifecycleNativeConfig && traits.TracksNativeEffects &&
		traits.BindsNativeProfileRoot && !traits.RequiresActiveNativeProjection &&
		facts.ProfileRoot == plan.NativeRegistryRoot
}

func completedUserPreparedOutcome(plan domain.DeliveryPlan, binding domain.ClientBinding, outcome domain.ActivationOutcome) bool {
	if outcome.Activation != domain.ActivationPrepared {
		return completedProjectOutcome(plan, outcome)
	}
	if !publishedPreparedUserDelivery(plan) ||
		outcome.Verification != domain.VerificationPackageValid ||
		outcome.Policy != domain.PolicyAllowed ||
		!completedProjectAuthentication(outcome.Authentication) ||
		!completedProjectEffect(plan, outcome.NativeEffect) ||
		outcome.ActivationAttested || outcome.AuthenticationAttested ||
		len(outcome.UserActions) == 0 || binding.LocalEntryObservation == nil ||
		binding.ValidateLocalEntryObservation() != nil ||
		!reflect.DeepEqual(binding.LocalEntryObservation.Facts().RevisionBasis, binding.SelectedDelivery) ||
		!binding.SelectedDelivery.OwnsProfileEntry(binding.NativeObjects) {
		return false
	}
	return true
}

func hasUserPreparedGroupTarget(got usecase.GroupResult) bool {
	for _, target := range got.Targets {
		if target.Plan.Scope == domain.ScopeUser && target.Activation.Activation == domain.ActivationPrepared {
			return true
		}
	}
	return false
}

func userPreparedGroupActions(got usecase.GroupResult) []string {
	var out []string
	for _, target := range got.Targets {
		if target.Plan.Scope == domain.ScopeUser && target.Activation.Activation == domain.ActivationPrepared {
			out = append(out, target.Activation.UserActions...)
		}
	}
	return out
}
