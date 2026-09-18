package cline

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

var _ clients.RegistryInspector = (*Adapter)(nil)

func (*Adapter) UsesNativeRegistryExecutable() bool { return false }

func (*Adapter) InspectNativeRegistry(ctx context.Context, _ clients.Env, _ domain.DetectedClient, _ domain.DeliveryPlan, _ *domain.ClientBinding) (clients.RegistryFinding, error) {
	if err := ctx.Err(); err != nil {
		return clients.RegistryIndeterminate, err
	}
	// Cline MCP identities are per-server and protected by exact receipts;
	// its skill paths are checked before the all-or-none native config batch.
	return clients.RegistryClear, nil
}
