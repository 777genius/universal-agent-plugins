package codex

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

var _ clients.PlanRefiner = (*Adapter)(nil)

func (*Adapter) RefinePlan(_ context.Context, _ clients.PlanInput, plan *domain.DeliveryPlan) error {
	if plan.Status != domain.PlanUnsupported && shared.HasListingCLI(plan.NativeRegistryExecutable) {
		plan.Status = domain.PlanReady
		plan.Activation = domain.ActivationActive
		plan.UserActions = shared.AppendUnique(plan.UserActions, "start a new Codex session to load the installed plugin")
		return nil
	}
	plan.UserActions = shared.AppendUnique(plan.UserActions, "finish installation in Codex Plugins, then start a new session")
	return nil
}
