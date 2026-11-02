package copilot

import (
	"context"
	"fmt"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

var _ clients.Projector = (*Adapter)(nil)
var _ clients.ActiveNativeProjector = (*Adapter)(nil)

// Project writes repository component receipts or the existing user marketplace.
func (*Adapter) Project(ctx context.Context, in clients.ProjectionInput) ([]domain.NativeObjectOwnership, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if in.Plan.Scope == domain.ScopeProject || in.Plan.TargetContext.Scope == domain.ScopeProject {
		if _, err := copilotPlanRoot(domain.DetectedClient{ConfigRoot: in.Plan.TargetContext.UserConfigRoot}, in.Plan); err != nil {
			return nil, err
		}
		if err := shared.ProjectMCPServers(shared.MCPProjection{Root: in.StagingPath, Envelope: in.Envelope,
			Names: shared.SupportedMCPNames(in.Plan), Dialect: shared.MCPDialectCursor, PluginRoot: in.Plan.ActivePath, DataPath: in.PluginDataPath}); err != nil {
			return nil, err
		}
		if err := projectCopilotStdio(in); err != nil {
			return nil, err
		}
		return copilotProjectObjects(in.StagingPath, in.Envelope, in.Plan)
	}
	if err := shared.ProjectCopilotMarketplace(in.StagingPath, in.Envelope, in.Plan); err != nil {
		return nil, err
	}
	return nil, nil
}

func (*Adapter) ProjectActiveNative(ctx context.Context, root string, envelope domain.PackageEnvelope, plan domain.DeliveryPlan, dataPath string) ([]domain.NativeObjectOwnership, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if plan.ClientID != domain.ClientCopilot || root == "" || root != plan.ActivePath {
		return nil, fmt.Errorf("copilot active projection client or root differs")
	}
	if err := plan.SelectedDelivery.ValidatePlan(plan, envelope.TreeDigest); err != nil {
		return nil, err
	}
	if plan.Scope == domain.ScopeProject || plan.TargetContext.Scope == domain.ScopeProject {
		if _, err := copilotPlanRoot(domain.DetectedClient{ConfigRoot: plan.TargetContext.UserConfigRoot}, plan); err != nil {
			return nil, err
		}
		if err := validateCopilotProjectStdio(root, envelope, plan, dataPath); err != nil {
			return nil, err
		}
		return copilotProjectObjects(root, envelope, plan)
	}
	return nil, nil
}
