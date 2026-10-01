package opencode

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

var _ clients.PlanRefiner = (*Adapter)(nil)

// Prior MCP receipts also require the config codec when the desired set is
// empty. Directory skill removal does not use that codec.
func (*Adapter) OwnedOpenCodeNativeRequirements(objects []domain.NativeObjectOwnership) (skills, config bool) {
	for _, object := range objects {
		switch object.Kind {
		case OpenCodeMCPObjectKind, OpenCodeV2MCPObjectKind:
			config = true
		case openCodeSkillKind:
			skills = true
		}
	}
	return
}

func (*Adapter) RefinePlan(_ context.Context, in clients.PlanInput, plan *domain.DeliveryPlan) error {
	if in.Client.OpenCodeHost != nil {
		skills, transports, err := clients.PlannedOpenCodeNativeRequirements(in.Envelope, *plan)
		if err != nil {
			return err
		}
		if err := in.Client.OpenCodeHost.ValidateNative(skills, transports); err != nil {
			return err
		}
		plan.OpenCodeHost = in.Client.OpenCodeHost
	}
	shared.PromoteNativeReady(plan, in.Client.ConfigRoot, shared.OnlyNativeComponents(plan.Components))
	plan.UserActions = shared.AppendUnique(plan.UserActions, "agentplugins will install the package's skills and MCP servers; restart OpenCode when complete")
	return nil
}
