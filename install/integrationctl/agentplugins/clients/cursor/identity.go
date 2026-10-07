package cursor

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
	// This manual preparation contract has no managed executable registration.
	// Clear leaves the existing prepared-directory collision walk in charge;
	// UsesNativeRegistryExecutable=false means no native discovery was attempted.
	// It does not prove discovery of plugins/local or absence of other sources.
	return clients.RegistryClear, nil
}
