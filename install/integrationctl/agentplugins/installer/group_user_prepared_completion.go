package installer

import (
	"reflect"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

// completedUserPreparedUpdate qualifies only the file operation for an existing
// user-scope group. It never attests client activation, trust or native loading.
// The host callback must already have succeeded before this proof is consulted.
func completedUserPreparedUpdate(prepared *PreparedOperation, got usecase.GroupResult, state domain.StateFileV2) bool {
	if prepared.req.Operation != OpUpdate || (prepared.req.Scope != "" && prepared.req.Scope != domain.ScopeUser) ||
		prepared.req.ProjectRoot != "" || got.Phase != usecase.GroupPhaseCompleted ||
		got.InstallationID == "" || got.InstallationID != prepared.req.InstallationID ||
		got.InstallationID != prepared.plan.InstallationID {
		return false
	}
	size := len(prepared.plan.Targets)
	if size != 2 || len(got.Targets) != size || len(prepared.clients) != size || len(prepared.req.Targets) != size {
		return false
	}
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
		if !ok || frozen.ClientID == "" || frozen.ClientID != prepared.req.Targets[i].ClientID ||
			frozen.ClientID != string(prepared.clients[i].ClientID) ||
			frozen.ConfigRoot != prepared.clients[i].ConfigRoot {
			return false
		}
		delete(targets, frozen.ClientID)
		plan := target.Plan
		context, contextErr := domain.NewTargetContext(domain.ScopeUser, "", frozen.ConfigRoot)
		if contextErr != nil || plan.Scope != domain.ScopeUser || plan.TargetContext != context ||
			plan.ActivePath == "" || plan.ActivePath != frozen.TargetPath || plan.PhysicalArtifactID == "" ||
			frozen.BindingID == "" ||
			frozen.BindingID != domain.ComputeClientBindingID(got.InstallationID, frozen.ClientID, string(domain.ScopeUser), plan.ActivePath) ||
			!reflect.DeepEqual(frozen.SelectedDelivery, plan.SelectedDelivery) ||
			!domain.SameProfileAuthority(frozen.ProfileAuthority, plan.ProfileAuthority()) {
			return false
		}
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
		nativeRoot := plan.NativeRegistryRoot
		if authority := plan.ProfileAuthority(); authority != nil {
			nativeRoot = authority.Facts().CanonicalRoot
		}
		if binding.NativeProfileRoot != nativeRoot ||
			!completedProjectOutcome(plan, target.Activation) ||
			!completedProjectLifecycle(binding, target.Activation) {
			return false
		}
		for key, other := range installation.Clients {
			if other.ClientID == frozen.ClientID && key != frozen.BindingID {
				return false
			}
		}
		if target.Activation.Activation == domain.ActivationPrepared {
			definition, supported := domain.ClientDefinitionFor(plan.ClientID)
			traits := plan.SelectedDelivery.EffectiveTraitsForScope(plan.ClientID, domain.ScopeUser)
			// Only the published, frozen user Local prepared-package route is
			// eligible. Pi and any tracked/active-native route remain fail-closed.
			if !supported || definition.Capabilities.PackageMode != domain.PackagePrepared ||
				traits.LifecycleKind != domain.LifecycleCLIRegistry ||
				traits.TracksNativeEffects || traits.RequiresActiveNativeProjection {
				return false
			}
			hasPrepared = true
		}
	}
	return hasPrepared && len(targets) == 0
}
