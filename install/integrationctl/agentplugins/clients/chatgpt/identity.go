package chatgpt

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

var _ clients.RegistryInspector = (*Adapter)(nil)

func (*Adapter) UsesNativeRegistryExecutable() bool { return false }

func (*Adapter) InspectNativeRegistry(ctx context.Context, _ clients.Env, _ domain.DetectedClient, plan domain.DeliveryPlan, managed *domain.ClientBinding) (clients.RegistryFinding, error) {
	if err := ctx.Err(); err != nil {
		return clients.RegistryIndeterminate, err
	}
	// ChatGPT's installed-plugin registry is remote and this adapter has no
	// authenticated read-only API. A validated explicit app binding (signed or
	// a personal Context7 receipt) authorizes only preparation of a new local
	// package, while an existing ownership receipt authorizes replacement of
	// that owned local package. Neither is treated as proof of remote
	// activation or registry availability.
	if plan.LocalPreparationAuthorized || managed != nil {
		return clients.RegistryClear, nil
	}
	return clients.RegistryIndeterminate, nil
}
