package clients

import "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"

// SelectLocalDelivery is the narrow PlanRefiner seam for the future Local
// adapter. NewLocalDelivery copies constructor inputs; generic core consumes
// frozen effective facts rather than adding ClientID or delivery-mode branches.
// No Local adapter is registered by this preparatory contract.
func SelectLocalDelivery(plan *domain.DeliveryPlan, facts domain.LocalDeliveryFacts) error {
	selected, err := domain.NewLocalDelivery(facts)
	if err != nil {
		return err
	}
	if err := selected.ValidatePlan(*plan, facts.CanonicalDigest); err != nil {
		return err
	}
	plan.SelectedDelivery = selected
	return nil
}
