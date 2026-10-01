package agentpluginscli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/cli/installerui"
	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/prompt"
	"github.com/777genius/plugin-kit-ai/cli/internal/promptio"
	"github.com/777genius/plugin-kit-ai/cli/internal/terminaltheme"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
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
		client, err := selectExplicitClient(target, opts.target, detectedMap)
		return client, detectedMap, err
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
	if err != nil && !errors.Is(err, io.EOF) {
		return domain.DetectedClient{}, detectedMap, err
	}
	choice, err := strconv.Atoi(strings.TrimSpace(line))
	if err != nil || choice < 1 || choice > len(detected) {
		return domain.DetectedClient{}, detectedMap, fmt.Errorf("invalid client selection")
	}
	return detected[choice-1], detectedMap, nil
}

func selectExplicitClient(target domain.ClientID, rawTarget string, detectedMap map[domain.ClientID]domain.DetectedClient) (domain.DetectedClient, error) {
	if strings.EqualFold(strings.TrimSpace(rawTarget), "openai") {
		return domain.DetectedClient{}, fmt.Errorf("target %q is ambiguous; use --target codex or --target chatgpt", rawTarget)
	}
	client, ok := detectedMap[target]
	if err := selectedDetectionError(target, detectedMap); err != nil {
		return domain.DetectedClient{}, err
	}
	if !ok && plansWithoutHostPresence(target) {
		client = syntheticUndetectedClient(target)
		detectedMap[target] = client
		ok = true
	}
	if !ok || (client.Status != domain.DetectionDetected && !plansWithoutHostPresence(target)) {
		return domain.DetectedClient{}, fmt.Errorf("target %q was not detected", rawTarget)
	}
	return client, nil
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
	return promptYesNoDefault(ctx, reader, writer, alternate, question, false)
}

func promptYesNoDefault(ctx context.Context, reader io.Reader, writer, alternate io.Writer, question string, defaultYes bool) (bool, error) {
	if ctx == nil {
		ctx = context.Background()
	} // Preserve the legacy direct caller seam.
	writer, err := promptio.VisibleOutput(writer, alternate)
	if err != nil {
		return false, prompt.NormalizeIOError(err)
	}
	title := prompt.SafeText(question)
	for _, hint := range []string{" [y/N]", " [Y/n]", " [y/n]"} {
		title = strings.TrimSuffix(title, hint)
	}
	req := installerui.ConfirmRequest{Title: title, Default: defaultYes}
	var result installerui.Confirmation
	if input, ok := reader.(*os.File); ok {
		output, ok := terminaltheme.Unwrap(writer).(*os.File)
		if !ok {
			return false, prompt.ErrPromptUnavailable
		}
		t, e := installerui.NewTerminal(installerui.TerminalConfig{Input: input, Output: output, Mode: installerui.ModePlain, NoColor: !terminaltheme.For(writer).Enabled})
		if e != nil {
			return false, normalizeTerminalConsentError(e)
		}
		result, err = t.Confirm(ctx, req)
	} else {
		// Existing injected nonterminal callers preserve the public line API seam.
		u, e := installerui.New(installerui.Config{Input: reader, Output: writer, ReadLine: promptio.ReadLine})
		if e != nil {
			return false, normalizeTerminalConsentError(e)
		}
		result, err = u.Confirm(ctx, req)
	}
	if err != nil {
		return false, normalizeTerminalConsentError(err)
	}
	if result.Cancelled {
		return false, prompt.ErrPromptCanceled
	}
	return result.Accepted, nil
}

func normalizeTerminalConsentError(err error) error {
	if errors.Is(err, installerui.ErrUnavailable) {
		return prompt.NormalizeIOError(errors.Join(promptio.ErrUnavailable, err))
	}
	if errors.Is(err, installerui.ErrCancelled) {
		return errors.Join(prompt.ErrPromptCanceled, err)
	}
	return prompt.NormalizeIOError(err)
}

func readInputLine(ctx context.Context, reader io.Reader) (string, error) {
	line, err := promptio.ReadLine(ctx, reader)
	return line, prompt.NormalizeIOError(err)
}
