package cline

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

var _ clients.Projector = (*Adapter)(nil)
var _ clients.ActiveNativeProjector = (*Adapter)(nil)

func (*Adapter) ProjectActiveNative(ctx context.Context, root string, envelope domain.PackageEnvelope, plan domain.DeliveryPlan, _ string) ([]domain.NativeObjectOwnership, error) {
	if clineProjectSelected(plan) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if err := validateClineProjection(root, plan, true); err != nil {
			return nil, err
		}
		return clineProjectObjects(root, envelope, plan)
	}
	return BuildClineNativeObjects(root, envelope, plan)
}

// Project writes Cline's native projection and records the objects it owns.
func (*Adapter) Project(ctx context.Context, in clients.ProjectionInput) ([]domain.NativeObjectOwnership, error) {
	if clineProjectSelected(in.Plan) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if err := validateClineProjection(in.StagingPath, in.Plan, false); err != nil {
			return nil, err
		}
		if err := shared.DeliverManagedStdio(in.DeliverLauncher(), in.StagingPath, in.Envelope, in.Plan); err != nil {
			return nil, err
		}
	}
	if err := ProjectClineNative(in.StagingPath, in.Envelope, in.Plan, in.PluginDataPath); err != nil {
		return nil, err
	}
	if clineProjectSelected(in.Plan) {
		return clineProjectObjects(in.StagingPath, in.Envelope, in.Plan)
	}
	return BuildClineNativeObjects(in.StagingPath, in.Envelope, in.Plan)
}
