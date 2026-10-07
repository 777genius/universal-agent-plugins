package providers

import (
	"context"
	"fmt"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// ProjectActiveNative recomputes the selected native projection in place from
// the exact installed package. The package digest is checked before reading any
// projected files; unlike Project, these builders never write to the tree.
func (stager Stager) ProjectActiveNative(ctx context.Context, envelope domain.PackageEnvelope, plan domain.DeliveryPlan, expectedDigest, dataPath string) ([]domain.NativeObjectOwnership, error) {
	if err := stager.requireDeps(); err != nil {
		return nil, err
	}
	if err := stager.Verify(ctx, plan.ActivePath, expectedDigest); err != nil {
		return nil, fmt.Errorf("verify active package before native projection: %w", err)
	}
	projector, ok := clients.As[clients.ActiveNativeProjector](stager.Registry, plan.ClientID)
	if !ok {
		return nil, fmt.Errorf("client %q has no read-only native projection", plan.ClientID)
	}
	return projector.ProjectActiveNative(ctx, plan.ActivePath, envelope, plan, dataPath)
}
