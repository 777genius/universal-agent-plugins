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
	if intent == domain.InstallIntentPrepare {
		if err := validatePreparationScope(registry, *plan); err != nil {
			return err
		}
	}
	if intent == domain.InstallIntentPrepare && plan.Scope != domain.ScopeUser &&
		!plan.SelectedDelivery.EffectiveTraitsForScope(plan.ClientID, plan.Scope).Allows(intent) {
		return fmt.Errorf("preparation is not qualified for scope %s", plan.Scope)
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

// Reapplying an intent to a previously built plan must not bypass explicit
// project context admission. Genuine historical user plans retain their default.
func validatePreparationScope(registry *clients.Registry, plan domain.DeliveryPlan) error {
	scope := plan.Scope
	if scope == "" {
		scope = domain.ScopeUser
	}
	if !clients.SupportsScope(registry, plan.ClientID, scope) {
		return fmt.Errorf("preparation is not qualified for scope %s", scope)
	}
	if plan.Scope != domain.ScopeProject && plan.TargetContext.Scope != domain.ScopeProject && plan.TargetContext.ProjectRoot == "" {
		return nil
	}
	if plan.Scope != domain.ScopeProject || plan.TargetContext.Scope != domain.ScopeProject {
		return fmt.Errorf("project preparation requires matching explicit project scope")
	}
	_, err := domain.NewTargetContext(plan.TargetContext.Scope, plan.TargetContext.ProjectRoot, plan.TargetContext.UserConfigRoot)
	return err
}
