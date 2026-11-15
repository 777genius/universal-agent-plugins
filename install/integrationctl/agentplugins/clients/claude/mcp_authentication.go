package claude

import (
	"slices"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// A connected local MCP transport proves local installation setup, separately
// from any credentials its future tools or a model session might require.
// The planner's warning is emitted only when catalog evidence is absent.
func qualifyClaudeNativeAuthentication(plan domain.DeliveryPlan, outcome domain.ActivationOutcome, allLocal bool) domain.ActivationOutcome {
	if !allLocal || outcome.Activation != domain.ActivationActive || outcome.Policy != domain.PolicyAllowed || (outcome.Authentication != "" && outcome.Authentication != domain.AuthenticationNotChecked) || !slices.Contains(plan.Warnings, "authentication_not_catalog_verified") {
		return outcome
	}
	for _, component := range plan.Components {
		if component.Support == domain.SupportUnsupported {
			continue
		}
		if component.Support != domain.SupportNative && component.Support != domain.SupportProjected {
			return outcome
		}
		if component.Kind != domain.ComponentSkill && component.Kind != domain.ComponentMCPServer {
			return outcome
		}
	}
	outcome.Authentication = domain.AuthenticationNotRequired
	return outcome
}
