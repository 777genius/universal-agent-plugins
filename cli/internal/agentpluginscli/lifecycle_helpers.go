package agentpluginscli

import (
	"context"
	"fmt"
	"io"
	"maps"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/transaction"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

func defaultBoundTargets(app App, selector, scope string, degradedOnly bool) ([]domain.ClientID, error) {
	state, err := app.StateStore.Load()
	if err != nil {
		return nil, err
	}
	installation, err := selectInstallation(state, selector)
	if err != nil {
		return nil, err
	}
	var targets []domain.ClientID
	for _, binding := range installation.Clients {
		if binding.Scope != scope || binding.Materialization == domain.MaterializationAbsent {
			continue
		}
		if degradedOnly && binding.Materialization != domain.MaterializationDegraded && binding.Activation != domain.ActivationFailed && binding.Activation != domain.ActivationManual && binding.Activation != domain.ActivationPrepared && binding.Authentication != domain.AuthenticationFailed && binding.Authentication != domain.AuthenticationPending && binding.Verification != domain.VerificationFailed {
			continue
		}
		targets = append(targets, bindingSurfaceTargets(binding)...)
	}
	if len(targets) == 0 {
		if degradedOnly {
			return defaultBoundTargets(app, selector, scope, false)
		}
		return nil, fmt.Errorf("plugin has no materialized target in %s scope", scope)
	}
	sortTargets(targets)
	return targets, nil
}

func defaultInstalledBindingTargets(ctx context.Context, app App, selector, scope string, degradedOnly bool) ([]domain.ClientID, error) {
	state, err := app.StateStore.Load()
	if err != nil {
		return nil, err
	}
	installation, err := selectInstallation(state, selector)
	if err != nil {
		return nil, err
	}
	var bindingTargets []domain.ClientID
	for _, binding := range installation.Clients {
		if binding.Scope != scope || binding.Materialization == domain.MaterializationAbsent {
			continue
		}
		if degradedOnly && binding.Materialization != domain.MaterializationDegraded && binding.Activation != domain.ActivationFailed && binding.Activation != domain.ActivationManual && binding.Activation != domain.ActivationPrepared && binding.Authentication != domain.AuthenticationFailed && binding.Authentication != domain.AuthenticationPending && binding.Verification != domain.VerificationFailed {
			continue
		}
		bindingTargets = append(bindingTargets, domain.ClientID(binding.ClientID))
	}
	if len(bindingTargets) == 0 {
		if degradedOnly {
			return defaultInstalledBindingTargets(ctx, app, selector, scope, false)
		}
		return nil, fmt.Errorf("plugin has no materialized target in %s scope", scope)
	}
	return resolveInstalledBindingTargets(ctx, app, bindingTargets, false)
}

func promptBoundTargets(cmd *cobra.Command, app App, selector, scope string) (string, error) {
	targets, err := defaultBoundTargets(app, selector, scope, false)
	if err != nil {
		return "", err
	}
	if len(targets) == 1 {
		return string(targets[0]), nil
	}
	_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Installed targets (all selected by default):")
	for index, target := range targets {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  %d. [x] %s\n", index+1, target)
	}
	_, _ = fmt.Fprint(cmd.OutOrStdout(), "Choose targets by number, comma-separated [all]: ")
	line, readErr := readInputLine(cmd.Context(), cmd.InOrStdin())
	if readErr != nil && readErr != io.EOF {
		return "", readErr
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return joinTargets(targets), nil
	}
	seen := map[int]struct{}{}
	var selected []domain.ClientID
	for _, raw := range strings.Split(line, ",") {
		choice, err := strconv.Atoi(strings.TrimSpace(raw))
		if err != nil || choice < 1 || choice > len(targets) {
			return "", fmt.Errorf("invalid installed-target multiselect")
		}
		if _, duplicate := seen[choice]; duplicate {
			return "", fmt.Errorf("duplicate installed-target multiselect choice %d", choice)
		}
		seen[choice] = struct{}{}
		selected = append(selected, targets[choice-1])
	}
	return joinTargets(selected), nil
}

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
