package usecase

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
)

// This observation follows exact package and native projection validation.
// Missing user configuration requires normal confirmed activation; it never
// creates a native ownership object or relaxes an explicit verifier refusal.
func (service Service) nativeDeliveryReadiness(ctx context.Context, input AddInput, plan domain.DeliveryPlan, delivery domain.StagedDelivery) (domain.StagedDelivery, bool, bool, error) {
	readiness, ok := service.Activator.(ports.ActivationReadinessInspector)
	if !ok {
		return delivery, true, false, nil
	}
	required, err := readiness.RequiresActivation(ctx, clonePreparedVerifierClient(input.Client), clonePreparedVerifierPlan(plan))
	return delivery, !required && err == nil, false, err
}
