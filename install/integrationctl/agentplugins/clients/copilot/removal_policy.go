package copilot

import (
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

var _ clients.RemovalAttestationPolicy = (*Adapter)(nil)

// RequiresExternalUninstalled reports the existing Copilot cleanup contract.
func (*Adapter) RequiresExternalUninstalled(env clients.Env, request domain.DeactivationRequest) bool {
	return shared.CopilotRequiresExternalUninstalled(env, request)
}
