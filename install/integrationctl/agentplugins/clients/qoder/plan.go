package qoder

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

var (
	_ clients.PlanPrecondition = (*Adapter)(nil)
	_ clients.PlanRefiner      = (*Adapter)(nil)
)

// CheckPlanPrecondition rejects names that cannot form the unambiguous
// "name@local" id Qoder registers plugins by. An '@' in the name would make
// every registration, verification, and uninstall addressable to a different
// plugin, so such packages never reach target resolution.
func (*Adapter) CheckPlanPrecondition(_ clients.PlanInput, plan *domain.DeliveryPlan) error {
	if qualifiedPluginName(plan.DeclaredName) {
		return nil
	}
	plan.Status = domain.PlanUnsupported
	plan.Activation = domain.ActivationFailed
	plan.Warnings = shared.AppendUnique(plan.Warnings, "qoder_plugin_name_requires_unqualified_identity")
	plan.UserActions = shared.AppendUnique(plan.UserActions, "rename the plugin without '@' or leading/trailing separators: Qoder registers plugins as name@local and ambiguous names cannot be verified or removed safely")
	return nil
}

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
