package agentpluginscli

import "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"

func clientTraits(id domain.ClientID, scopes ...domain.InstallScope) domain.ClientTraits {
	return (domain.SelectedDelivery{}).EffectiveTraitsForScope(id, selectedScope(scopes))
}

func clientDefinition(id domain.ClientID) (domain.ClientDefinition, bool) {
	return domain.ClientDefinitionFor(id)
}

func plansWithoutHostPresence(id domain.ClientID) bool {
	definition, ok := clientDefinition(id)
	return ok && definition.PlansWithoutHostPresence
}

func requiresPersonalMapping(id domain.ClientID) bool {
	return clientTraits(id).RequiresPersonalMappingForPrepare
}

func allowsPrepare(id domain.ClientID) bool {
	return clientTraits(id).Allows(domain.InstallIntentPrepare)
}

func allowsHostedPrepare(id domain.ClientID) bool {
	traits := clientTraits(id)
	return traits.Allows(domain.InstallIntentPrepare) && !traits.RequiresPersonalMappingForPrepare
}

func reportsMCPToolNamespaceCollision(id domain.ClientID) bool {
	return clientTraits(id).ReportsMCPToolNamespaceCollision
}

func nativeConfigLifecycle(id domain.ClientID, scopes ...domain.InstallScope) bool {
	return clientTraits(id, scopes...).LifecycleKind == domain.LifecycleNativeConfig
}

func sharesPhysicalBackend(id domain.ClientID, scopes ...domain.InstallScope) bool {
	return len(domain.BackendSiblingsForScope(id, selectedScope(scopes))) > 0
}

func nativeCLIRegistryOwner(id domain.ClientID, scopes ...domain.InstallScope) domain.ClientID {
	if owner, ok := nativePackageSibling(id, scopes...); ok {
		return owner
	}
	return id
}

func nativePackageSibling(id domain.ClientID, scopes ...domain.InstallScope) (domain.ClientID, bool) {
	candidates := append([]domain.ClientID{id}, domain.BackendSiblingsForScope(id, selectedScope(scopes))...)
	for _, candidate := range candidates {
		definition, ok := clientDefinition(candidate)
		if ok && definition.Capabilities.PackageMode == domain.PackageNative && len(domain.BackendSiblingsForScope(candidate, selectedScope(scopes))) > 0 {
			return candidate, true
		}
	}
	return "", false
}

func personalMappingPrepareSelected(intents map[domain.ClientID]domain.InstallIntent) bool {
	for id, intent := range intents {
		if intent == domain.InstallIntentPrepare && requiresPersonalMapping(id) {
			return true
		}
	}
	return false
}

func personalMappingTarget(targets []domain.ClientID) (domain.ClientID, bool) {
	for _, target := range targets {
		if requiresPersonalMapping(target) {
			return target, true
		}
	}
	return "", false
}

func containsPersonalMappingTarget(targets []domain.ClientID) bool {
	_, ok := personalMappingTarget(targets)
	return ok
}

func withoutPersonalMappingTargets(targets []domain.ClientID) []domain.ClientID {
	filtered := make([]domain.ClientID, 0, len(targets))
	for _, target := range targets {
		if !requiresPersonalMapping(target) {
			filtered = append(filtered, target)
		}
	}
	return filtered
}

func directoryPreparationClient(targets []domain.ClientID) (domain.ClientID, domain.DirectoryResolvePurpose, bool) {
	for _, target := range targets {
		definition, ok := clientDefinition(target)
		if ok && definition.DirectoryPreparationPurpose != "" {
			return target, definition.DirectoryPreparationPurpose, true
		}
	}
	return "", "", false
}

func assignPrepareIntents(intents map[domain.ClientID]domain.InstallIntent) {
	for _, id := range domain.SupportedClientIDs() {
		if allowsPrepare(id) {
			intents[id] = domain.InstallIntentPrepare
		}
	}
}

func skipVersionProbeForPrepare(id domain.ClientID, intents map[domain.ClientID]domain.InstallIntent) bool {
	return intents[id] == domain.InstallIntentPrepare && allowsHostedPrepare(id)
}

func appendBackendSiblings(values []string, id domain.ClientID, scopes ...domain.InstallScope) []string {
	for _, sibling := range domain.BackendSiblingsForScope(id, selectedScope(scopes)) {
		values = append(values, string(sibling))
	}
	return values
}

func expandBackendSiblingTargets(targets []domain.ClientID, scopes ...domain.InstallScope) []domain.ClientID {
	result := append([]domain.ClientID(nil), targets...)
	for _, target := range targets {
		result = append(result, domain.BackendSiblingsForScope(target, selectedScope(scopes))...)
	}
	return result
}

func syntheticUndetectedClient(id domain.ClientID) domain.DetectedClient {
	return domain.DetectedClient{
		ClientID:    id,
		DisplayName: domain.ClientDisplayName(id),
		Status:      domain.DetectionNotDetected,
	}
}
