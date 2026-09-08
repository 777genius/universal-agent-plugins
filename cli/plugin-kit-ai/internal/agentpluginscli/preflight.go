package agentpluginscli

import (
	"context"
	"fmt"
	"sort"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	clientplanner "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/planner"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
)

func detectClientsForLifecycleResolution(ctx context.Context, detector ports.ClientDetector, probeVersion bool) ([]domain.DetectedClient, error) {
	if probing, ok := detector.(ports.VersionProbingClientDetector); probeVersion && ok {
		return probing.DetectWithVersionProbe(ctx)
	}
	return detector.Detect(ctx)
}

// preflightSelectedTargets resolves the complete caller-selected client set
// before any package or Directory work begins. ChatGPT is the sole synthetic
// client: signed compatibility may authorize preparing its local package even
// though the remote product cannot be detected locally.
func preflightSelectedTargets(ctx context.Context, app App, targets []domain.ClientID, clients []domain.DetectedClient, probeVersion bool) ([]domain.DetectedClient, map[domain.ClientID]domain.DetectedClient, error) {
	if clients == nil {
		detected, err := detectClientsForLifecycleResolution(ctx, app.Detector, probeVersion)
		if err != nil {
			return nil, nil, fmt.Errorf("detect AI clients: %w", err)
		}
		clients = detected
	}
	detected := make(map[domain.ClientID]domain.DetectedClient, len(clients)+1)
	for _, client := range clients {
		detected[client.ClientID] = client
	}
	selected := make([]domain.DetectedClient, 0, len(targets))
	for _, target := range targets {
		client, ok := detectedSharedClient(target, detected)
		if target == domain.ClientChatGPT && !ok {
			client = domain.DetectedClient{ClientID: target, DisplayName: "ChatGPT", Status: domain.DetectionNotDetected}
			detected[target] = client
			ok = true
		}
		if !ok || (client.Status != domain.DetectionDetected && target != domain.ClientChatGPT) {
			return nil, nil, fmt.Errorf("target %q was not detected; no target was changed", target)
		}
		detected[target] = client
		selected = append(selected, client)
	}
	return selected, detected, nil
}

// preflightInstalledBindings resolves one real, currently detected client for
// each installed physical binding. Unlike preflightSelectedTargets, it may use
// the other member of the Copilot/VS Code shared backend. This is safe only for
// compatibility and lifecycle checks of an already-recorded physical binding;
// explicit user targets must continue through the strict preflight above.
func preflightInstalledBindings(bindingTargets []domain.ClientID, detected map[domain.ClientID]domain.DetectedClient) ([]domain.DetectedClient, error) {
	selected := make([]domain.DetectedClient, 0, len(bindingTargets))
	seen := make(map[domain.ClientID]struct{}, len(bindingTargets))
	for _, target := range bindingTargets {
		client, ok := clientplanner.DetectedPhysicalClient(target, detected)
		if target == domain.ClientChatGPT && !ok {
			client = domain.DetectedClient{ClientID: target, DisplayName: "ChatGPT", Status: domain.DetectionNotDetected}
			ok = true
		}
		if !ok {
			return nil, fmt.Errorf("installed target %q was not detected; no target was changed", target)
		}
		if _, duplicate := seen[client.ClientID]; duplicate {
			continue
		}
		seen[client.ClientID] = struct{}{}
		selected = append(selected, client)
	}
	return selected, nil
}

func resolveInstalledBindingTargets(ctx context.Context, app App, bindingTargets []domain.ClientID, probeVersion bool) ([]domain.ClientID, error) {
	clients, err := detectClientsForLifecycleResolution(ctx, app.Detector, probeVersion)
	if err != nil {
		return nil, fmt.Errorf("detect AI clients: %w", err)
	}
	detected := make(map[domain.ClientID]domain.DetectedClient, len(clients))
	for _, client := range clients {
		detected[client.ClientID] = client
	}
	resolved, err := preflightInstalledBindings(bindingTargets, detected)
	if err != nil {
		return nil, err
	}
	targets := make([]domain.ClientID, 0, len(resolved))
	for _, client := range resolved {
		targets = append(targets, client.ClientID)
	}
	sortTargets(targets)
	return targets, nil
}

func detectedClientValues(clients map[domain.ClientID]domain.DetectedClient) []domain.DetectedClient {
	values := make([]domain.DetectedClient, 0, len(clients))
	for _, client := range clients {
		values = append(values, client)
	}
	sort.Slice(values, func(i, j int) bool { return values[i].ClientID < values[j].ClientID })
	return values
}
