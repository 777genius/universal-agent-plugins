package agentpluginscli

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/discoveryv1"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// promptCompatibleDetectedTargets keeps the no-flag path convenient without
// turning ambient client detection into an unsafe all-or-nothing guess. It
// first narrows the installed clients to one complete target set that the
// selected package can actually serve, then asks the user to confirm it.
func promptCompatibleDetectedTargets(ctx context.Context, cmd *cobra.Command, app App, source string, intents ...map[domain.ClientID]domain.InstallIntent) ([]domain.ClientID, []domain.DetectedClient, *loadedPackage, error) {
	clients, err := app.Detector.Detect(ctx)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("detect AI clients: %w", err)
	}
	detected := detectedSupportedClients(clients)
	if len(detected) == 0 {
		selection, all, err := promptTargetChoices(cmd, app, detected, nil, clients)
		return selection, all, nil, err
	}

	compatible, skipped, preloaded, err := app.compatibleDetectedTargets(ctx, source, detected, intents...)
	if err != nil {
		return nil, nil, nil, err
	}
	selection, all, err := promptTargetChoices(cmd, app, compatible, skipped, clients)
	if err != nil {
		if preloaded != nil && preloaded.cleanup != nil {
			_ = preloaded.cleanup()
		}
		return nil, nil, nil, err
	}
	return selection, all, preloaded, nil
}

func detectedSupportedClients(clients []domain.DetectedClient) []domain.DetectedClient {
	result := make([]domain.DetectedClient, 0, len(clients))
	for _, client := range clients {
		if client.Status == domain.DetectionDetected && supportedTarget(client.ClientID) {
			result = append(result, client)
		}
	}
	sort.SliceStable(result, func(i, j int) bool { return targetOrder(result[i].ClientID) < targetOrder(result[j].ClientID) })
	return result
}

func subtractDetectedClients(all, selected []domain.DetectedClient) []domain.DetectedClient {
	kept := make(map[domain.ClientID]struct{}, len(selected))
	for _, client := range selected {
		kept[client.ClientID] = struct{}{}
	}
	result := make([]domain.DetectedClient, 0, len(all)-len(selected))
	for _, client := range all {
		if _, ok := kept[client.ClientID]; !ok {
			result = append(result, client)
		}
	}
	return result
}

// targetSkip contains only installer-owned guidance, never raw planner/provider errors.
type targetSkip struct {
	Client domain.ClientID
	Reason string
}

func skippedTargets(clients []domain.DetectedClient, reason string) []targetSkip {
	var skipped []targetSkip
	for _, client := range clients {
		skipped = append(skipped, targetSkip{Client: client.ClientID, Reason: strings.ReplaceAll(reason, "--target", "--target "+string(client.ClientID))})
	}
	return skipped
}

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
