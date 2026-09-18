package copilot

import (
	"context"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

var _ clients.PlanRefiner = (*Adapter)(nil)

// RefinePlan promotes the plan when the Copilot CLI that performs the install
// is present. The CLI is this client's own executable; VS Code reaches the same
// backend through its sibling.
func (*Adapter) RefinePlan(_ context.Context, in clients.PlanInput, plan *domain.DeliveryPlan) error {
	executable := in.Client.ExecutablePath
	shared.PromoteBackendReady(plan, executable)
	action := "GitHub Copilot CLI is required for automatic activation"
	if strings.TrimSpace(executable) != "" {
		action = "agentplugins will install and verify the plugin through GitHub Copilot CLI automatically"
	}
	plan.UserActions = shared.AppendUnique(plan.UserActions, action)
	return nil
}
