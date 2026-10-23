package gemini

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

var (
	_ clients.PlanRefiner      = (*Adapter)(nil)
	_ clients.PlanPrecondition = (*Adapter)(nil)
)

func (*Adapter) CheckPlanPrecondition(in clients.PlanInput, plan *domain.DeliveryPlan) error {
	_, err := geminiNativeRoot(in.Client, in.TargetContext, plan.NativeRegistryRoot)
	return err
}

func (*Adapter) RefinePlan(_ context.Context, in clients.PlanInput, plan *domain.DeliveryPlan) error {
	root := in.Client.ConfigRoot
	if in.TargetContext.Scope == domain.ScopeProject {
		root = plan.NativeRegistryRoot
	}
	shared.PromoteNativeReady(plan, root, shared.OnlyNativeComponents(plan.Components))
	plan.UserActions = shared.AppendUnique(plan.UserActions, "agentplugins will install and verify Gemini CLI skills and MCP servers automatically; reload them in a running session or restart Gemini CLI")
	return nil
}
