package agentpluginscli

import (
	"context"
	"errors"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/prompt"
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
	compatible, skipped, loaded, err := compatibleTargetsDeferringHostQualification(ctx, preview, source, detected)
	if loaded != nil && loaded.cleanup != nil {
		defer func() { _ = loaded.cleanup() }()
	}
	if err != nil {
		return nil, nil, err
	}
	compatible, skipped = offerSavedHostedPreparation(app, detected, compatible, skipped)
	return compatible, skipped, nil
}

// offerSavedHostedPreparation keeps a client whose automatic host check failed
// in the choices when the saved installation itself falls back to file-only
// preparation. That fallback exists only for a fresh User-scope install on a
// client that allows hosted preparation (installer.previewHostedPackage), so no
// other scope or client is offered. Selection still runs the same preflight and
// confirmation before anything is written.
func offerSavedHostedPreparation(app App, detected, compatible []domain.DetectedClient, skipped []targetSkip) ([]domain.DetectedClient, []targetSkip) {
	if app.scope() != domain.ScopeUser {
		return compatible, skipped
	}
	byID := detectedClientMap(detected)
	kept := skipped[:0]
	for _, skip := range skipped {
		client, ok := byID[skip.Client]
		if !ok || skip.Kind != prompt.SkipManual || !allowsHostedPrepare(skip.Client) {
			kept = append(kept, skip)
			continue
		}
		client.DisplayName += preparedOnlySuffix
		compatible = append(compatible, client)
	}
	return compatible, kept
}

// compatibleTargetsDeferringHostQualification offers detected clients whose
// planning failed only because read-only discovery does not qualify a native
// host. The selected operation qualifies that host before review and effects,
// so presenting the choice runs no native executable. Without the real
// preparation port, the shared classifier keeps its ordinary skip.
func compatibleTargetsDeferringHostQualification(ctx context.Context, app App, source string, detected []domain.DetectedClient, intents ...map[domain.ClientID]domain.InstallIntent) ([]domain.DetectedClient, []targetSkip, *loadedPackage, error) {
	deferred := map[domain.ClientID]bool{}
	if app.Planner != nil && app.Lifecycle.ClientPreparation != nil {
		app.Planner = hostDeferredPlanner{DeliveryPlanner: app.Planner, deferred: deferred}
	}
	compatible, skipped, loaded, err := app.compatibleDetectedTargets(ctx, source, detected, intents...)
	if err != nil {
		return nil, nil, loaded, err
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
	return compatible, kept, loaded, nil
}

// Missing prepared host authority is deferred only through its typed contract,
// and only operations with the real preparation port use this planner.
// Qualification still occurs after selection; arbitrary planning failures and
// already-qualified host refusals remain excluded by the shared classifier.
// Offline unchanged actions can also use this choice without needing a host;
// fresh offline planning still goes through its existing authoritative gate.
type hostDeferredPlanner struct {
	ports.DeliveryPlanner
	deferred map[domain.ClientID]bool
}

func (planner hostDeferredPlanner) Plan(ctx context.Context, request domain.PlanRequest) (domain.DeliveryPlan, error) {
	plan, err := planner.DeliveryPlanner.Plan(ctx, request)
	// Only a detected executable can be qualified after selection.
	if request.Client.OpenCodeHost == nil && request.Client.ExecutablePath != "" && errors.Is(err, clients.ErrOpenCodeAdapterUnavailable) {
		planner.deferred[request.Client.ClientID] = true
	}
	return plan, err
}
