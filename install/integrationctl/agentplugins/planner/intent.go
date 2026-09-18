package planner

import (
	"fmt"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// KiroPrepareAction is the user-facing leftover after a prepared Kiro
// delivery. The wording is owned by clients/kiro.PrepareAction; the facade
// keeps a copy so planner production code never imports a concrete adapter.
// facade_actions_test.go locks the two strings together.
const KiroPrepareAction = "After preparation, open or restart Kiro, review the MCP servers and complete authentication in Kiro on first connection when prompted, then verify their tools in Kiro. Runtime connections have not been verified."

// ApplyInstallIntent retains all package and Directory compatibility decisions.
// It is also applied on its own, to a plan that was already built, when a
// caller changes its mind about a target, which is why it takes the registry
// rather than reading one from a planner.
func ApplyInstallIntent(registry *clients.Registry, plan *domain.DeliveryPlan, intent domain.InstallIntent) error {
	if registry == nil {
		return clients.ErrRegistryRequired
	}
	if err := intent.Validate(plan.ClientID); err != nil {
		return err
	}
	if intent == domain.InstallIntentPrepare && plan.Scope != domain.ScopeUser {
		return fmt.Errorf("preparation supports user scope only")
	}
	plan.InstallIntent = intent
	if intent != domain.InstallIntentPrepare || plan.Status == domain.PlanUnsupported {
		return nil
	}
	refiner, ok := clients.As[clients.PreparationRefiner](registry, plan.ClientID)
	if !ok {
		return fmt.Errorf("unsupported install intent %q for client %s", intent, plan.ClientID)
	}
	return refiner.RefinePreparation(plan)
}
