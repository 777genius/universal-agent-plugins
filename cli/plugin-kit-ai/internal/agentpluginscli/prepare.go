package agentpluginscli

import (
	"context"
	"errors"
	"strings"

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
func (app App) addLifecycleIntents(ctx context.Context, source, scope string, explicit map[domain.ClientID]domain.InstallIntent, targetSets ...[]domain.ClientID) (map[domain.ClientID]domain.InstallIntent, error) {
	if explicit == nil {
		explicit = make(map[domain.ClientID]domain.InstallIntent)
	}
	if app.StateStore == nil {
		return explicit, nil
	}
	state, err := app.StateStore.Load()
	if err != nil {
		return nil, err
	}
	installation, _ := locallyMatchedInstallation(state, source)
	needsGuidedResolution := strings.Contains(strings.ToLower(source), "context7") && (len(targetSets) == 0 || len(targetSets[0]) == 0)
	if !needsGuidedResolution && len(targetSets) > 0 {
		for _, target := range targetSets[0] {
			needsGuidedResolution = needsGuidedResolution || strings.Contains(strings.ToLower(source), "context7") && (target == domain.ClientKiro || target == domain.ClientChatGPT)
		}
	}
	if isDirectorySelector(source) && app.DirectoryClient != nil && (len(state.Installations) > 0 || needsGuidedResolution) {
		bundle, err := app.DirectoryClient.Load(ctx, installedDirectoryFloor(state))
		if err != nil {
			return nil, err
		}
		productID, err := directorySelectorProductID(bundle.Snapshot, source)
		if err != nil && !errors.Is(err, domain.ErrDirectoryNotFound) {
			return nil, err
		}
		if err == nil && productID == "context7" {
			explicit[domain.ClientKiro] = domain.InstallIntentPrepare
			explicit[domain.ClientChatGPT] = domain.InstallIntentPrepare
		}
		if len(state.Installations) > 0 {
			installation, _, err = retainedDirectoryInstallation(state, source, productID)
			if err != nil {
				return nil, err
			}
		}
	}
	return lifecycleInstallIntents(installation, scope, explicit), nil
}

// Direct packages are already loaded before this decision, so only packages
// with useful Kiro components enter the guided preparation lane.
func applyLoadedGuidedIntents(opts *options, loaded loadedPackage, targets []domain.ClientID) {
	if opts.installIntents == nil {
		opts.installIntents = make(map[domain.ClientID]domain.InstallIntent)
	}
	if (loaded.origin != domain.OriginModeDirect && loaded.envelope.Manifest.Name != "context7") || (!loaded.envelope.MCP.Enabled && len(loaded.envelope.Skills) == 0) {
		return
	}
	for _, target := range targets {
		if target == domain.ClientKiro && opts.installIntents[target] == "" {
			opts.installIntents[target] = domain.InstallIntentPrepare
		}
		if target == domain.ClientChatGPT && loaded.chatGPTPreparation {
			opts.installIntents[target] = domain.InstallIntentPrepare
		}
	}
}

func preflightAddTargets(ctx context.Context, app App, opts *options, source string, targets []domain.ClientID, clients []domain.DetectedClient) ([]domain.DetectedClient, map[domain.ClientID]domain.DetectedClient, error) {
	return preflightSelectedTargets(ctx, app, targets, clients, !opts.dryRun && isDirectorySelector(source), opts.installIntents)
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
