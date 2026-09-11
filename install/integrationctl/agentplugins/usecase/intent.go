package usecase

import (
	"context"
	"fmt"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/planner"
)

// planInstall resolves intent before any activation preflight, including grouped
// compatibility checks. An omitted flag retains even an absent binding's intent.
func (service Service) planInstall(ctx context.Context, input *AddInput, physicalID string, installation *domain.Installation) (domain.DeliveryPlan, error) {
	if err := input.InstallIntent.Validate(input.Client.ClientID); err != nil {
		return domain.DeliveryPlan{}, err
	}
	if installation != nil {
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
				return domain.DeliveryPlan{}, err
			}
			if input.InstallIntent != "" && input.InstallIntent != binding.InstallIntent && binding.Materialization != domain.MaterializationAbsent {
				return domain.DeliveryPlan{}, fmt.Errorf("install intent differs from the persisted binding; remove the binding before changing mode")
			}
			if input.InstallIntent == "" {
				input.InstallIntent = binding.InstallIntent
			}
		}
	}
	if input.Client.ClientID == domain.ClientChatGPT && input.InstallIntent == domain.InstallIntentPrepare && input.Envelope.LocalChatGPTMapping == nil {
		return domain.DeliveryPlan{}, fmt.Errorf("ChatGPT preparation requires a personal Context7 registration receipt")
	}
	if input.Envelope.LocalChatGPTMapping != nil {
		if input.Client.ClientID != domain.ClientChatGPT || input.Scope != domain.ScopeUser || input.InstallIntent != domain.InstallIntentPrepare || input.OriginMode != domain.OriginModeDirectory || input.DirectoryResolution == nil || input.DirectoryResolution.ProductID != "context7" || input.DirectoryResolution.DistributionID != "upstash/context7" {
			return domain.DeliveryPlan{}, fmt.Errorf("personal ChatGPT mapping requires explicit canonical Directory Context7 user preparation")
		}
		if installation != nil && installation.LocalChatGPTMapping != nil && *installation.LocalChatGPTMapping != *input.Envelope.LocalChatGPTMapping && !installation.LocalChatGPTMapping.IsLegacyContext7Registration() {
			return domain.DeliveryPlan{}, fmt.Errorf("personal ChatGPT registration differs from retained receipt")
		}
	}
	plan, err := service.Planner.Plan(ctx, input.Envelope, input.Client, input.Scope, physicalID)
	if err == nil {
		err = planner.ApplyInstallIntent(&plan, input.InstallIntent)
	}
	return plan, err
}
