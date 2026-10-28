package copilot

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

var _ clients.RegistryInspector = (*Adapter)(nil)
var _ clients.PreparedRegistryInspector = (*Adapter)(nil)

func (*Adapter) UsesNativeRegistryExecutable() bool { return true }

func (*Adapter) InspectNativeRegistry(ctx context.Context, env clients.Env, client domain.DetectedClient, plan domain.DeliveryPlan, managed *domain.ClientBinding) (clients.RegistryFinding, error) {
	if err := ctx.Err(); err != nil {
		return clients.RegistryIndeterminate, err
	}
	if plan.Scope == domain.ScopeProject || plan.TargetContext.Scope == domain.ScopeProject {
		if _, err := copilotPlanRoot(client, plan); err != nil {
			return clients.RegistryIndeterminate, err
		}
		return inspectCopilotProject(plan, managed, env.NativeConfig)
	}
	return shared.InspectCopilotRegistryWithListingRecognition(ctx, env, plan, managed, recognizeCopilotUserRegistry)
}

func (*Adapter) InspectPreparedRegistry(plan domain.DeliveryPlan, name string, owned bool) (clients.RegistryFinding, error) {
	if plan.Scope != domain.ScopeProject && plan.TargetContext.Scope != domain.ScopeProject {
		return shared.InspectUnqualifiedPluginRoot(plan.TargetRoot, name, plan.ActivePath, owned)
	}
	if _, err := copilotPlanRoot(domain.DetectedClient{ConfigRoot: plan.TargetContext.UserConfigRoot}, plan); err != nil {
		return clients.RegistryIndeterminate, err
	}
	var managed *domain.ClientBinding
	if owned {
		managed = &domain.ClientBinding{Scope: string(domain.ScopeProject), NativeProfileRoot: plan.NativeRegistryRoot,
			PhysicalArtifact: plan.PhysicalArtifactID, TargetLocator: plan.ActivePath, NativeObjects: plan.PreviousNativeObjects}
	}
	return inspectCopilotProject(plan, managed, nativeconfig.New())
}
