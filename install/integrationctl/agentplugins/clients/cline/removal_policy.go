package cline

import (
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

var _ clients.RemovalAttestationPolicy = (*Adapter)(nil)

// RequiresExternalUninstalled keeps legacy unowned projections on the existing
// manual removal path; unrelated native object kinds cannot prove ownership.
func (*Adapter) RequiresExternalUninstalled(_ clients.Env, request domain.DeactivationRequest) bool {
	return len(ClineObjects(request.NativeObjects)) == 0
}
