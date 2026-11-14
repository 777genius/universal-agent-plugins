package opencode

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

var _ clients.Projector = (*Adapter)(nil)
var _ clients.ActiveNativeProjector = (*Adapter)(nil)

func (*Adapter) ProjectActiveNative(ctx context.Context, root string, envelope domain.PackageEnvelope, plan domain.DeliveryPlan, dataPath string) ([]domain.NativeObjectOwnership, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if plan.Scope == domain.ScopeProject || plan.TargetContext.Scope == domain.ScopeProject {
		if err := validateOpenCodeProjectActiveProjection(root, envelope, plan, dataPath); err != nil {
			return nil, err
		}
	}
	return BuildOpenCodeNativeObjects(root, envelope, plan)
}

// Project writes OpenCode's native projection and records the objects it owns.
func (*Adapter) Project(_ context.Context, in clients.ProjectionInput) ([]domain.NativeObjectOwnership, error) {
	if in.Plan.OpenCodeHost != nil {
		skills, transports, err := clients.PlannedOpenCodeNativeRequirements(in.Envelope, in.Plan)
		if err != nil {
			return nil, err
		}
		if err := in.Plan.OpenCodeHost.ValidateNative(skills, transports); err != nil {
			return nil, err
		}
	}
	if err := ProjectOpenCodeNative(in.StagingPath, in.Envelope, in.Plan, in.PluginDataPath); err != nil {
		return nil, err
	}
	return BuildOpenCodeNativeObjects(in.StagingPath, in.Envelope, in.Plan)
}
