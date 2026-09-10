package agentpluginscli

import (
	"context"
	"errors"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
)

// Only the current operation's effective Kiro intent may suppress execution.
// Ordinary resolution retains the detector's normal version discovery behavior.
func (app App) detectForLifecycle(ctx context.Context, probeVersion bool, intents map[domain.ClientID]domain.InstallIntent) ([]domain.DetectedClient, error) {
	if !probeVersion || intents[domain.ClientKiro] != domain.InstallIntentPrepare {
		return detectClientsForLifecycleResolution(ctx, app.Detector, probeVersion)
	}
	clients, err := app.Detector.Detect(ctx)
	if err != nil {
		return nil, err
	}
	var targets []domain.ClientID
	for _, client := range clients {
		if client.ClientID != domain.ClientKiro && client.Status == domain.DetectionDetected {
			targets = append(targets, client.ClientID)
		}
	}
	if targeted, ok := app.Detector.(ports.TargetedVersionProbingClientDetector); ok && len(targets) > 0 {
		probed, err := targeted.DetectTargetsWithVersionProbe(ctx, targets)
		if err != nil {
			return nil, err
		}
		for i, client := range clients {
			if client.ClientID == domain.ClientKiro {
				continue
			}
			for _, updated := range probed {
				if updated.ClientID == client.ClientID {
					clients[i] = updated
				}
			}
		}
	}
	return clients, nil
}

// Match the same client and scope as lifecycle planning, including preferences
// left after removal. Do not infer intent from another installation or scope.
func lifecycleInstallIntents(installation domain.Installation, scope string, explicit map[domain.ClientID]domain.InstallIntent) map[domain.ClientID]domain.InstallIntent {
	intents := make(map[domain.ClientID]domain.InstallIntent)
	for client, intent := range explicit {
		intents[client] = intent
	}
	for _, preference := range installation.InstallPreferences {
		if string(preference.Scope) == scope && intents[preference.ClientID] == "" {
			intents[preference.ClientID] = preference.InstallIntent
		}
	}
	for _, binding := range installation.Clients {
		client := domain.ClientID(binding.ClientID)
		if binding.Scope == scope && intents[client] == "" {
			intents[client] = binding.InstallIntent
		}
	}
	return intents
}

// Resolve Directory aliases to the retained product before any executable
// detection, just as acquisition does before choosing a release.
func (app App) addLifecycleIntents(ctx context.Context, source, scope string, explicit map[domain.ClientID]domain.InstallIntent) (map[domain.ClientID]domain.InstallIntent, error) {
	if app.StateStore == nil {
		return explicit, nil
	}
	state, err := app.StateStore.Load()
	if err != nil {
		return nil, err
	}
	installation, _ := locallyMatchedInstallation(state, source)
	if isDirectorySelector(source) && app.DirectoryClient != nil && len(state.Installations) > 0 {
		bundle, err := app.DirectoryClient.Load(ctx, installedDirectoryFloor(state))
		if err != nil {
			return nil, err
		}
		productID, err := directorySelectorProductID(bundle.Snapshot, source)
		if err != nil && !errors.Is(err, domain.ErrDirectoryNotFound) {
			return nil, err
		}
		installation, _, err = retainedDirectoryInstallation(state, source, productID)
		if err != nil {
			return nil, err
		}
	}
	return lifecycleInstallIntents(installation, scope, explicit), nil
}

func preflightAddTargets(ctx context.Context, app App, opts *options, source string, targets []domain.ClientID, clients []domain.DetectedClient) ([]domain.DetectedClient, map[domain.ClientID]domain.DetectedClient, error) {
	intents, err := app.addLifecycleIntents(ctx, source, opts.scope, opts.installIntents)
	if err != nil {
		return nil, nil, err
	}
	return preflightSelectedTargets(ctx, app, targets, clients, !opts.dryRun && isDirectorySelector(source), intents)
}

func automaticInstallTargets(targets []domain.ClientID, intents map[domain.ClientID]domain.InstallIntent) []domain.ClientID {
	var automatic []domain.ClientID
	for _, target := range targets {
		if intents[target] == domain.InstallIntentAutomatic {
			automatic = append(automatic, target)
		}
	}
	return automatic
}
