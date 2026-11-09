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
	componentDescription := "Cline skills"
	hasMCP := shared.HasSupportedMCP(plan.Components)
	if hasMCP {
		componentDescription = "Cline MCP entries"
		if shared.ComponentKindPresent(plan.Components, domain.ComponentSkill) {
			componentDescription = "Cline skills and MCP entries"
		}
	}
	action := "agentplugins will install and verify " + componentDescription + " in the selected native profile automatically"
	if hasMCP {
		action += "; Cline CLI reads its selected MCP settings on its next process. For a Cline IDE extension, open MCP Servers > Configure > Configure MCP Servers and confirm the settings file matches the selected profile before expecting those entries"
	} else {
		action += "; verify Skills discovery in the selected Cline CLI or IDE session"
	}
	plan.UserActions = shared.AppendUnique(plan.UserActions, action)
	return nil
}
