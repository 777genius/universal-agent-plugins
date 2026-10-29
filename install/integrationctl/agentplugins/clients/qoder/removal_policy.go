package qoder

import (
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

var _ clients.RemovalAttestationPolicy = (*Adapter)(nil)

// RequiresExternalUninstalled shares the runtime's exact unavailable-CLI rule.
func (*Adapter) RequiresExternalUninstalled(env clients.Env, request domain.DeactivationRequest) bool {
	if request.TargetContext.Scope == domain.ScopeProject || request.TargetContext.ProjectRoot != "" {
		return false
	}
	return !shared.HasClientCLI(env, request.BackendExecutable)
}
