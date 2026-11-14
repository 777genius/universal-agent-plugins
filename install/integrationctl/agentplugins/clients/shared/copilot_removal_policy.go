package shared

import (
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// CopilotRequiresExternalUninstalled exposes the two existing manual branches of
// CompleteCopilotDeactivation without changing that frozen shared runtime path.
// Contract tests compare this read-only fact with its actual deactivation result.
func CopilotRequiresExternalUninstalled(env clients.Env, request domain.DeactivationRequest) bool {
	return !HasClientCLI(env, request.BackendExecutable) || request.CurrentActivation != domain.ActivationActive
}
