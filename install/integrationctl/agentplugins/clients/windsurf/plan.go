package windsurf

import (
	"context"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

var (
	_ clients.PlanRefiner          = (*Adapter)(nil)
	_ clients.CompatibilityLimiter = (*Adapter)(nil)
)

// RefinePlan claims only the MCP part of a package: a legacy Windsurf channel
// takes MCP servers through its local configuration, while skills stay in the
// prepared package and are never reported as activated.
func (*Adapter) RefinePlan(_ context.Context, in clients.PlanInput, plan *domain.DeliveryPlan) error {
	hasMCP := shared.ComponentKindPresent(plan.Components, domain.ComponentMCPServer)
	shared.PromoteNativeReady(plan, in.Client.ConfigRoot, hasMCP)
	action := "select exactly one legacy Windsurf channel and import the prepared MCP configuration manually"
	if strings.TrimSpace(in.Client.ConfigRoot) != "" && hasMCP {
		action = "agentplugins will update the selected legacy Windsurf channel's local MCP configuration; refresh MCP servers before first use"
	}
	plan.UserActions = shared.AppendUnique(plan.UserActions, action)
	if shared.ComponentKindPresent(plan.Components, domain.ComponentSkill) {
		plan.Warnings = shared.AppendUnique(plan.Warnings, "windsurf_skills_prepared_only")
		plan.UserActions = shared.AppendUnique(plan.UserActions, "Windsurf skills remain in the prepared package and are not claimed as activated")
	}
	return nil
}

// ClientLimitations records that a channel still has to be chosen and that
// skills are prepared rather than activated.
func (*Adapter) ClientLimitations(domain.PackageEnvelope) []string {
	return []string{"windsurf_channel_selection_required", "windsurf_skills_prepared_only"}
}

// ComponentLimitations adds nothing per component: the limitations Windsurf
// carries are about the client, not about any single item.
func (*Adapter) ComponentLimitations(domain.PackageEnvelope, domain.ComponentDecision) ([]string, []string) {
	return nil, nil
}
