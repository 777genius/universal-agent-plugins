package windsurf

import (
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

var _ clients.RemovalAttestationPolicy = (*Adapter)(nil)

// RequiresExternalUninstalled preserves both existing manual-removal branches.
func (*Adapter) RequiresExternalUninstalled(_ clients.Env, request domain.DeactivationRequest) bool {
	if request.TargetContext.Scope == domain.ScopeProject {
		return false
	}
	return len(WindsurfObjects(request.NativeObjects)) == 0 || strings.TrimSpace(request.Client.ConfigRoot) == ""
}
