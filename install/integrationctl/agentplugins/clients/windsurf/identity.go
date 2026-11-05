package windsurf

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

var _ clients.RegistryInspector = (*Adapter)(nil)

func (*Adapter) UsesNativeRegistryExecutable() bool { return false }

func (*Adapter) InspectNativeRegistry(ctx context.Context, env clients.Env, client domain.DetectedClient, plan domain.DeliveryPlan, managed *domain.ClientBinding) (clients.RegistryFinding, error) {
	if err := ctx.Err(); err != nil {
		return clients.RegistryIndeterminate, err
	}
	if plan.Scope == domain.ScopeProject || plan.TargetContext.Scope == domain.ScopeProject {
		root, err := devinProjectPlanRoot(client, plan)
		if err != nil {
			return clients.RegistryIndeterminate, err
		}
		if managed != nil {
			if err := validateDevinProjectBinding(root, plan.PhysicalArtifactID, *managed); err != nil {
				return clients.RegistryIndeterminate, err
			}
			selected := *managed
			selected.NativeObjects = devinNativeObjects(managed.NativeObjects)
			managed = &selected
		}
		return devinComponents(root).Inspect(plan.Components, managed, env.NativeConfig)
	}
	if primaryDevinExecutable(client.ExecutablePath) || isDevinGlobalPlan(plan) ||
		(managed != nil && hasDevinGlobalObjects(managed.NativeObjects)) {
		root, err := devinGlobalPlanRoot(client, plan)
		if err != nil {
			return clients.RegistryIndeterminate, err
		}
		if managed != nil {
			if err := validateDevinGlobalBinding(root, plan.PhysicalArtifactID, plan.ActivePath, *managed); err != nil {
				return clients.RegistryIndeterminate, err
			}
			copy := *managed
			copy.NativeObjects = devinNativeObjects(managed.NativeObjects)
			managed = &copy
		}
		return devinGlobalComponents(root, plan.ActivePath).Inspect(plan.Components, managed, env.NativeConfig)
	}
	return InspectWindsurfRegistry(plan, managed, env.NativeConfig)
}
