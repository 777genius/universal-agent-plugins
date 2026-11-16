package gemini

import (
	"context"
	"fmt"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

var (
	_ clients.PlanRefiner      = (*Adapter)(nil)
	_ clients.PlanPrecondition = (*Adapter)(nil)
)

func (*Adapter) CheckPlanPrecondition(in clients.PlanInput, plan *domain.DeliveryPlan) error {
	_, err := geminiNativeRoot(in.Client, in.TargetContext, plan.NativeRegistryRoot)
	return err
}

func (*Adapter) RefinePlan(_ context.Context, in clients.PlanInput, plan *domain.DeliveryPlan) error {
	if in.TargetContext.Scope == domain.ScopeProject {
		shared.QualifyRawSkillInstallationAuthentication(in.Envelope, plan)
	}
	root := in.Client.ConfigRoot
	if in.TargetContext.Scope == domain.ScopeProject {
		root = plan.NativeRegistryRoot
	}
	shared.PromoteNativeReady(plan, root, shared.OnlyNativeComponents(plan.Components))
	if in.TargetContext.Scope == domain.ScopeProject {
		plan.Activation = domain.ActivationPrepared
		if in.Intent == domain.InstallIntentAutomatic {
			plan.Warnings = shared.AppendUnique(plan.Warnings, fmt.Sprintf("Installation may trust the selected project %q in Gemini CLI. Existing distrust is preserved; this preference remains after removal.", in.TargetContext.ProjectRoot))
			for _, component := range plan.Components {
				if component.Kind == domain.ComponentMCPServer && component.Support == domain.SupportNative {
					plan.Warnings = shared.AppendUnique(plan.Warnings, geminiMCPReviewWarning)
				}
			}
		}
		plan.UserActions = shared.AppendUnique(plan.UserActions, "project skills and MCP settings can be configured automatically; native discovery, workspace trust and system policy require observation in Gemini CLI")
		return nil
	}
	plan.UserActions = shared.AppendUnique(plan.UserActions, "agentplugins will install and verify Gemini CLI skills and MCP servers automatically; reload them in a running session or restart Gemini CLI")
	return nil
}
