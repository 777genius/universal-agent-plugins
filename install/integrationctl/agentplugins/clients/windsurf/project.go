package windsurf

import (
	"context"
	"fmt"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

var _ clients.Projector = (*Adapter)(nil)
var _ clients.ActiveNativeProjector = (*Adapter)(nil)

func (*Adapter) ProjectActiveNative(ctx context.Context, root string, envelope domain.PackageEnvelope, plan domain.DeliveryPlan, dataPath string) ([]domain.NativeObjectOwnership, error) {
	if plan.Scope == domain.ScopeProject || plan.TargetContext.Scope == domain.ScopeProject {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if plan.ClientID != domain.ClientWindsurf || root != plan.ActivePath {
			return nil, fmt.Errorf("devin active package identity differs")
		}
		if err := plan.SelectedDelivery.ValidatePlan(plan, envelope.TreeDigest); err != nil {
			return nil, err
		}
		if err := validateDevinProjectionRoot(root, plan); err != nil {
			return nil, err
		}
		if err := validateDevinProjectStdio(root, envelope, plan, dataPath); err != nil {
			return nil, err
		}
		return devinProjectObjects(root, envelope, plan)
	}
	return BuildWindsurfNativeObjects(root, plan)
}

// Project copies the managed stdio launcher when needed, writes Windsurf MCP,
// and records the objects it owns.
func (*Adapter) Project(ctx context.Context, in clients.ProjectionInput) ([]domain.NativeObjectOwnership, error) {
	if in.Plan.Scope == domain.ScopeProject || in.Plan.TargetContext.Scope == domain.ScopeProject {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if err := validateDevinProjectionRoot(in.StagingPath, in.Plan); err != nil {
			return nil, err
		}
		if err := in.Plan.SelectedDelivery.ValidatePlan(in.Plan, in.Envelope.TreeDigest); err != nil {
			return nil, err
		}
	}
	if err := shared.DeliverManagedStdio(in.DeliverLauncher(), in.StagingPath, in.Envelope, in.Plan); err != nil {
		return nil, err
	}
	if err := ProjectWindsurfMCP(in.StagingPath, in.Envelope, in.Plan, in.PluginDataPath); err != nil {
		return nil, err
	}
	if in.Plan.Scope == domain.ScopeProject {
		return devinProjectObjects(in.StagingPath, in.Envelope, in.Plan)
	}
	return BuildWindsurfNativeObjects(in.StagingPath, in.Plan)
}
