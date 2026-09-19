package usecase

import (
	"context"
	"fmt"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// planInstall resolves intent before any activation preflight, including grouped
// compatibility checks. An omitted flag retains even an absent binding's intent.
func (service Service) planInstall(ctx context.Context, input *AddInput, physicalID string, installation *domain.Installation) (domain.DeliveryPlan, error) {
	if err := input.InstallIntent.Validate(input.Client.ClientID); err != nil {
		return domain.DeliveryPlan{}, err
	}
	if err := resolvePersistedIntent(input, installation); err != nil {
		return domain.DeliveryPlan{}, err
	}
	if err := validatePersonalMapping(input, installation); err != nil {
		return domain.DeliveryPlan{}, err
	}
	return service.Planner.Plan(ctx, domain.PlanRequest{
		Envelope:           input.Envelope,
		Client:             input.Client,
		Scope:              input.Scope,
		PhysicalArtifactID: physicalID,
		InstallIntent:      input.InstallIntent,
		Detected:           service.Detected,
	})
}

func resolvePersistedIntent(input *AddInput, installation *domain.Installation) error {
	if installation == nil {
		return nil
	}
	if input.InstallIntent == "" {
		for _, preference := range installation.InstallPreferences {
			if preference.ClientID == input.Client.ClientID && preference.Scope == input.Scope {
				input.InstallIntent = preference.InstallIntent
			}
		}
	}
	for _, binding := range installation.Clients {
		if binding.ClientID != string(input.Client.ClientID) || binding.Scope != string(input.Scope) {
			continue
		}
		if err := binding.InstallIntent.Validate(input.Client.ClientID); err != nil {
			return err
		}
		if input.InstallIntent != "" && input.InstallIntent != binding.InstallIntent && binding.Materialization != domain.MaterializationAbsent {
			return fmt.Errorf("install intent differs from the persisted binding; remove the binding before changing mode")
		}
		if input.InstallIntent == "" {
			input.InstallIntent = binding.InstallIntent
		}
	}
	return nil
}

func validatePersonalMapping(input *AddInput, installation *domain.Installation) error {
	traits := domain.ClientTraitsFor(input.Client.ClientID)
	if traits.RequiresPersonalMappingForPrepare && input.InstallIntent == domain.InstallIntentPrepare && input.Envelope.LocalChatGPTMapping == nil {
		return fmt.Errorf("%s preparation requires a personal Context7 registration receipt", clientDisplayName(input.Client.ClientID))
	}
	if input.Envelope.LocalChatGPTMapping == nil {
		return nil
	}
	if !traits.RequiresPersonalMappingForPrepare || input.Scope != domain.ScopeUser || input.InstallIntent != domain.InstallIntentPrepare || input.OriginMode != domain.OriginModeDirectory || input.DirectoryResolution == nil || input.DirectoryResolution.ProductID != "context7" || input.DirectoryResolution.DistributionID != "upstash/context7" {
		return fmt.Errorf("personal ChatGPT mapping requires explicit canonical Directory Context7 user preparation")
	}
	if installation != nil && installation.LocalChatGPTMapping != nil && *installation.LocalChatGPTMapping != *input.Envelope.LocalChatGPTMapping && !installation.LocalChatGPTMapping.IsLegacyContext7Registration() {
		return fmt.Errorf("personal ChatGPT registration differs from retained receipt")
	}
	return nil
}
