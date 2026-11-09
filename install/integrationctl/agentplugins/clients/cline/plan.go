package cline

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

var _ clients.PlanRefiner = (*Adapter)(nil)

func (*Adapter) RefinePlan(_ context.Context, in clients.PlanInput, plan *domain.DeliveryPlan) error {
	if clineProjectSelected(*plan) || in.TargetContext.ProjectRoot != "" {
		return refineClineProjectPlan(in, plan)
	}
	shared.PromoteNativeReady(plan, in.Client.ConfigRoot, shared.OnlyNativeComponents(plan.Components))
	plan.UserActions = shared.AppendUnique(plan.UserActions, "agentplugins will install and verify Cline skills and MCP entries in the selected native profile automatically; Cline CLI reads its selected MCP settings on its next process. For a Cline IDE extension, open MCP Servers > Configure > Configure MCP Servers and confirm the settings file matches the selected profile before expecting those entries")
	return nil
}
