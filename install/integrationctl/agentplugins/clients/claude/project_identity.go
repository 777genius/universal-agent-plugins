package claude

import (
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

func claudeRegistryListCommand(plan domain.DeliveryPlan, client domain.DetectedClient, userRoot string) (legacyports.Command, error) {
	if plan.Scope == domain.ScopeProject || plan.TargetContext.Scope == domain.ScopeProject {
		root, err := claudePlanProjectRoot(client, plan)
		if err != nil {
			return legacyports.Command{}, err
		}
		return claudeProjectListCommand(plan.NativeRegistryExecutable, client, plan.TargetContext, root, plan.ActivePath)
	}
	return ClaudeListCommand(plan.NativeRegistryExecutable, userRoot, plan.ActivePath)
}
