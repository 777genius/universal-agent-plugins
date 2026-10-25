package installer

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
)

// Optional live discovery cannot disable project filesystem ownership checks.
// Both entry points deliberately retain the provider's prepared semantics.
type preparedProjectIdentityObserver struct {
	identity providers.NativeIdentityObserver
}

func (o preparedProjectIdentityObserver) ObserveNativeIdentity(ctx context.Context, client domain.DetectedClient, plan domain.DeliveryPlan, managed *domain.ClientBinding) (domain.NativeIdentityObservation, error) {
	return o.identity.ObservePreparedIdentity(ctx, client, plan, managed)
}

func (o preparedProjectIdentityObserver) ObservePreparedIdentity(ctx context.Context, client domain.DetectedClient, plan domain.DeliveryPlan, managed *domain.ClientBinding) (domain.NativeIdentityObservation, error) {
	return o.identity.ObservePreparedIdentity(ctx, client, plan, managed)
}
