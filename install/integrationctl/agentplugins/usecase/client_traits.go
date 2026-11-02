package usecase

import "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"

func nativeLifecycleClient(clientID domain.ClientID, selected domain.SelectedDelivery, scopes ...domain.InstallScope) bool {
	traits := selected.EffectiveTraitsForScope(clientID, selectedScope(scopes))
	return traits.TracksNativeEffects || traits.LifecycleKind == domain.LifecycleNativeConfig
}

func sharesPhysicalBackend(id domain.ClientID, selected domain.SelectedDelivery, scopes ...domain.InstallScope) bool {
	return selected.SharesBackendForScope(id, selectedScope(scopes))
}

func selectedScope(scopes []domain.InstallScope) domain.InstallScope {
	if len(scopes) == 0 || scopes[0] == "" {
		return domain.ScopeUser
	}
	return scopes[0]
}

func clientDisplayName(id domain.ClientID) string {
	definition, ok := domain.ClientDefinitionFor(id)
	if !ok || definition.DisplayName == "" {
		return string(id)
	}
	return definition.DisplayName
}
