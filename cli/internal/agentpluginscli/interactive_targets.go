package agentpluginscli

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/spf13/cobra"
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
	detectionSkipped := make([]targetSkip, 0)
	for _, client := range clients {
		if client.DetectionError != nil && supportedTarget(client.ClientID) {
			detectionSkipped = append(detectionSkipped, targetSkip{Client: client.ClientID, Reason: "client configuration could not be inspected safely; correct it and retry"})
		}
	}
	if len(detected) == 0 {
		selection, all, err := promptTargetChoices(cmd, app, detected, detectionSkipped, clients)
		return selection, all, nil, err
	}

	compatible, skipped, preloaded, err := app.compatibleDetectedTargets(ctx, source, detected, intents...)
	if err != nil {
		return nil, nil, nil, err
	}
	skipped = append(detectionSkipped, skipped...)
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
