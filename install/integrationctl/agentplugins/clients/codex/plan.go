package codex

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

var _ clients.PlanRefiner = (*Adapter)(nil)

func (*Adapter) RefinePlan(_ context.Context, _ clients.PlanInput, plan *domain.DeliveryPlan) error {
	plan.UserActions = shared.AppendUnique(plan.UserActions, "finish installation in Codex Plugins, then start a new session")
	return nil
}
