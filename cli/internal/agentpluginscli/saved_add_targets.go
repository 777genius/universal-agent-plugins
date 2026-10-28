package agentpluginscli

import (
	"context"
	"errors"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
)

// Automatic selection uses disposable package acquisition and shared host
// admission. The authoritative saved operation resolves and revalidates its pin
// after selection; no native executable runs while presenting choices.
func compatibleSavedAddTargets(ctx context.Context, app App, source string, detected []domain.DetectedClient) ([]domain.DetectedClient, []targetSkip, error) {
	app.dependencySnapshots = nil
	preview, cleanup, err := dependencyPreviewApp(app, &options{dryRun: true})
	if err != nil {
		return nil, nil, err
	}
	defer cleanup()
	deferred := map[domain.ClientID]bool{}
	if preview.Planner != nil && preview.Lifecycle.ClientPreparation != nil {
		preview.Planner = savedAddDeferredPlanner{DeliveryPlanner: preview.Planner, deferred: deferred}
	}
	compatible, skipped, loaded, err := preview.compatibleDetectedTargets(ctx, source, detected)
	if loaded != nil && loaded.cleanup != nil {
		defer func() { _ = loaded.cleanup() }()
	}
	if err != nil {
		return nil, nil, err
	}
	for _, client := range detected {
		if deferred[client.ClientID] {
			compatible = append(compatible, client)
		}
	}
	kept := skipped[:0]
	for _, skip := range skipped {
		if !deferred[skip.Client] {
			kept = append(kept, skip)
		}
	}
	return compatible, kept, nil
}

// Missing prepared host authority is deferred only through its typed contract,
// and only operations with the real preparation port use this planner.
// Qualification still occurs after selection; arbitrary planning failures and
// already-qualified host refusals remain excluded by the shared classifier.
// Offline unchanged actions can also use this choice without needing a host;
// fresh offline planning still goes through its existing authoritative gate.
type savedAddDeferredPlanner struct {
	ports.DeliveryPlanner
	deferred map[domain.ClientID]bool
}

func (planner savedAddDeferredPlanner) Plan(ctx context.Context, request domain.PlanRequest) (domain.DeliveryPlan, error) {
	plan, err := planner.DeliveryPlanner.Plan(ctx, request)
	if request.Client.OpenCodeHost == nil && errors.Is(err, clients.ErrOpenCodeAdapterUnavailable) {
		planner.deferred[request.Client.ClientID] = true
	}
	return plan, err
}
