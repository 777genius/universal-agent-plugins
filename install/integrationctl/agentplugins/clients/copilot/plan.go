package copilot

import (
	"context"
	"fmt"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

var _ clients.PlanRefiner = (*Adapter)(nil)

// RefinePlan keeps repository configuration separate from user CLI activation.
func (*Adapter) RefinePlan(ctx context.Context, in clients.PlanInput, plan *domain.DeliveryPlan) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if in.TargetContext.Scope == domain.ScopeProject || plan.Scope == domain.ScopeProject {
		if _, err := copilotPlanRoot(in.Client, *plan); err != nil {
			return err
		}
		if _, err := copilotTrustPath(in.Client, *plan); err != nil {
			return err
		}
		shared.PromoteNativeReady(plan, plan.NativeRegistryRoot, shared.OnlyNativeComponents(plan.Components))
		plan.Activation = domain.ActivationPrepared
		plan.UserActions = shared.AppendUnique(plan.UserActions, "agentplugins will configure repository skills and MCP entries and remember the selected project folder in Copilot for future sessions; this trusts repository configuration without granting tool, path or URL permissions; native discovery and session policy remain unobserved")
		plan.LocalActions = shared.AppendUnique(plan.LocalActions, fmt.Sprintf("remember only project folder %s in selected Copilot profile %s; retain this folder trust on plugin removal", plan.TargetContext.ProjectRoot, plan.TargetContext.UserConfigRoot))
		return nil
	}
	executable := in.Client.ExecutablePath
	shared.PromoteBackendReady(plan, executable)
	action := "GitHub Copilot CLI is required for automatic activation"
	if strings.TrimSpace(executable) != "" {
		action = "agentplugins will install and verify the plugin through GitHub Copilot CLI automatically"
	}
	plan.UserActions = shared.AppendUnique(plan.UserActions, action)
	return nil
}
