package gemini

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
	root, err := geminiInspectionRoot(client, plan)
	if err != nil {
		return clients.RegistryIndeterminate, err
	}
	plan.NativeRegistryRoot = root
	return InspectGeminiRegistry(plan, managed, env.NativeConfig)
}

func geminiInspectionRoot(client domain.DetectedClient, plan domain.DeliveryPlan) (string, error) {
	if plan.Scope == domain.ScopeProject {
		return geminiPlanRoot(client, plan)
	}
	root := client.ConfigRoot
	if root == "" {
		root = plan.NativeRegistryRoot
	}
	if root == "" {
		return "", nil
	}
	return root, validateProfile(root, plan.NativeRegistryRoot)
}
