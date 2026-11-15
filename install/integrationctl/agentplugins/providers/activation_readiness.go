package providers

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
)

var _ ports.ActivationReadinessInspector = Activator{}

// RequiresActivation delegates optional read-only configuration observations
// to the selected client. Existing adapters retain projection-only behavior.
func (activator Activator) RequiresActivation(ctx context.Context, client domain.DetectedClient, plan domain.DeliveryPlan) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if err := activator.requireRegistry(); err != nil {
		return false, err
	}
	readiness, ok := clients.As[clients.ActivationReadiness](activator.Registry, client.ClientID)
	if !ok {
		return false, nil
	}
	return readiness.RequiresActivation(ctx, activator.env(), client, plan)
}
