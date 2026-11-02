package installer

import (
	"fmt"
	"reflect"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

// completedProjectGroup checks the existing fresh operation and saved identity
// before reporting file completion. It neither activates clients nor observes
// files: the core already performed those operations and retained their facts.
func completedProjectGroup(prepared *PreparedOperation, got usecase.GroupResult, state domain.StateFileV2) error {
	if got.Phase != usecase.GroupPhaseCompleted || got.InstallationID == "" ||
		got.InstallationID != prepared.plan.InstallationID || got.InstallationID != prepared.req.InstallationID || prepared.req.ProjectRoot == "" {
		return fmt.Errorf("%w: project group operation is not complete", ErrIncomplete)
	}
	size := len(prepared.plan.Targets)
	if size == 0 || len(got.Targets) != size || len(prepared.clients) != size || len(prepared.req.Targets) != size {
		return fmt.Errorf("%w: project group target set is inconsistent", ErrIncomplete)
	}
	installation, err := completedGroupInstallation(state, got.InstallationID)
	if err != nil {
		return err
	}
	targets := make(map[string]usecase.AddResult, size)
	for _, target := range got.Targets {
		id := string(target.Plan.ClientID)
		if _, duplicate := targets[id]; duplicate || id == "" {
			return fmt.Errorf("%w: project group target identity is ambiguous", ErrIncomplete)
		}
		targets[id] = target
	}
	for i, frozen := range prepared.plan.Targets {
		target, ok := targets[frozen.ClientID]
		if !ok || frozen.ClientID != string(prepared.clients[i].ClientID) || frozen.ClientID != prepared.req.Targets[i].ClientID {
			return fmt.Errorf("%w: project group target differs from preparation", ErrIncomplete)
		}
		delete(targets, frozen.ClientID)
		if err := completedProjectTarget(prepared, frozen, target, installation); err != nil {
			return err
		}
	}
	return nil
}

func completedGroupInstallation(state domain.StateFileV2, id string) (domain.Installation, error) {
	var found domain.Installation
	count := 0
	for _, installation := range state.Installations {
		if installation.InstallationID == id {
			found, count = installation, count+1
		}
	}
	if count != 1 {
		return domain.Installation{}, fmt.Errorf("%w: committed project installation is missing or ambiguous", ErrIncomplete)
	}
	return found, nil
}

func completedProjectTarget(prepared *PreparedOperation, frozen PlanTarget, target usecase.AddResult, installation domain.Installation) error {
	if target.InstallationID != installation.InstallationID || target.GroupPhase != usecase.GroupTargetExternalCompleted || target.Failure != nil {
		return fmt.Errorf("%w: project target operation is not complete", ErrIncomplete)
	}
	if err := completedProjectPlan(prepared, frozen, target.Plan); err != nil {
		return err
	}
	binding, ok := installation.Clients[frozen.BindingID]
	if !ok || !completedProjectBindingIdentity(frozen, target.Plan, binding) {
		return fmt.Errorf("%w: committed project binding differs from completed target", ErrIncomplete)
	}
	for key, other := range installation.Clients {
		if other.ClientID == frozen.ClientID && key != frozen.BindingID {
			return fmt.Errorf("%w: committed project client has ambiguous bindings", ErrIncomplete)
		}
	}
	if binding.NativeActivationAttempt != "" || binding.PendingNativeIntent != nil {
		return fmt.Errorf("%w: committed project binding retains an unresolved native attempt", ErrIncomplete)
	}
	if !completedProjectOutcome(target.Plan, target.Activation) || !completedProjectLifecycle(binding, target.Activation) {
		return fmt.Errorf("%w: project target lacks a verified completed operation", ErrIncomplete)
	}
	return nil
}

func completedProjectPlan(prepared *PreparedOperation, frozen PlanTarget, plan domain.DeliveryPlan) error {
	context := preparedTargetContext(prepared, frozen.ClientID)
	bindingID := domain.ComputeClientBindingID(prepared.plan.InstallationID, frozen.ClientID, string(domain.ScopeProject), plan.ActivePath)
	if plan.Scope != domain.ScopeProject || plan.TargetContext != context || context.Scope != domain.ScopeProject ||
		context.UserConfigRoot != frozen.ConfigRoot || frozen.BindingID == "" || frozen.BindingID != bindingID ||
		plan.ActivePath == "" || plan.ActivePath != frozen.TargetPath || plan.PhysicalArtifactID == "" {
		return fmt.Errorf("%w: project result differs from the prepared context", ErrIncomplete)
	}
	if !reflect.DeepEqual(frozen.SelectedDelivery, plan.SelectedDelivery) || !domain.SameProfileAuthority(frozen.ProfileAuthority, plan.ProfileAuthority()) {
		return fmt.Errorf("%w: project result differs from the prepared authority", ErrIncomplete)
	}
	matched := false
	for _, delivery := range prepared.deliveries {
		if string(delivery.plan.ClientID) != frozen.ClientID {
			continue
		}
		matched = true
		if !completedProjectDeliveryIdentity(delivery.plan, plan) {
			return fmt.Errorf("%w: project result differs from the frozen delivery", ErrIncomplete)
		}
	}
	if !matched {
		return fmt.Errorf("%w: project result has no frozen delivery", ErrIncomplete)
	}
	return nil
}

func completedProjectDeliveryIdentity(frozen, current domain.DeliveryPlan) bool {
	return frozen.ClientID == current.ClientID && frozen.Scope == current.Scope && frozen.TargetContext == current.TargetContext &&
		frozen.PhysicalArtifactID == current.PhysicalArtifactID && frozen.ActivePath == current.ActivePath &&
		frozen.NativeRegistryRoot == current.NativeRegistryRoot && frozen.ProfileNamespace() == current.ProfileNamespace() &&
		domain.SameProfileAuthority(frozen.ProfileAuthority(), current.ProfileAuthority()) && reflect.DeepEqual(frozen.SelectedDelivery, current.SelectedDelivery)
}

func completedProjectBindingIdentity(frozen PlanTarget, plan domain.DeliveryPlan, binding domain.ClientBinding) bool {
	root := plan.NativeRegistryRoot
	if authority := plan.ProfileAuthority(); authority != nil {
		root = authority.Facts().CanonicalRoot
	}
	return binding.ClientBindingID == frozen.BindingID && binding.ClientID == frozen.ClientID && binding.Scope == string(domain.ScopeProject) &&
		binding.TargetLocator == plan.ActivePath && binding.PhysicalArtifact == plan.PhysicalArtifactID && binding.NativeProfileRoot == root &&
		binding.ProfileNamespace == plan.ProfileNamespace() && domain.SameProfileAuthority(binding.ProfileAuthority, plan.ProfileAuthority()) &&
		reflect.DeepEqual(binding.SelectedDelivery, plan.SelectedDelivery)
}

func completedProjectLifecycle(binding domain.ClientBinding, outcome domain.ActivationOutcome) bool {
	return binding.Materialization == domain.MaterializationMaterialized && binding.Activation == outcome.Activation &&
		binding.Authentication == outcome.Authentication && binding.Policy == outcome.Policy && binding.Verification == outcome.Verification
}

func completedProjectOutcome(plan domain.DeliveryPlan, outcome domain.ActivationOutcome) bool {
	if outcome.Policy != domain.PolicyAllowed || !completedProjectAuthentication(outcome.Authentication) || !completedProjectEffect(plan, outcome.NativeEffect) {
		return false
	}
	switch outcome.Activation {
	case domain.ActivationPrepared:
		return outcome.Verification == domain.VerificationInstalled && !outcome.ActivationAttested && len(outcome.UserActions) != 0
	case domain.ActivationActive, domain.ActivationNotRequired:
		return outcome.Verification == domain.VerificationInstalled || outcome.Verification == domain.VerificationRuntime
	default:
		return false
	}
}

func completedProjectAuthentication(authentication domain.AuthenticationState) bool {
	switch authentication {
	case domain.AuthenticationNotRequired, domain.AuthenticationNotChecked, domain.AuthenticationPending, domain.AuthenticationComplete:
		return true
	default:
		return false
	}
}

func completedProjectEffect(plan domain.DeliveryPlan, effect domain.NativeEffectState) bool {
	switch effect {
	case domain.NativeEffectUnchanged, domain.NativeEffectCommitted:
		return true
	case "":
		traits := plan.SelectedDelivery.EffectiveTraitsForScope(plan.ClientID, plan.Scope)
		return !traits.TracksNativeEffects && (traits.LifecycleKind == domain.LifecycleCLIRegistry || traits.LifecycleKind == domain.LifecyclePrepared)
	default:
		return false
	}
}
