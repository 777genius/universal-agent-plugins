package planner

import (
	"fmt"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/kiro"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// KiroPrepareAction is re-exported from the client adapter that owns it, so the
// CLI keeps one name to render.
const KiroPrepareAction = kiro.PrepareAction

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
