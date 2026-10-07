package cline

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

var _ clients.Projector = (*Adapter)(nil)
var _ clients.ActiveNativeProjector = (*Adapter)(nil)

func (*Adapter) ProjectActiveNative(_ context.Context, root string, envelope domain.PackageEnvelope, plan domain.DeliveryPlan, _ string) ([]domain.NativeObjectOwnership, error) {
	return BuildClineNativeObjects(root, envelope, plan)
}

// Project writes Cline's native projection and records the objects it owns.
func (*Adapter) Project(_ context.Context, in clients.ProjectionInput) ([]domain.NativeObjectOwnership, error) {
	if err := ProjectClineNative(in.StagingPath, in.Envelope, in.Plan, in.PluginDataPath); err != nil {
		return nil, err
	}
	return BuildClineNativeObjects(in.StagingPath, in.Envelope, in.Plan)
}
