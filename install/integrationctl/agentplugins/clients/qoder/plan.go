package qoder

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

var _ clients.PlanRefiner = (*Adapter)(nil)

// RefinePlan promotes on the Qoder CLI: the package lands in the managed root,
// so readiness is the CLI's ability to register it, not a config root to write.
func (*Adapter) RefinePlan(_ context.Context, in clients.PlanInput, plan *domain.DeliveryPlan) error {
	shared.PromoteNativeReady(plan, in.Client.ExecutablePath, shared.OnlyNativeComponents(plan.Components))
	if plan.Status == domain.PlanUnsupported {
		return nil
	}
	if in.Client.ExecutablePath == "" {
		plan.UserActions = shared.AppendUnique(plan.UserActions, "install Qoder CLI to verify and manage this native plugin, then rerun agentplugins update")
	} else {
		plan.UserActions = shared.AppendUnique(plan.UserActions, "agentplugins will register and verify the plugin through Qoder CLI; run /plugins reload or restart Qoder to use it")
	}
	return nil
}
