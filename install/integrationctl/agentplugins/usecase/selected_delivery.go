package usecase

import (
	"fmt"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// validateSelectedPlan runs before activation/component/identity preflight, so
// an old CLI receipt cannot authorize a command or profile effect for Local.
func validateSelectedPlan(plan *domain.DeliveryPlan, envelope domain.PackageEnvelope, installation *domain.Installation, refresh bool) error {
	if err := plan.SelectedDelivery.ValidatePlan(*plan, envelope.TreeDigest); err != nil {
		return err
	}
	if installation == nil {
		return nil
	}
	for _, binding := range installation.Clients {
		if binding.Scope != string(plan.Scope) || !sameNativeBackend(domain.ClientID(binding.ClientID), plan.ClientID) {
			continue
		}
		if err := binding.SelectedDelivery.Validate(); err != nil {
			return err
		}
		if binding.Materialization == domain.MaterializationAbsent {
			continue
		}
		if err := validatePlannedSelection(binding.SelectedDelivery, plan.SelectedDelivery, refresh); err != nil {
			return err
		}
		if facts, ok := binding.SelectedDelivery.LocalFacts(); ok && facts.CanonicalDigest == envelope.TreeDigest {
			var err error
			plan.SelectedDelivery, err = plan.SelectedDelivery.WithProjectionDigest(facts.ProjectionDigest)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func validateDeliverySelection(recorded, selected domain.SelectedDelivery) error {
	if err := recorded.Validate(); err != nil {
		return err
	}
	if err := selected.Validate(); err != nil {
		return err
	}
	if recorded.Mode() != selected.Mode() {
		return fmt.Errorf("selected delivery mode differs from persisted binding; remove or explicitly rebind first")
	}
	if !recorded.SameSelection(selected) {
		return fmt.Errorf("selected delivery profile, shell, components or owned authority differs from persisted binding; reviewed refresh is required")
	}
	return nil
}

func sealStagedSelection(plan *domain.DeliveryPlan, delivery domain.StagedDelivery) error {
	selected, err := plan.SelectedDelivery.WithProjectionDigest(delivery.ArtifactDigest)
	if err != nil {
		return err
	}
	plan.SelectedDelivery = selected
	return nil
}

func selectedCanonicalDigest(plan domain.DeliveryPlan) string {
	facts, _ := plan.SelectedDelivery.LocalFacts()
	return facts.CanonicalDigest
}

func validatePlannedSelection(recorded, selected domain.SelectedDelivery, refresh bool) error {
	if !refresh {
		return validateDeliverySelection(recorded, selected)
	}
	if err := recorded.Validate(); err != nil {
		return err
	}
	if !recorded.SameProfile(selected) {
		return fmt.Errorf("projection refresh cannot change delivery mode or physical profile/entry authority")
	}
	return nil
}

// Unknown persisted modes cannot authorize even directory recovery on the
// confirmed usecase path. Infrastructure locking precedes this read; package,
// profile and process effects do not.
func (service Service) validateRecordedDeliveries() error {
	state, err := service.StateStore.Load()
	if err != nil {
		return err
	}
	for _, installation := range state.Installations {
		for _, binding := range installation.Clients {
			if err := binding.SelectedDelivery.Validate(); err != nil {
				return err
			}
		}
	}
	return nil
}
