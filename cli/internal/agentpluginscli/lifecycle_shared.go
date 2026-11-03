package agentpluginscli

import (
	"fmt"
	"io"
	"maps"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/transaction"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

func lifecycleService(app App, detected map[domain.ClientID]domain.DetectedClient) usecase.Service {
	service := app.Lifecycle
	service.StateStore = app.StateStore
	if app.Planner != nil {
		service.Planner = app.Planner
	}
	if app.Targets != nil {
		service.Targets = app.Targets
	}
	service.Detected = maps.Clone(detected)
	return service
}

func environmentAuthorityForBinding(state domain.StateFileV2, installationID string, clientID domain.ClientID, action string) (*domain.EnvironmentAuthority, error) {
	var binding domain.ClientBinding
	found := false
	for _, installation := range state.Installations {
		if installation.InstallationID != installationID {
			continue
		}
		for _, candidate := range installation.Clients {
			if candidate.ClientID == string(clientID) {
				binding, found = candidate, true
				break
			}
		}
	}
	if !found {
		return nil, nil
	}
	for _, receipt := range state.EnvironmentReceipts {
		if receipt.InstallationID == installationID && receipt.ClientBindingID == binding.ClientBindingID &&
			receipt.ClientID == string(clientID) {
			if action != "repair" {
				return nil, fmt.Errorf("%w: use deps sync for owned bindings", transaction.ErrEnvironmentAuthority)
			}
			return &domain.EnvironmentAuthority{EnvironmentID: receipt.EnvironmentID, Alias: receipt.Alias,
				ClientID: receipt.ClientID, ReceiptID: receipt.ReceiptID, InstallationID: receipt.InstallationID,
				ClientBindingID: receipt.ClientBindingID, SourceKey: receipt.SourceKey, ExpectedAction: action}, nil
		}
	}
	return nil, nil
}

func updateSource(installation domain.Installation) string {
	if installation.Source.SourceKind == "archive" && filepath.IsAbs(installation.Source.SourceURL) {
		return localArchiveUpdateSource(installation.Source)
	}
	if installation.Source.SourceKind != "" && installation.Source.RequestedSource == "" {
		return directSourceRequested(installation.Source)
	}
	if installation.Source.Repository == "" && filepath.IsAbs(installation.Source.CanonicalSource) {
		return installation.Source.CanonicalSource
	}
	if value := strings.TrimSpace(installation.Source.RequestedSource); value != "" {
		return value
	}
	if installation.Source.Repository != "" {
		value := "github:" + installation.Source.Repository
		if installation.Source.PackageSubpath != "" {
			value += "//" + installation.Source.PackageSubpath
		}
		return value
	}
	return installation.Source.CanonicalSource
}

func selectedRepairBinding(installation domain.Installation, clientID domain.ClientID, scope domain.InstallScope) (domain.ClientBinding, error) {
	var matches []domain.ClientBinding
	for _, binding := range installation.Clients {
		if binding.Scope == string(scope) && binding.Materialization != domain.MaterializationAbsent && bindingAffectsTarget(binding, clientID) {
			matches = append(matches, binding)
		}
	}
	if len(matches) != 1 {
		return domain.ClientBinding{}, fmt.Errorf("repair requires exactly one bound target for %s in %s scope", clientID, scope)
	}
	return matches[0], nil
}

func repairSource(installation domain.Installation, binding domain.ClientBinding) (string, error) {
	canonical := strings.TrimSpace(installation.Source.CanonicalSource)
	if installation.Source.Repository == "" && filepath.IsAbs(canonical) {
		return canonical, nil
	}
	revision := ""
	if binding.PackageRevision != nil {
		revision = strings.TrimSpace(binding.PackageRevision.ResolvedRevision)
	}
	if revision == "" {
		return "", fmt.Errorf("the selected client binding has no exact resolved revision; refusing repair from a moving source")
	}
	if installation.Source.SourceKind != "" {
		source := installation.Source
		source.ResolvedRevision = revision
		if source.SourceKind == "archive" {
			source.ArchiveSHA256 = strings.TrimPrefix(revision, "sha256:")
		}
		return directSourceRequested(source), nil
	}
	if repository := strings.TrimSpace(installation.Source.Repository); repository != "" {
		value := "github:" + repository + "@" + revision
		if subpath := strings.TrimSpace(installation.Source.PackageSubpath); subpath != "" {
			value += "//" + subpath
		}
		return value, nil
	}
	if canonical != "" {
		return canonical + "#" + revision, nil
	}
	return "", fmt.Errorf("the selected client binding source cannot be reconstructed safely")
}

func requireNonInteractiveMutation(app App, opts *options, action string) error {
	if !automatedMutation(app, opts) || strings.TrimSpace(opts.target) != "" {
		return nil
	}
	return fmt.Errorf("automated %s requires --target", action)
}

// automatedMutation identifies invocations where prompting is unavailable or
// would break the machine-readable JSON contract. These flows still require every
// command-specific selector/target guard.
func automatedMutation(app App, opts *options) bool {
	return !app.Terminal || opts.format == "json"
}

func mutationConfirmed(app App, opts *options) bool {
	return automatedMutation(app, opts) || strings.TrimSpace(opts.target) != ""
}

func selectBoundClient(
	cmd *cobra.Command,
	app App,
	opts *options,
	installation domain.Installation,
	clients []domain.DetectedClient,
	requireDetected bool,
	reader io.Reader,
) (domain.DetectedClient, map[domain.ClientID]domain.DetectedClient, error) {
	detectedMap := make(map[domain.ClientID]domain.DetectedClient, len(clients))
	for _, client := range clients {
		detectedMap[client.ClientID] = client
	}
	bound := make(map[domain.ClientID]struct{})
	for _, binding := range installation.Clients {
		if binding.Materialization != domain.MaterializationAbsent && binding.Scope == opts.scope {
			bound[domain.ClientID(binding.ClientID)] = struct{}{}
		}
	}
	if len(bound) == 0 {
		return domain.DetectedClient{}, detectedMap, fmt.Errorf("plugin has no materialized target in %s scope", opts.scope)
	}
	choose := func(clientID domain.ClientID) (domain.DetectedClient, error) {
		if _, ok := bound[clientID]; !ok {
			return domain.DetectedClient{}, fmt.Errorf("plugin is not installed for target %q in %s scope", clientID, opts.scope)
		}
		if err := selectedDetectionError(clientID, detectedMap); err != nil {
			return domain.DetectedClient{}, err
		}
		client, ok := detectedMap[clientID]
		if !ok {
			client = domain.DetectedClient{ClientID: clientID, DisplayName: string(clientID), Status: domain.DetectionNotDetected}
		}
		if requireDetected && client.Status != domain.DetectionDetected && !plansWithoutHostPresence(clientID) {
			return domain.DetectedClient{}, fmt.Errorf("target %q is no longer detected; remove remains available", clientID)
		}
		return client, nil
	}
	if strings.EqualFold(strings.TrimSpace(opts.target), "openai") {
		return domain.DetectedClient{}, detectedMap, fmt.Errorf("target %q is ambiguous; use --target codex or --target chatgpt", opts.target)
	}
	if target := normalizeTarget(opts.target); target != "" {
		client, err := choose(target)
		return client, detectedMap, err
	}
	ids := make([]domain.ClientID, 0, len(bound))
	for clientID := range bound {
		ids = append(ids, clientID)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	if len(ids) == 1 {
		client, err := choose(ids[0])
		return client, detectedMap, err
	}
	if !app.Terminal || opts.format == "json" {
		return domain.DetectedClient{}, detectedMap, fmt.Errorf("plugin has multiple installed targets; choose one or more with --target codex,cursor")
	}
	if _, err := fmt.Fprintln(cmd.OutOrStdout(), "Installed targets:"); err != nil {
		return domain.DetectedClient{}, detectedMap, err
	}
	for index, clientID := range ids {
		client := detectedMap[clientID]
		display := client.DisplayName
		if display == "" {
			display = string(clientID)
		}
		if _, err := fmt.Fprintf(cmd.OutOrStdout(), "  %d. %s\n", index+1, display); err != nil {
			return domain.DetectedClient{}, detectedMap, err
		}
	}
	if _, err := fmt.Fprint(cmd.OutOrStdout(), "Choose one target: "); err != nil {
		return domain.DetectedClient{}, detectedMap, err
	}
	line, readErr := readInputLine(cmd.Context(), reader)
	if readErr != nil && readErr != io.EOF {
		return domain.DetectedClient{}, detectedMap, readErr
	}
	choice, parseErr := strconv.Atoi(strings.TrimSpace(line))
	if parseErr != nil || choice < 1 || choice > len(ids) {
		return domain.DetectedClient{}, detectedMap, fmt.Errorf("invalid client selection")
	}
	client, err := choose(ids[choice-1])
	return client, detectedMap, err
}

func renderUpdateResult(writer io.Writer, format string, envelope domain.PackageEnvelope, result usecase.AddResult, dryRun bool) error {
	data := newAddResultData(envelope, result, dryRun)
	if format == "json" {
		return writeJSONOutput(writer, "update", data)
	}
	if dryRun {
		return renderHumanPlan(writer, envelope, result)
	}
	if err := renderOpenCodeRuntimeNotice(writer, result); err != nil {
		return err
	}
	if result.NoChange {
		if action := localTargetLifecycleAction(result, ""); action != "" {
			if _, err := fmt.Fprintf(writer, "Next: %s\n", action); err != nil {
				return err
			}
		}
		_, _ = fmt.Fprintln(writer, "Already up to date. No changes made.")
		return nil
	}
	if result.Mutated && fullyInstalled(result.Activation) {
		if reportsMCPToolNamespaceCollision(result.Plan.ClientID) && len(domain.SelectedMCPNames(result.Plan)) > 0 {
			_, _ = fmt.Fprintln(writer, "OpenCode MCP configuration updated and verified.")
		} else {
			_, _ = fmt.Fprintln(writer, "Updated and verified for the selected client.")
		}
		return nil
	}
	if result.Mutated {
		if result.Activation.Authentication == domain.AuthenticationPending {
			if result.Activation.Activation == domain.ActivationActive {
				_, _ = fmt.Fprintln(writer, "Package updated and client activation completed. Authentication is pending.")
			} else {
				_, _ = fmt.Fprintln(writer, "Package updated. Authentication and client activation are pending.")
			}
		} else if result.Activation.Authentication == domain.AuthenticationNotChecked && result.Activation.Activation == domain.ActivationActive {
			_, _ = fmt.Fprintln(writer, "Package updated and client activation verified. Authentication requirements have not been checked.")
		} else {
			_, _ = fmt.Fprintln(writer, "Package updated. Activation is not complete yet.")
		}
	}
	if action := nextLocalLifecycleAction(result); action != "" && !fullyInstalled(result.Activation) {
		_, _ = fmt.Fprintf(writer, "Next: %s\n", action)
	}
	return nil
}

func renderRemoveResult(writer io.Writer, format string, installation domain.Installation, result usecase.RemoveResult, dryRun bool) error {
	data := newRemoveResultData(installation, result, dryRun)
	if format == "json" {
		return writeJSONOutput(writer, "remove", data)
	}
	if dryRun {
		if result.Deactivation.ArtifactRemovalAllowed {
			_, _ = fmt.Fprintf(writer, "Would remove %s from %s. No changes made.\n", installation.DeclaredName, result.ClientID)
		} else {
			_, _ = fmt.Fprintf(writer, "Would not remove %s from %s until the external client uninstall is confirmed. No changes made.\n", installation.DeclaredName, result.ClientID)
		}
		for _, action := range result.Deactivation.UserActions {
			_, _ = fmt.Fprintf(writer, "Next: %s\n", action)
		}
		for _, action := range result.Deactivation.LocalActions {
			_, _ = fmt.Fprintf(writer, "Next: %s\n", action)
		}
		return nil
	}
	if result.Mutated {
		_, _ = fmt.Fprintln(writer, "Removed the selected managed package.")
	}
	if removalNeedsManualStep(result.Deactivation) {
		_, _ = fmt.Fprintln(writer, removalIncompleteNotice)
	}
	for _, action := range result.Deactivation.UserActions {
		_, _ = fmt.Fprintf(writer, "Next: %s\n", action)
	}
	for _, action := range result.Deactivation.LocalActions {
		_, _ = fmt.Fprintf(writer, "Next: %s\n", action)
	}
	return nil
}

// removalIncompleteNotice keeps partial removals explicit: without it a run
// that stopped for a manual client step reads like a finished removal.
const removalIncompleteNotice = "Removal is not complete: a manual client step is still required."

func removalNeedsManualStep(deactivation domain.DeactivationOutcome) bool {
	return !deactivation.ArtifactRemovalAllowed || len(deactivation.UserActions) > 0
}

type removeResultData struct {
	OperationID    string                  `json:"operation_id,omitempty"`
	Plugin         string                  `json:"plugin"`
	Version        string                  `json:"version,omitempty"`
	Source         string                  `json:"source"`
	Revision       string                  `json:"revision,omitempty"`
	TreeDigest     string                  `json:"tree_digest,omitempty"`
	ManifestDigest string                  `json:"manifest_digest,omitempty"`
	Directory      *domain.DirectoryOrigin `json:"directory,omitempty"`
	NextAction     string                  `json:"next_action,omitempty"`
	DryRun         bool                    `json:"dry_run"`
	Result         usecase.RemoveResult    `json:"result"`
}

func newRemoveResultData(installation domain.Installation, result usecase.RemoveResult, dryRun bool) removeResultData {
	return removeResultData{
		OperationID: result.Receipt.OperationID, Plugin: installation.DeclaredName,
		Version: installation.Package.Version, Source: publicSource(installation.Source), Revision: installation.Source.ResolvedRevision,
		TreeDigest: installation.Source.TreeDigest, ManifestDigest: installation.Package.ManifestDigest, Directory: cloneDirectoryOrigin(installation.Directory),
		NextAction: nextRemoveAction(result), DryRun: dryRun, Result: result,
	}
}

func nextRemoveAction(result usecase.RemoveResult) string {
	if len(result.Deactivation.LocalActions) > 0 {
		return result.Deactivation.LocalActions[0]
	}
	if len(result.Deactivation.UserActions) > 0 {
		return result.Deactivation.UserActions[0]
	}
	return ""
}
