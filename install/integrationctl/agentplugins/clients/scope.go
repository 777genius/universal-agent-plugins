package clients

import "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"

// SupportsScope checks declared capability and concrete project hooks without
// loading package bytes or touching native configuration.
func SupportsScope(registry *Registry, id domain.ClientID, scope domain.InstallScope) bool {
	if registry == nil {
		return false
	}
	if _, ok := registry.Lookup(id); !ok {
		return false
	}
	definition, ok := domain.ClientDefinitionFor(id)
	if !ok {
		return false
	}
	declared := false
	for _, candidate := range definition.Capabilities.Scopes {
		declared = declared || candidate == scope
	}
	if !declared || scope != domain.ScopeProject {
		return declared
	}
	if !definition.ProjectPolicy.Valid() || definition.PlansWithoutHostPresence || definition.Traits.RequiresPersonalMappingForPrepare {
		return false
	}
	_, layout := As[ProjectTargetLayout](registry, id)
	_, native := As[NativeRegistryLayout](registry, id)
	return layout && native
}
