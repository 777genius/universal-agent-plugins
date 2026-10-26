package agentpluginscli

import (
	"context"
	"fmt"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
)

// Project adapters can require a current version even for a local package.
// Validate the complete selection before executing the one missing version
// probe, and retain the original discovery facts for every client.
func preflightProjectLifecycleTargets(ctx context.Context, app App, opts *options, targets []domain.ClientID, clients []domain.DetectedClient, probeVersion bool, intents ...map[domain.ClientID]domain.InstallIntent) ([]domain.DetectedClient, map[domain.ClientID]domain.DetectedClient, error) {
	selected, detected, err := preflightSelectedTargets(ctx, app, targets, clients, probeVersion, intents...)
	if err != nil {
		return nil, nil, err
	}
	if err := ensureSelectedProjectVersions(ctx, app, opts, targets, detected); err != nil {
		return nil, nil, err
	}
	for i := range selected {
		selected[i] = detected[selected[i].ClientID]
	}
	return selected, detected, nil
}

func ensureSelectedProjectVersions(ctx context.Context, app App, opts *options, targets []domain.ClientID, detected map[domain.ClientID]domain.DetectedClient) error {
	if app.scope() != domain.ScopeProject || opts.dryRun {
		return nil
	}
	selected := false
	for _, target := range targets {
		selected = selected || target == domain.ClientKimi
	}
	client, ok := detected[domain.ClientKimi]
	if !selected || !ok || client.Status != domain.DetectionDetected || client.DetectionError != nil || client.ExecutablePath == "" || strings.TrimSpace(client.Version) != "" {
		return nil
	}
	targeted, ok := app.Detector.(ports.TargetedVersionProbingClientDetector)
	if !ok {
		// Keep the existing adapter version precondition closed. A detector
		// without targeted probing must never execute unrelated clients.
		return nil
	}
	probed, err := targeted.DetectTargetsWithVersionProbe(ctx, []domain.ClientID{domain.ClientKimi})
	if err != nil {
		return fmt.Errorf("target %q version detection failed; no target was changed: %w", domain.ClientKimi, err)
	}
	updated, err := selectedKimiVersion(client, probed)
	if err != nil {
		return err
	}
	client.Version = updated
	detected[domain.ClientKimi] = client
	return nil
}

func selectedKimiVersion(original domain.DetectedClient, probed []domain.DetectedClient) (string, error) {
	var client domain.DetectedClient
	matches := 0
	for _, candidate := range probed {
		if candidate.ClientID == domain.ClientKimi {
			client = candidate
			matches++
		}
	}
	if matches != 1 {
		return "", fmt.Errorf("target %q version detection returned %d matching clients; no target was changed", domain.ClientKimi, matches)
	}
	if client.Status != domain.DetectionDetected || client.DetectionError != nil || strings.TrimSpace(client.Version) == "" {
		return "", fmt.Errorf("target %q version detection did not return a successful version; no target was changed", domain.ClientKimi)
	}
	if client.ExecutablePath != original.ExecutablePath || client.ConfigRoot != original.ConfigRoot || client.ProfileNamespace != original.ProfileNamespace || !domain.SameProfileAuthority(client.ProfileAuthority, original.ProfileAuthority) {
		return "", fmt.Errorf("target %q identity changed during version detection; no target was changed", domain.ClientKimi)
	}
	return client.Version, nil
}
