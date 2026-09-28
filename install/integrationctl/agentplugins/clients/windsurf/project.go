package windsurf

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

var _ clients.Projector = (*Adapter)(nil)
var _ clients.ActiveNativeProjector = (*Adapter)(nil)

func (*Adapter) ProjectActiveNative(_ context.Context, root string, _ domain.PackageEnvelope, plan domain.DeliveryPlan, _ string) ([]domain.NativeObjectOwnership, error) {
	return BuildWindsurfNativeObjects(root, plan)
}

// Project copies the managed stdio launcher when needed, writes Windsurf MCP,
// and records the objects it owns.
func (*Adapter) Project(_ context.Context, in clients.ProjectionInput) ([]domain.NativeObjectOwnership, error) {
	if err := shared.DeliverManagedStdio(in.DeliverLauncher(), in.StagingPath, in.Envelope, in.Plan); err != nil {
		return nil, err
	}
	if err := ProjectWindsurfMCP(in.StagingPath, in.Envelope, in.Plan, in.PluginDataPath); err != nil {
		return nil, err
	}
	return BuildWindsurfNativeObjects(in.StagingPath, in.Plan)
}
