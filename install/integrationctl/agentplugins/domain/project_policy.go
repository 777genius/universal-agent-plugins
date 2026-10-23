package domain

// ProjectPolicy is a qualified client's scoped lifecycle declaration. Its zero
// value enables nothing. Client stacks add it together with project layouts;
// user traits and physical backend sharing remain unchanged.
type ProjectPolicy struct {
	LifecycleKind LifecycleKind `json:"-"`
	BackendFamily string        `json:"-"`
}

func (policy ProjectPolicy) Valid() bool {
	return policy.BackendFamily != "" && (policy.LifecycleKind == LifecycleNativeConfig ||
		policy.LifecycleKind == LifecycleCLIRegistry || policy.LifecycleKind == LifecyclePrepared)
}

// withProjectScope is used only by individually qualified client table rows.
// Projects have independent physical registries unless explicitly declared.
func withProjectScope(definition ClientDefinition, kind LifecycleKind) ClientDefinition {
	definition.ProjectPolicy = ProjectPolicy{LifecycleKind: kind, BackendFamily: string(definition.ID)}
	definition.Capabilities.Scopes = append(definition.Capabilities.Scopes, ScopeProject)
	return definition
}

// EffectiveTraitsForScope never borrows a user policy for project delivery.
// Admission must reject an absent project declaration before mutation.
func (delivery SelectedDelivery) EffectiveTraitsForScope(id ClientID, scope InstallScope) ClientTraits {
	if scope != ScopeProject {
		return delivery.EffectiveTraits(id)
	}
	definition, ok := ClientDefinitionFor(id)
	if !ok || !definition.ProjectPolicy.Valid() || !delivery.IsZero() {
		return ClientTraits{}
	}
	traits := definition.Traits
	traits.LifecycleKind = definition.ProjectPolicy.LifecycleKind
	traits.SavedDependenciesDeferred = false
	if traits.LifecycleKind == LifecycleNativeConfig {
		traits.TracksNativeEffects = true
		traits.BindsNativeProfileRoot = true
		traits.SupportsPreparedRecovery = true
		traits.UsesManagedStdioLauncher = true
	}
	return traits
}

func BackendFamilyForScope(id ClientID, scope InstallScope) string {
	definition, ok := ClientDefinitionFor(id)
	if !ok {
		return ""
	}
	if scope == ScopeProject {
		if !definition.ProjectPolicy.Valid() {
			return ""
		}
		return definition.ProjectPolicy.BackendFamily
	}
	return definition.BackendFamily
}

func SameClientBackendForScope(first, second ClientID, scope InstallScope) bool {
	if first == second {
		return true
	}
	left := BackendFamilyForScope(first, scope)
	return left != "" && left == BackendFamilyForScope(second, scope)
}

func BackendSiblingsForScope(id ClientID, scope InstallScope) []ClientID {
	if scope != ScopeProject {
		return BackendSiblings(id)
	}
	var siblings []ClientID
	for _, candidate := range clientDefinitions {
		if candidate.ID != id && SameClientBackendForScope(id, candidate.ID, scope) {
			siblings = append(siblings, candidate.ID)
		}
	}
	return siblings
}

func (delivery SelectedDelivery) SharesBackendForScope(id ClientID, scope InstallScope) bool {
	if scope != ScopeProject {
		return delivery.SharesBackend(id)
	}
	return delivery.IsZero() && len(BackendSiblingsForScope(id, scope)) > 0
}
