package agentpluginscli

import (
	"context"
	"fmt"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
)

// Selected adapters can require an observed version even for a local package.
// Validate the complete selection before executing the one missing version
// probe, and retain the original discovery facts for every client.
func preflightProjectLifecycleTargets(ctx context.Context, app App, opts *options, targets []domain.ClientID, observedClients []domain.DetectedClient, probeVersion bool, intents ...map[domain.ClientID]domain.InstallIntent) ([]domain.DetectedClient, map[domain.ClientID]domain.DetectedClient, error) {
	selected, detected, err := preflightSelectedTargets(ctx, app, targets, observedClients, probeVersion, intents...)
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
	if (app.scope() != domain.ScopeProject && app.scope() != domain.ScopeUser) || opts.dryRun {
		return nil
	}
	if app.ClientRegistry == nil {
		return clients.ErrRegistryRequired
	}
	requested := missingProjectVersionTargets(app.ClientRegistry, targets, detected, app.scope())
	if len(requested) == 0 {
		return nil
	}
	targeted, ok := app.Detector.(ports.TargetedVersionProbingClientDetector)
	if !ok {
		// Keep the existing adapter version precondition closed. A detector
		// without targeted probing must never execute unrelated clients.
		return nil
	}
	return probeMissingProjectVersions(ctx, app.ClientRegistry, app.scope(), targeted, requested, detected)
}

// Preserve mandatory version requirements as one atomic probe. Optional
// client capabilities are queried separately, so their failures cannot
// suppress an existing mandatory host-version requirement.
func probeMissingProjectVersions(ctx context.Context, registry *clients.Registry, scope domain.InstallScope,
	detector ports.TargetedVersionProbingClientDetector, requested []domain.ClientID, detected map[domain.ClientID]domain.DetectedClient) error {
	mandatory, optional := splitProjectVersionProbes(registry, scope, requested)
	versions := make(map[domain.ClientID]string, len(requested))
	if len(mandatory) > 0 {
		probed, err := detector.DetectTargetsWithVersionProbe(ctx, mandatory)
		if err != nil {
			return fmt.Errorf("targets %v version detection failed; no target was changed: %w", mandatory, err)
		}
		for _, id := range mandatory {
			version, err := selectedProjectVersion(detected[id], probed)
			if err != nil {
				return err
			}
			versions[id] = version
		}
	}
	for _, id := range optional {
		if err := ctx.Err(); err != nil {
			return err
		}
		probed, err := detector.DetectTargetsWithVersionProbe(ctx, []domain.ClientID{id})
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			continue
		}
		if version, err := selectedProjectVersion(detected[id], probed); err == nil {
			versions[id] = version
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	for id, version := range versions {
		client := detected[id]
		client.Version = version
		detected[id] = client
	}
	return nil
}

func splitProjectVersionProbes(registry *clients.Registry, scope domain.InstallScope,
	requested []domain.ClientID) (mandatory, optional []domain.ClientID) {
	for _, id := range requested {
		probe, ok := clients.As[clients.BestEffortProjectVersionProbe](registry, id)
		if scope == domain.ScopeProject && ok && probe.ProjectVersionProbeBestEffort() {
			optional = append(optional, id)
		} else {
			mandatory = append(mandatory, id)
		}
	}
	return mandatory, optional
}

func missingProjectVersionTargets(registry *clients.Registry, targets []domain.ClientID, detected map[domain.ClientID]domain.DetectedClient, scopes ...domain.InstallScope) []domain.ClientID {
	scope := domain.ScopeProject
	if len(scopes) > 0 {
		scope = scopes[0]
	}
	var requested []domain.ClientID
	seen := map[domain.ClientID]bool{}
	for _, id := range targets {
		client, ok := detected[id]
		if seen[id] || !ok || client.Status != domain.DetectionDetected || client.DetectionError != nil || client.ExecutablePath == "" || strings.TrimSpace(client.Version) != "" {
			continue
		}
		required := false
		switch scope {
		case domain.ScopeProject:
			if requirement, ok := clients.As[clients.ProjectVersionRequirement](registry, id); ok {
				required = requirement.RequiresProjectVersion(client)
			}
		case domain.ScopeUser:
			if requirement, ok := clients.As[clients.UserVersionRequirement](registry, id); ok {
				required = requirement.RequiresUserVersion(client)
			}
		}
		if !required {
			continue
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
	if client.DetectionError != nil {
		return "", fmt.Errorf("target %q version detection failed; no target was changed: %w", original.ClientID, client.DetectionError)
	}
	if client.Status != domain.DetectionDetected || strings.TrimSpace(client.Version) == "" {
		return "", fmt.Errorf("target %q version detection did not return a successful version; no target was changed", original.ClientID)
	}
	if client.ExecutablePath != original.ExecutablePath || client.ConfigRoot != original.ConfigRoot || client.NativeConfigRoot != original.NativeConfigRoot || client.ProfileNamespace != original.ProfileNamespace || !domain.SameProfileAuthority(client.ProfileAuthority, original.ProfileAuthority) {
		return "", fmt.Errorf("target %q identity changed during version detection; no target was changed", original.ClientID)
	}
	return client.Version, nil
}
