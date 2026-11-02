package cursor

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
	if plan.Scope == domain.ScopeProject {
		root, err := cursorPlanRoot(client, plan)
		if err != nil {
			return clients.RegistryIndeterminate, err
		}
		if managed != nil {
			selected := *managed
			selected.NativeObjects = cursorNativeObjects(managed.NativeObjects)
			managed = &selected
		}
		return cursorComponents(root).Inspect(plan.Components, managed, env.NativeConfig)
	}
	// This manual preparation contract has no managed executable registration.
	// Clear leaves the existing prepared-directory collision walk in charge;
	// UsesNativeRegistryExecutable=false means no native discovery was attempted.
	// It does not prove discovery of plugins/local or absence of other sources.
	return clients.RegistryClear, nil
}
