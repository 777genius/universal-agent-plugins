package shared

import (
	"slices"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// ObservedSkillInstallationAuthentication qualifies installation authentication
// only after a caller has freshly verified the exact enabled native installation
// and its owned Skill bytes as active, installed, and policy-allowed. A command
// rendered as a Skill is inert text at this boundary. This does not qualify
// future tool/model use or replace explicit catalog authentication decisions.
// The raw-Skill planner qualifier remains a separate, narrower policy.
func ObservedSkillInstallationAuthentication(plan domain.DeliveryPlan) domain.AuthenticationState {
	if plan.Authentication != domain.AuthenticationNotChecked || plan.Status == domain.PlanUnsupported ||
		!slices.Contains(plan.Warnings, "authentication_not_catalog_verified") ||
		slices.Contains(plan.Warnings, "authentication_requirement_unknown") || len(plan.Components) == 0 {
		return plan.Authentication
	}
	for _, component := range plan.Components {
		if component.Kind != domain.ComponentSkill || component.Name == "" ||
			(component.Support != domain.SupportNative && component.Support != domain.SupportProjected) {
			return plan.Authentication
		}
		switch component.Source {
		case "":
			if component.SourceName != "" {
				return plan.Authentication
			}
		case domain.ComponentSourceCommand:
			if component.SourceName == "" {
				return plan.Authentication
			}
		default:
			return plan.Authentication
		}
	}
	return domain.AuthenticationNotRequired
}
