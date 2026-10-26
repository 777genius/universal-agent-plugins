package agentpluginscli

import (
	"context"
	"fmt"
	"path/filepath"
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
	requested := missingProjectVersionTargets(targets, detected)
	if len(requested) == 0 {
		return nil
	}
	targeted, ok := app.Detector.(ports.TargetedVersionProbingClientDetector)
	if !ok {
		// Keep the existing adapter version precondition closed. A detector
		// without targeted probing must never execute unrelated clients.
		return nil
	}
	probed, err := targeted.DetectTargetsWithVersionProbe(ctx, requested)
	if err != nil {
		return fmt.Errorf("targets %v version detection failed; no target was changed: %w", requested, err)
	}
	versions := make(map[domain.ClientID]string, len(requested))
	for _, id := range requested {
		version, err := selectedProjectVersion(detected[id], probed)
		if err != nil {
			return err
		}
		versions[id] = version
	}
	for _, id := range requested {
		client := detected[id]
		client.Version = versions[id]
		detected[id] = client
	}
	return nil
}

func missingProjectVersionTargets(targets []domain.ClientID, detected map[domain.ClientID]domain.DetectedClient) []domain.ClientID {
	var requested []domain.ClientID
	seen := map[domain.ClientID]bool{}
	for _, id := range targets {
		client, ok := detected[id]
		if seen[id] || !ok || client.Status != domain.DetectionDetected || client.DetectionError != nil || client.ExecutablePath == "" || strings.TrimSpace(client.Version) != "" {
			continue
		}
		if id != domain.ClientKimi {
			name := strings.ToLower(filepath.Base(client.ExecutablePath))
			if id != domain.ClientWindsurf || !filepath.IsAbs(client.ExecutablePath) || filepath.Clean(client.ExecutablePath) != client.ExecutablePath || (name != "devin" && name != "devin.exe") {
				continue
			}
		}
		requested = append(requested, id)
		seen[id] = true
	}
	return requested
}

func selectedProjectVersion(original domain.DetectedClient, probed []domain.DetectedClient) (string, error) {
	var client domain.DetectedClient
	matches := 0
	for _, candidate := range probed {
		if candidate.ClientID == original.ClientID {
			client = candidate
			matches++
		}
	}
	if matches != 1 {
		return "", fmt.Errorf("target %q version detection returned %d matching clients; no target was changed", original.ClientID, matches)
	}
	if client.Status != domain.DetectionDetected || client.DetectionError != nil || strings.TrimSpace(client.Version) == "" {
		return "", fmt.Errorf("target %q version detection did not return a successful version; no target was changed", original.ClientID)
	}
	if client.ExecutablePath != original.ExecutablePath || client.ConfigRoot != original.ConfigRoot || client.ProfileNamespace != original.ProfileNamespace || !domain.SameProfileAuthority(client.ProfileAuthority, original.ProfileAuthority) {
		return "", fmt.Errorf("target %q identity changed during version detection; no target was changed", original.ClientID)
	}
	return client.Version, nil
}
