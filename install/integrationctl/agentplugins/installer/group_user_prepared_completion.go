package installer

import (
	"fmt"
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

// validUserPreparedGroupShape bounds the exception to exact user Update groups.
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

// completedUserPreparedTarget matches one frozen selected target to core results.
func completedUserPreparedTarget(prepared *PreparedOperation, frozen PlanTarget, i int, target usecase.AddResult, installation domain.Installation) bool {
	plan := target.Plan
	if !validUserPreparedTargetIdentity(prepared, frozen, i, plan) {
		return false
	}
	// The immutable pre-confirmation delivery must agree with the core's
	// completed result; a different native target cannot borrow its proof.
	matched := false
	for _, delivery := range prepared.deliveries {
		if string(delivery.plan.ClientID) != frozen.ClientID {
			continue
		}
		if matched || !completedUserPreparedDeliveryIdentity(delivery.plan, plan) {
			return false
		}
		matched = true
	}
	return matched && completedUserPreparedBinding(frozen, plan, target.Activation, installation)
}

// validUserPreparedTargetIdentity compares frozen user target and core plan.
func validUserPreparedTargetIdentity(prepared *PreparedOperation, frozen PlanTarget, i int, plan domain.DeliveryPlan) bool {
	if frozen.ClientID == "" || frozen.ClientID != prepared.req.Targets[i].ClientID ||
		frozen.ClientID != string(prepared.clients[i].ClientID) ||
		frozen.ConfigRoot != prepared.clients[i].ConfigRoot {
		return false
	}
	context, err := domain.NewTargetContext(domain.ScopeUser, "", frozen.ConfigRoot)
	return err == nil && plan.Scope == domain.ScopeUser && plan.TargetContext == context &&
		plan.ActivePath != "" && plan.ActivePath == frozen.TargetPath && plan.PhysicalArtifactID != "" &&
		frozen.BindingID != "" &&
		frozen.BindingID == domain.ComputeClientBindingID(prepared.plan.InstallationID, frozen.ClientID, string(domain.ScopeUser), plan.ActivePath) &&
		completedUserStagedSelection(frozen.SelectedDelivery, plan.SelectedDelivery) &&
		domain.SameProfileAuthority(frozen.ProfileAuthority, plan.ProfileAuthority())
}

// completedUserStagedSelection permits only the digest added by the actual
// stager. All pre-staging selected Local authority must remain byte-identical.
func completedUserStagedSelection(frozen, current domain.SelectedDelivery) bool {
	if reflect.DeepEqual(frozen, current) {
		return true
	}
	if frozen.IsZero() || current.IsZero() || frozen.ProjectionDigest() != "" || current.ProjectionDigest() == "" {
		return false
	}
	sealed, err := frozen.WithProjectionDigest(current.ProjectionDigest())
	return err == nil && reflect.DeepEqual(sealed, current)
}

// completedUserPreparedDeliveryIdentity retains every frozen plan field while
// allowing the SDK's single staged-projection digest seal.
func completedUserPreparedDeliveryIdentity(frozen, current domain.DeliveryPlan) bool {
	if !completedUserStagedSelection(frozen.SelectedDelivery, current.SelectedDelivery) {
		return false
	}
	frozen.SelectedDelivery = current.SelectedDelivery
	return completedProjectDeliveryIdentity(frozen, current)
}

// completedUserPreparedBinding checks the loaded owned binding and data receipt.
func completedUserPreparedBinding(frozen PlanTarget, plan domain.DeliveryPlan, outcome domain.ActivationOutcome, installation domain.Installation) bool {
	binding, found := installation.Clients[frozen.BindingID]
	if !found || !validUserPreparedBindingRecord(frozen, plan, binding) {
		return false
	}
	// The returned core Plan is the pre-staging snapshot. Only persisted
	// SelectedDelivery can carry the exact digest sealed by the stager.
	sealed := plan
	sealed.SelectedDelivery = binding.SelectedDelivery
	if binding.NativeProfileRoot != expectedUserNativeRoot(sealed) ||
		!completedUserPreparedOutcome(sealed, binding, outcome) || !completedProjectLifecycle(binding, outcome) ||
		!ownedUserPreparedDataReceipt(binding, installation) {
		return false
	}
	for key, other := range installation.Clients {
		if other.ClientID == frozen.ClientID && key != frozen.BindingID {
			return false
		}
	}
	return true
}

// validUserPreparedBindingRecord refuses mismatched or pending saved authority.
func validUserPreparedBindingRecord(frozen PlanTarget, plan domain.DeliveryPlan, binding domain.ClientBinding) bool {
	return binding.ClientBindingID == frozen.BindingID && binding.ClientID == frozen.ClientID &&
		binding.Scope == string(domain.ScopeUser) && binding.TargetLocator == plan.ActivePath &&
		binding.PhysicalArtifact == plan.PhysicalArtifactID &&
		binding.ProfileNamespace == plan.ProfileNamespace() &&
		domain.SameProfileAuthority(binding.ProfileAuthority, plan.ProfileAuthority()) &&
		completedUserStagedSelection(plan.SelectedDelivery, binding.SelectedDelivery) &&
		binding.NativeActivationAttempt == "" && binding.PendingNativeIntent == nil
}

// expectedUserNativeRoot preserves the selected Local physical profile token.
func expectedUserNativeRoot(plan domain.DeliveryPlan) string {
	if authority := plan.ProfileAuthority(); authority != nil {
		return authority.Facts().CanonicalRoot
	}
	if facts, ok := plan.SelectedDelivery.LocalFacts(); ok {
		return facts.ProfileRoot
	}
	if plan.SelectedDelivery.EffectiveTraitsForScope(plan.ClientID, domain.ScopeUser).BindsNativeProfileRoot {
		return plan.NativeRegistryRoot
	}
	return ""
}

// ownedUserPreparedDataReceipt rejects missing, stale or indeterminate data.
func ownedUserPreparedDataReceipt(binding domain.ClientBinding, installation domain.Installation) bool {
	receipt, found := installation.DataReceipts[binding.DataReceiptID]
	return binding.DataReceiptID != "" && found && receipt.DataReceiptID == binding.DataReceiptID &&
		receipt.Scope == string(domain.ScopeUser) && receipt.Locator != "" &&
		receipt.State == domain.DataReceiptOwned && receipt.OwnershipDigest != ""
}

// publishedPreparedUserDelivery admits only the qualified selected Local route.
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

// completedUserPreparedOutcome keeps Local prepared while proving its recorded selector.
func completedUserPreparedOutcome(plan domain.DeliveryPlan, binding domain.ClientBinding, outcome domain.ActivationOutcome) bool {
	if outcome.Activation == domain.ActivationManual {
		// A user CLI registry may have finished writing its owned files while
		// native registration still requires a real manual step. Do not attest
		// activation, and do not extend this to project or native-config routes.
		traits := plan.SelectedDelivery.EffectiveTraitsForScope(plan.ClientID, domain.ScopeUser)
		return plan.Scope == domain.ScopeUser && traits.LifecycleKind == domain.LifecycleCLIRegistry &&
			outcome.Verification == domain.VerificationPackageValid && outcome.Policy == domain.PolicyAllowed &&
			completedProjectAuthentication(outcome.Authentication) && completedProjectEffect(plan, outcome.NativeEffect) &&
			!outcome.ActivationAttested && !outcome.AuthenticationAttested && len(outcome.UserActions) != 0
	}
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

// hasUserPreparedGroupTarget stops lost bindings from bypassing the pending check.
func hasUserPreparedGroupTarget(got usecase.GroupResult) bool {
	for _, target := range got.Targets {
		if target.Plan.Scope == domain.ScopeUser && target.Activation.Activation == domain.ActivationPrepared {
			return true
		}
	}
	return false
}

// userPreparedGroupActions preserves verified next steps without activation claims.
func userPreparedGroupActions(got usecase.GroupResult) []string {
	var out []string
	for _, target := range got.Targets {
		if target.Plan.Scope == domain.ScopeUser && target.Activation.Activation == domain.ActivationPrepared {
			out = append(out, target.Activation.UserActions...)
		}
	}
	return out
}

// finishUserPreparedHandoff reports verified file completion separately from
// native activation. A pending handoff stays pending on every unproven route.
func (e *Engine) finishUserPreparedHandoff(prepared *PreparedOperation, result *Result, got usecase.GroupResult, state domain.StateFileV2, loadErr error) (bool, error) {
	if loadErr != nil {
		result.Outcome, result.Reason = OutcomeIncomplete, "committed group state is unknown"
		return true, loadErr
	}
	if !e.anyGroupHostHandoffPending(prepared) && !hasUserPreparedGroupTarget(got) {
		return true, nil
	}
	if e.cfg.OnCommittedBinding == nil || !completedUserPreparedUpdate(prepared, got, state) {
		result.Outcome = OutcomeIncomplete
		result.Reason = "committed binding is not activated"
		return true, fmt.Errorf("%s", result.Reason)
	}
	// Local stays Prepared; only the fully verified file update is complete.
	result.ManualActions = append(result.ManualActions, userPreparedGroupActions(got)...)
	return false, nil
}
