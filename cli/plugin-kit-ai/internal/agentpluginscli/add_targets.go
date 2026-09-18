package agentpluginscli

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/prompt"
	"github.com/777genius/plugin-kit-ai/cli/internal/promptio"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/spf13/cobra"
)

func selectClient(
	cmd *cobra.Command,
	app App,
	opts *options,
	clients []domain.DetectedClient,
) (domain.DetectedClient, map[domain.ClientID]domain.DetectedClient, error) {
	detectedMap := make(map[domain.ClientID]domain.DetectedClient, len(clients))
	var detected []domain.DetectedClient
	for _, client := range clients {
		detectedMap[client.ClientID] = client
		if client.Status == domain.DetectionDetected {
			detected = append(detected, client)
		}
	}
	target := normalizeTarget(opts.target)
	if target != "" {
		if strings.EqualFold(strings.TrimSpace(opts.target), "openai") {
			return domain.DetectedClient{}, detectedMap, fmt.Errorf("target %q is ambiguous; use --target codex or --target chatgpt", opts.target)
		}
		client, ok := detectedMap[target]
		if !ok && plansWithoutHostPresence(target) {
			client = syntheticUndetectedClient(target)
			detectedMap[target] = client
			ok = true
		}
		if !ok || (client.Status != domain.DetectionDetected && !plansWithoutHostPresence(target)) {
			return domain.DetectedClient{}, detectedMap, fmt.Errorf("target %q was not detected", opts.target)
		}
		return client, detectedMap, nil
	}
	if len(detected) == 0 {
		return domain.DetectedClient{}, detectedMap, fmt.Errorf("no supported local AI client was detected; use --target chatgpt for ChatGPT, or install/detect another client")
	}
	if len(detected) == 1 {
		return detected[0], detectedMap, nil
	}
	if !app.Terminal || opts.format == "json" {
		return domain.DetectedClient{}, detectedMap, fmt.Errorf("multiple clients detected; choose one or more with --target codex,cursor")
	}
	sort.SliceStable(detected, func(i, j int) bool { return targetOrder(detected[i].ClientID) < targetOrder(detected[j].ClientID) })
	_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Detected clients:")
	for index, client := range detected {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  %d. %s\n", index+1, client.DisplayName)
	}
	_, _ = fmt.Fprint(cmd.OutOrStdout(), "Choose one target: ")
	line, err := readInputLine(cmd.Context(), cmd.InOrStdin())
	if err != nil && err != io.EOF {
		return domain.DetectedClient{}, detectedMap, err
	}
	choice, err := strconv.Atoi(strings.TrimSpace(line))
	if err != nil || choice < 1 || choice > len(detected) {
		return domain.DetectedClient{}, detectedMap, fmt.Errorf("invalid client selection")
	}
	return detected[choice-1], detectedMap, nil
}

func normalizeTarget(value string) domain.ClientID {
	id, _ := domain.ParseClientID(value)
	return id
}

func backendExecutable(selected domain.DetectedClient, clients map[domain.ClientID]domain.DetectedClient) string {
	if path := strings.TrimSpace(selected.ExecutablePath); path != "" {
		return path
	}
	definition, _ := domain.ClientDefinitionFor(selected.ClientID)
	if definition.Capabilities.PackageMode == domain.PackageNative {
		return selected.ExecutablePath
	}
	for _, sibling := range domain.BackendSiblings(selected.ClientID) {
		if path := strings.TrimSpace(clients[sibling].ExecutablePath); path != "" {
			return path
		}
	}
	return selected.ExecutablePath
}

// detectedSharedClient resolves a logical surface through an already detected
// native sibling backend. It does not invent that backend from a logical
// surface when the backend itself is absent.
func detectedSharedClient(target domain.ClientID, clients map[domain.ClientID]domain.DetectedClient) (domain.DetectedClient, bool) {
	client, exact := clients[target]
	if exact && client.Status == domain.DetectionDetected {
		return client, true
	}
	definition, ok := domain.ClientDefinitionFor(target)
	if !ok || definition.Capabilities.PackageMode == domain.PackageNative {
		return client, exact
	}
	for _, sibling := range domain.BackendSiblings(target) {
		peer, found := clients[sibling]
		if !found || peer.Status != domain.DetectionDetected {
			continue
		}
		peerDefinition, peerOK := domain.ClientDefinitionFor(sibling)
		if !peerOK || peerDefinition.Capabilities.PackageMode != domain.PackageNative {
			continue
		}
		client = peer
		client.ClientID = target
		if definition.DisplayName != "" {
			client.DisplayName = definition.DisplayName + " (shared " + definition.BackendFamily + " backend)"
		}
		client.ExecutablePath = ""
		client.ConfigRoot = ""
		clients[target] = client
		return client, true
	}
	return client, exact
}

func promptYesNo(ctx context.Context, reader io.Reader, writer, alternate io.Writer, question string) (bool, error) {
	var err error
	writer, err = promptio.VisibleOutput(writer, alternate)
	if err != nil {
		return false, err
	}
	if _, err := fmt.Fprint(&planWriter{writer: writer}, prompt.SafeText(question)+" "); err != nil {
		return false, err
	}
	line, err := promptio.ReadLine(ctx, reader)
	if err != nil {
		return false, err
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == "yes", nil
}

func readInputLine(ctx context.Context, reader io.Reader) (string, error) {
	return promptio.ReadLine(ctx, reader)
}
