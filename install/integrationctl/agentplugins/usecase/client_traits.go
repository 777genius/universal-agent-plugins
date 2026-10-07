package usecase

import "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"

func nativeLifecycleClient(clientID domain.ClientID, selected domain.SelectedDelivery) bool {
	traits := selected.EffectiveTraits(clientID)
	return traits.TracksNativeEffects || traits.LifecycleKind == domain.LifecycleNativeConfig
}

func sharesPhysicalBackend(id domain.ClientID, selected domain.SelectedDelivery) bool {
	return selected.SharesBackend(id)
}

func clientDisplayName(id domain.ClientID) string {
	definition, ok := domain.ClientDefinitionFor(id)
	if !ok || definition.DisplayName == "" {
		return string(id)
	}
	return definition.DisplayName
}
