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
	if !probeVersion || !hostedPrepareSelected(intents) {
		return detectClientsForLifecycleResolution(ctx, app.Detector, probeVersion)
	}
	clients, err := app.Detector.Detect(ctx)
	if err != nil {
		return nil, err
	}
	return app.mergeHostedPrepareProbes(ctx, clients, intents)
}

func (app App) mergeHostedPrepareProbes(ctx context.Context, clients []domain.DetectedClient, intents map[domain.ClientID]domain.InstallIntent) ([]domain.DetectedClient, error) {
	targets := versionProbeTargets(clients, intents)
	targeted, ok := app.Detector.(ports.TargetedVersionProbingClientDetector)
	if !ok || len(targets) == 0 {
		return clients, nil
	}
	probed, err := targeted.DetectTargetsWithVersionProbe(ctx, targets)
	if err != nil {
		return nil, err
	}
	applyProbedVersions(clients, probed, intents)
	return clients, nil
}

func versionProbeTargets(clients []domain.DetectedClient, intents map[domain.ClientID]domain.InstallIntent) []domain.ClientID {
	var targets []domain.ClientID
	for _, client := range clients {
		if !skipVersionProbeForPrepare(client.ClientID, intents) && client.Status == domain.DetectionDetected {
			targets = append(targets, client.ClientID)
		}
	}
	return targets
}

func applyProbedVersions(clients, probed []domain.DetectedClient, intents map[domain.ClientID]domain.InstallIntent) {
	for i, client := range clients {
		if skipVersionProbeForPrepare(client.ClientID, intents) {
			continue
		}
		for _, updated := range probed {
			if updated.ClientID == client.ClientID {
				clients[i] = updated
			}
		}
	}
}

func hostedPrepareSelected(intents map[domain.ClientID]domain.InstallIntent) bool {
	for id, intent := range intents {
		if intent == domain.InstallIntentPrepare && allowsHostedPrepare(id) {
			return true
		}
	}
	return false
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
	if isDirectorySelector(source) && app.DirectoryClient != nil && (len(state.Installations) > 0 || context7GuidedResolution(source, targetSets)) {
		installation, explicit, err = app.directoryLifecycleIntents(ctx, state, source, installation, explicit)
		if err != nil {
			return nil, err
		}
	}
	return lifecycleInstallIntents(installation, scope, explicit), nil
}

func context7GuidedResolution(source string, targetSets [][]domain.ClientID) bool {
	if !strings.Contains(strings.ToLower(source), "context7") {
		return false
	}
	if len(targetSets) == 0 || len(targetSets[0]) == 0 {
		return true
	}
	for _, target := range targetSets[0] {
		if allowsPrepare(target) {
			return true
		}
	}
	return false
}

func (app App) directoryLifecycleIntents(
	ctx context.Context,
	state domain.StateFileV2,
	source string,
	installation domain.Installation,
	explicit map[domain.ClientID]domain.InstallIntent,
) (domain.Installation, map[domain.ClientID]domain.InstallIntent, error) {
	bundle, err := app.DirectoryClient.Load(ctx, installedDirectoryFloor(state))
	if err != nil {
		return installation, explicit, err
	}
	productID, err := directorySelectorProductID(bundle.Snapshot, source)
	if err != nil && !errors.Is(err, domain.ErrDirectoryNotFound) {
		return installation, explicit, err
	}
	if err == nil && productID == "context7" {
		assignPrepareIntents(explicit)
	}
	if len(state.Installations) > 0 {
		installation, _, err = retainedDirectoryInstallation(state, source, productID)
		if err != nil {
			return installation, explicit, err
		}
	}
	return installation, explicit, nil
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
		assignLoadedGuidedIntent(opts, loaded, target)
	}
}

func assignLoadedGuidedIntent(opts *options, loaded loadedPackage, target domain.ClientID) {
	if requiresPersonalMapping(target) {
		if loaded.chatGPTPreparation {
			opts.installIntents[target] = domain.InstallIntentPrepare
		}
		return
	}
	if allowsHostedPrepare(target) && opts.installIntents[target] == "" {
		opts.installIntents[target] = domain.InstallIntentPrepare
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
