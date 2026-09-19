package usecase

import "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"

func nativeLifecycleClient(clientID domain.ClientID) bool {
	return domain.ClientTraitsFor(clientID).LifecycleKind == domain.LifecycleNativeConfig
}

func sharesPhysicalBackend(id domain.ClientID) bool {
	return len(domain.BackendSiblings(id)) > 0
}

func clientDisplayName(id domain.ClientID) string {
	definition, ok := domain.ClientDefinitionFor(id)
	if !ok || definition.DisplayName == "" {
		return string(id)
	}
	return definition.DisplayName
}
