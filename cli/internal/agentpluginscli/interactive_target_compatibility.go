package agentpluginscli

import (
	"context"
	"fmt"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/discoveryv1"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func (app App) compatibleDetectedTargets(ctx context.Context, source string, detected []domain.DetectedClient, intents ...map[domain.ClientID]domain.InstallIntent) ([]domain.DetectedClient, []targetSkip, *loadedPackage, error) {
	candidates := detected
	var skipped []targetSkip

	switch {
	case strings.HasPrefix(source, "discovery:"):
		return app.compatibleDiscoveryDetectedTargets(ctx, source, detected)
	case isDirectorySelector(source):
		compatible, err := app.compatibleDirectoryTargets(ctx, source, detected, intents...)
		if err != nil {
			return nil, nil, nil, err
		}
		skipped = append(skipped, skippedTargets(subtractDetectedClients(detected, compatible), "the catalog has no compatible release for this combination of clients and system; check this client separately with --target")...)
		candidates = compatible
		if len(candidates) == 0 {
			return nil, skipped, nil, nil
		}
	}
	if len(intents) > 0 && personalMappingPrepareSelected(intents[0]) {
		app.chatGPTPreparation = true
	}
	return app.loadCompatibleDetectedTargets(ctx, source, detected, candidates, skipped, intents...)
}

func (app App) compatibleDiscoveryDetectedTargets(ctx context.Context, source string, detected []domain.DetectedClient) ([]domain.DetectedClient, []targetSkip, *loadedPackage, error) {
	compatible, err := app.compatibleDiscoveryTargets(ctx, source, detected)
	if err != nil {
		return nil, nil, nil, err
	}
	skipped := skippedTargets(subtractDetectedClients(detected, compatible), "not listed as compatible in the signed Discovery record; inspect this client separately with --target")
	return compatible, skipped, nil, nil
}

func (app App) loadCompatibleDetectedTargets(
	ctx context.Context,
	source string,
	detected, candidates []domain.DetectedClient,
	skipped []targetSkip,
	intents ...map[domain.ClientID]domain.InstallIntent,
) ([]domain.DetectedClient, []targetSkip, *loadedPackage, error) {
	request := app.addResolutionRequest(source, nil)
	if isDirectorySelector(source) {
		request = withDetectedClients(app.addResolutionRequest(source, detectedClientIDs(candidates)), detectedClientMap(detected))
	}
	loaded, err := app.loadPackageFor(ctx, source, request)
	if err != nil {
		return nil, nil, nil, err
	}
	compatible, rejected := app.compatibleLoadedTargets(ctx, loaded, candidates, intents...)
	skipped = append(skipped, rejected...)
	return compatible, skipped, &loaded, nil
}

func (app App) compatibleDiscoveryTargets(ctx context.Context, selector string, detected []domain.DetectedClient) ([]domain.DetectedClient, error) {
	if app.DiscoveryClient == nil {
		return nil, fmt.Errorf("signed Discovery Index dependencies are unavailable; use owner/repository@FULL_SHA[//path]")
	}
	bundle, err := app.DiscoveryClient.Load(ctx, 0)
	if err != nil {
		return nil, fmt.Errorf("load signed Discovery Index: %w", err)
	}
	record, err := resolveDiscoveryRecord(bundle, selector)
	if err != nil {
		return nil, err
	}
	allowed := make(map[string]struct{}, len(record.CompatibleClients))
	for _, client := range record.CompatibleClients {
		allowed[client] = struct{}{}
	}
	compatible := make([]domain.DetectedClient, 0, len(detected))
	for _, client := range detected {
		if _, ok := allowed[string(client.ClientID)]; ok {
			compatible = append(compatible, client)
		}
	}
	return compatible, nil
}

func detectedClientIDs(clients []domain.DetectedClient) []domain.ClientID {
	result := make([]domain.ClientID, len(clients))
	for index, client := range clients {
		result[index] = client.ClientID
	}
	return result
}

func detectedClientMap(clients []domain.DetectedClient) map[domain.ClientID]domain.DetectedClient {
	result := make(map[domain.ClientID]domain.DetectedClient, len(clients))
	for _, client := range clients {
		result[client.ClientID] = client
	}
	return result
}

func resolveDiscoveryRecord(bundle discoveryv1.VerifiedBundle, selector string) (discoveryv1.Record, error) {
	var matches []discoveryv1.Record
	for _, record := range bundle.Search.Records {
		if record.Slug == selector {
			matches = append(matches, record)
		}
	}
	if len(matches) == 0 {
		return discoveryv1.Record{}, fmt.Errorf("discovery package %q was not found", selector)
	}
	if len(matches) != 1 {
		return discoveryv1.Record{}, fmt.Errorf("discovery package %q is ambiguous", selector)
	}
	if matches[0].Availability != "available" {
		return discoveryv1.Record{}, fmt.Errorf("discovery package %q is unavailable and cannot be newly acquired", selector)
	}
	return matches[0], nil
}
