package agentpluginscli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"path/filepath"
	"sort"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/dirswap"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	clientplanner "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/planner"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
	"github.com/spf13/cobra"
)

type publicClient struct {
	BindingID                 string                         `json:"-"`
	ClientID                  string                         `json:"client_id"`
	Scope                     string                         `json:"scope"`
	Materialization           domain.MaterializationState    `json:"materialization"`
	Activation                domain.ActivationState         `json:"activation"`
	Authentication            domain.AuthenticationState     `json:"authentication"`
	Policy                    domain.PolicyState             `json:"policy"`
	Verification              domain.VerificationState       `json:"verification"`
	PackageRevision           *publicClientRevision          `json:"package_revision,omitempty"`
	AffectedSurfaces          []string                       `json:"affected_surfaces,omitempty"`
	ReceiptReconciled         *bool                          `json:"receipt_reconciled,omitempty"`
	NativeDiscoveryReconciled *bool                          `json:"native_discovery_reconciled,omitempty"`
	NativeIdentityState       domain.NativeIdentityState     `json:"native_identity_state,omitempty"`
	ClientVersion             string                         `json:"client_version,omitempty"`
	NativeDiscoveryEvidence   *publicNativeDiscoveryEvidence `json:"native_discovery_evidence,omitempty"`
}

type publicNativeDiscoveryEvidence struct {
	Basis              string                   `json:"basis"`
	VersionOperation   publicVersionOperation   `json:"version_operation"`
	DiscoveryOperation publicDiscoveryOperation `json:"discovery_operation"`
}

type publicVersionOperation struct {
	Argv                  []string `json:"argv"`
	ObservedClientVersion string   `json:"observed_client_version,omitempty"`
}

type publicDiscoveryOperation struct {
	Argv       []string `json:"argv"`
	Discovered bool     `json:"discovered"`
	ProductID  string   `json:"product_id"`
}

type publicClientRevision struct {
	Version          string           `json:"version,omitempty"`
	ResolvedRevision string           `json:"resolved_revision,omitempty"`
	DistributionID   string           `json:"distribution_id,omitempty"`
	ReleaseSequence  uint64           `json:"release_sequence,omitempty"`
	TreeDigest       string           `json:"tree_digest"`
	ManifestDigest   string           `json:"manifest_digest"`
	Evidence         []publicEvidence `json:"evidence,omitempty"`
}

type publicDetectedClient struct {
	ClientID    domain.ClientID        `json:"client_id"`
	DisplayName string                 `json:"display_name"`
	Status      domain.DetectionStatus `json:"status"`
	Version     string                 `json:"version,omitempty"`
	Surfaces    []domain.ClientSurface `json:"surfaces,omitempty"`
}

type publicInstallation struct {
	InstallationID    string                    `json:"installation_id"`
	Name              string                    `json:"name"`
	Version           string                    `json:"version,omitempty"`
	Source            string                    `json:"source"`
	NeedsRebind       bool                      `json:"needs_rebind,omitempty"`
	Clients           []publicClient            `json:"clients"`
	Directory         *publicInstalledDirectory `json:"directory,omitempty"`
	Warnings          []publicSafetyWarning     `json:"warnings,omitempty"`
	MixedVersion      bool                      `json:"mixed_version"`
	ConvergenceAction string                    `json:"convergence_action,omitempty"`
}

type doctorFinding struct {
	Status           string `json:"status"`
	Code             string `json:"code"`
	Subject          string `json:"subject,omitempty"`
	InstallationName string `json:"installation_name,omitempty"`
	InstallationID   string `json:"installation_id,omitempty"`
	ClientID         string `json:"client_id,omitempty"`
	Message          string `json:"message"`
	RecoveryAction   string `json:"recovery_action,omitempty"`
}

type supportedClient struct {
	ClientID    domain.ClientID    `json:"client_id"`
	PackageMode domain.PackageMode `json:"package_mode"`
}

func newListCommand(app App, opts *options) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List tracked Agent Plugins installations",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := validateCommonOptions(opts); err != nil {
				return err
			}
			state, err := app.StateStore.Load()
			if err != nil {
				return err
			}
			installations := publicInstallations(state, false)
			if opts.format == "json" {
				return writeJSONOutput(cmd.OutOrStdout(), "list", map[string]any{"installations": installations})
			}
			return renderInstallationList(cmd.OutOrStdout(), installations)
		},
	}
}

func newInfoCommand(app App, opts *options) *cobra.Command {
	return &cobra.Command{
		Use:   "info <name-or-installation-id>",
		Short: "Show lifecycle state without exposing local paths",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateCommonOptions(opts); err != nil {
				return err
			}
			state, err := app.StateStore.Load()
			if err != nil {
				return err
			}
			installation, err := selectInstallation(state, args[0])
			if err != nil {
				if strings.Contains(err.Error(), "ambiguous") || !isDirectorySelector(strings.TrimSpace(args[0])) {
					return err
				}
				product, inspectErr := inspectDirectoryProduct(cmd.Context(), app, state, args[0], opts.target)
				if inspectErr != nil {
					return inspectErr
				}
				if opts.format == "json" {
					return writeJSONOutput(cmd.OutOrStdout(), "info", product)
				}
				return renderProductInspection(cmd.OutOrStdout(), product)
			}
			public, err := inspectInstalledProduct(cmd.Context(), app, state, installation)
			if err != nil {
				return fmt.Errorf("inspect signed Directory: %w", err)
			}
			if strings.TrimSpace(opts.target) != "" {
				if err := reconcileInstalledInfo(cmd.Context(), app, installation, opts.target, &public); err != nil {
					return err
				}
			}
			if opts.format == "json" {
				return writeJSONOutput(cmd.OutOrStdout(), "info", public)
			}
			return renderInstallation(cmd.OutOrStdout(), public)
		},
	}
}

func newDoctorCommand(app App, opts *options) *cobra.Command {
	return &cobra.Command{
		Use:   "doctor [name-or-installation-id]",
		Short: "Read-only health check for clients, state, and open operations",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateCommonOptions(opts); err != nil {
				return err
			}
			return runDoctor(cmd.Context(), cmd, app, opts, args)
		},
	}
}

func runDoctor(ctx context.Context, cmd *cobra.Command, app App, opts *options, args []string) error {
	clients, err := app.Detector.Detect(ctx)
	if err != nil {
		return err
	}
	state, err := app.StateStore.Load()
	if err != nil {
		return err
	}
	open, err := app.Lifecycle.Kernel.Directory.ListOpen()
	if err != nil {
		return err
	}
	publicClients := make([]publicDetectedClient, 0, len(clients))
	for _, client := range clients {
		publicClients = append(publicClients, publicDetectedClient{
			ClientID: client.ClientID, DisplayName: client.DisplayName, Status: client.Status, Version: client.Version,
			Surfaces: append([]domain.ClientSurface(nil), client.Surfaces...),
		})
	}
	report := struct {
		ReadOnly           bool                   `json:"read_only"`
		ToolVersion        string                 `json:"tool_version"`
		SupportedClients   []supportedClient      `json:"supported_clients"`
		Clients            []publicDetectedClient `json:"clients"`
		InstallationCount  int                    `json:"installation_count"`
		OpenOperationCount int                    `json:"open_operation_count"`
		Installation       *publicInstallation    `json:"installation,omitempty"`
		Findings           []doctorFinding        `json:"findings"`
	}{ReadOnly: true, ToolVersion: normalizedToolVersion(app.Version), SupportedClients: doctorSupportedClients(), Clients: publicClients,
		InstallationCount: len(publicInstallations(state, false)), OpenOperationCount: len(open)}
	selected := (*domain.Installation)(nil)
	if len(args) == 1 {
		installation, err := selectInstallation(state, args[0])
		if err != nil {
			return err
		}
		value, err := inspectInstalledProduct(ctx, app, state, installation)
		if err != nil {
			return fmt.Errorf("inspect signed Directory: %w", err)
		}
		report.Installation = &value
		selected = &installation
	}
	report.Findings = doctorFindings(ctx, app, clients, state, open, selected)
	safety, err := doctorDirectorySafetyFindings(ctx, app, state, selected, report.Installation)
	if err != nil {
		return fmt.Errorf("inspect signed Directory: %w", err)
	}
	if len(safety) > 0 {
		filtered := report.Findings[:0]
		for _, finding := range report.Findings {
			if finding.Code != "no_degradation_detected" {
				filtered = append(filtered, finding)
			}
		}
		report.Findings = append(filtered, safety...)
	}
	if opts.format == "json" {
		return writeJSONOutput(cmd.OutOrStdout(), "doctor", report)
	}
	_, _ = fmt.Fprintln(cmd.OutOrStdout(), "agentplugins doctor (read-only)")
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Tool version: %s\n", report.ToolVersion)
	for _, client := range clients {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  %s: %s\n", client.DisplayName, client.Status)
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Tracked installations: %d\n", report.InstallationCount)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Open recovery operations: %d\n", report.OpenOperationCount)
	for _, finding := range report.Findings {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  [%s] %s: %s\n", finding.Status, finding.Code, finding.Message)
		if finding.InstallationID != "" {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "    Installation: %s (%s)\n", finding.InstallationName, finding.InstallationID)
		}
		if finding.ClientID != "" {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "    Client: %s\n", finding.ClientID)
		}
		if finding.RecoveryAction != "" {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "    Recovery: %s\n", finding.RecoveryAction)
		}
	}
	if report.Installation != nil {
		return renderInstallation(cmd.OutOrStdout(), *report.Installation)
	}
	return nil
}

func normalizedToolVersion(version string) string {
	if version = strings.TrimSpace(version); version != "" {
		return version
	}
	return "development"
}

func doctorSupportedClients() []supportedClient {
	ids := domain.SupportedClientIDs()
	result := make([]supportedClient, 0, len(ids))
	for _, id := range ids {
		capabilities, _ := clientplanner.Capabilities(id)
		result = append(result, supportedClient{ClientID: id, PackageMode: capabilities.PackageMode})
	}
	return result
}

func doctorFindings(ctx context.Context, app App, detected []domain.DetectedClient, state domain.StateFileV2, open []dirswap.Receipt, selected *domain.Installation) []doctorFinding {
	var findings []doctorFinding
	detectedByID := make(map[string]domain.DetectedClient, len(detected))
	for _, client := range detected {
		detectedByID[string(client.ClientID)] = client
	}
	if len(open) > 0 {
		findings = append(findings, degradedFinding("open_recovery_operations", "operations", "an interrupted managed-directory operation requires recovery", "rerun the intended add, update, or remove command to perform transactional recovery"))
	}
	installations := state.Installations
	if selected != nil {
		installations = []domain.Installation{*selected}
	}
	for _, installation := range installations {
		if installation.NeedsRebind {
			findings = append(findings, scopedFinding("degraded", "source_rebind_required", installation, "", "the source identity needs an explicit rebind", "automatic mutation is intentionally blocked; recover the original source identity and review an explicit rebind against the recorded installation"))
		}
		for _, binding := range installation.Clients {
			if binding.Materialization == domain.MaterializationAbsent {
				continue
			}
			if _, supported := clientplanner.Capabilities(domain.ClientID(binding.ClientID)); !supported {
				findings = append(findings, scopedFinding("degraded", "unsupported_client_binding", installation, binding.ClientID, "the tracked binding names an unsupported client", blockedStateRecovery))
				continue
			}
			client, visible := detectedByID[binding.ClientID]
			if binding.ClientID == string(domain.ClientChatGPT) && !visible {
				client = domain.DetectedClient{ClientID: domain.ClientChatGPT, DisplayName: "ChatGPT", Status: domain.DetectionNotDetected}
			}
			if binding.ClientID != string(domain.ClientChatGPT) && (!visible || client.Status != domain.DetectionDetected) {
				findings = append(findings, scopedFinding("degraded", "client_not_visible", installation, binding.ClientID, "the package is tracked but no current client visibility evidence was detected", "install or launch the client so its CLI, desktop application, or configuration directory is visible, then rerun doctor"))
			}
			if binding.ClientID == string(domain.ClientChatGPT) {
				inventoryCurrent := packageInventoryAppliesToBinding(installation, binding)
				if inventoryCurrent && installation.Package.Inventory.MCPPresent && (!installation.Package.Inventory.AppPresent || len(installation.Package.Inventory.AppBindings) == 0) {
					findings = append(findings, scopedFinding("degraded", "chatgpt_app_binding_missing", installation, binding.ClientID, "the ChatGPT target has MCP content but no valid registered app mapping", "register the MCP connection in ChatGPT Developer Mode, add a valid root .app.json mapping, then update this target"))
				} else if !inventoryCurrent {
					findings = append(findings, scopedFinding("unknown", "chatgpt_registration_unverified", installation, binding.ClientID, "remote ChatGPT registration for this earlier client revision cannot be inferred from the latest package inventory", "verify the installed plugin revision and connection status in ChatGPT Plugins; update this target before relying on current package metadata"))
				} else if installation.Package.Inventory.AppPresent {
					findings = append(findings, scopedFinding("unknown", "chatgpt_registration_unverified", installation, binding.ClientID, "the .app.json mapping is package-valid, but remote ChatGPT registration cannot be observed locally", "verify the mapped connection and plugin status in ChatGPT Plugins; rerun add with --activation-complete only after checking it in a new chat"))
				}
			}
			if visible && client.Status == domain.DetectionDetected && binding.ClientID == string(domain.ClientCopilot) && strings.TrimSpace(client.ExecutablePath) == "" {
				findings = append(findings, scopedFinding("degraded", "copilot_cli_missing", installation, binding.ClientID, "GitHub Copilot CLI is unavailable for automatic Copilot activation", "install GitHub Copilot CLI, ensure copilot is on PATH, and rerun doctor"))
			}
			switch binding.Activation {
			case domain.ActivationManual, domain.ActivationPrepared:
				findings = append(findings, scopedFinding("degraded", "activation_pending", installation, binding.ClientID, "client activation is not complete", fmt.Sprintf("complete the displayed external activation step, then rerun the same `agentplugins add ... --target %s%s` command", binding.ClientID, doctorScopeFlag(binding))))
			case domain.ActivationFailed:
				findings = append(findings, scopedFinding("degraded", "activation_failed", installation, binding.ClientID, "client activation failed", fmt.Sprintf("resolve the client-reported activation error, then rerun the same `agentplugins add ... --target %s%s` command", binding.ClientID, doctorScopeFlag(binding))))
			}
			switch binding.Authentication {
			case domain.AuthenticationPending:
				findings = append(findings, scopedFinding("degraded", "authentication_pending", installation, binding.ClientID, "plugin authentication is pending", fmt.Sprintf("complete the displayed external authentication step, then rerun the same `agentplugins add ... --target %s%s` command", binding.ClientID, doctorScopeFlag(binding))))
			case domain.AuthenticationFailed:
				findings = append(findings, scopedFinding("degraded", "authentication_failed", installation, binding.ClientID, "plugin authentication failed", fmt.Sprintf("reauthorize the plugin in the selected client, then rerun the same `agentplugins add ... --target %s%s` command", binding.ClientID, doctorScopeFlag(binding))))
			case domain.AuthenticationNotChecked:
				findings = append(findings, scopedFinding("unknown", "authentication_not_checked", installation, binding.ClientID, "authentication requirements have not been verified", "check the package's authentication instructions and verify access in the selected client"))
			}
			if binding.Materialization == domain.MaterializationDegraded || binding.Verification == domain.VerificationFailed {
				findings = append(findings, scopedFinding("degraded", "installation_verification_failed", installation, binding.ClientID, "the managed package is marked degraded or failed verification", repairAction(installation, binding)))
			}
			if binding.ClientID == string(domain.ClientChatGPT) || (visible && client.Status == domain.DetectionDetected) {
				findings = append(findings, checkManagedIntegrity(ctx, app, client, installation, binding)...)
			}
		}
	}
	if len(findings) == 0 {
		findings = append(findings, doctorFinding{Status: "healthy", Code: "no_degradation_detected", Message: "no tracked degradation was detected"})
	}
	return findings
}

func packageInventoryAppliesToBinding(installation domain.Installation, binding domain.ClientBinding) bool {
	if binding.PackageRevision == nil {
		return true
	}
	return binding.PackageRevision.ManifestDigest == installation.Package.ManifestDigest &&
		binding.PackageRevision.TreeDigest == installation.Source.TreeDigest
}

const blockedStateRecovery = "automatic mutation is intentionally blocked; restore state-v2.json from a trusted backup that matches the managed source, or recover and review the original source and binding metadata"

func checkManagedIntegrity(ctx context.Context, app App, client domain.DetectedClient, installation domain.Installation, binding domain.ClientBinding) []doctorFinding {
	if binding.Materialization != domain.MaterializationMaterialized || strings.TrimSpace(binding.TargetLocator) == "" {
		return nil
	}
	physicalID := strings.TrimSpace(binding.PhysicalArtifact)
	if physicalID == "" {
		return []doctorFinding{scopedFinding("degraded", "managed_target_unverifiable", installation, binding.ClientID, "the managed target has no physical artifact identity", blockedStateRecovery)}
	}
	target, err := (clientplanner.Planner{ManagedRoot: app.ManagedRoot}).ResolveTarget(ctx, client, domain.InstallScope(binding.Scope), physicalID)
	if err != nil || filepath.Clean(target.ActivePath) != filepath.Clean(binding.TargetLocator) {
		return []doctorFinding{scopedFinding("degraded", "managed_target_mismatch", installation, binding.ClientID, "the recorded managed target does not match the current safe client target", blockedStateRecovery)}
	}
	expected := ""
	sequence := 0
	for _, receipt := range binding.Receipts {
		if receipt.Sequence >= sequence && strings.TrimSpace(receipt.AfterDigest) != "" {
			sequence = receipt.Sequence
			expected = receipt.AfterDigest
		}
	}
	if expected == "" || app.Lifecycle.Stager == nil {
		return []doctorFinding{scopedFinding("unknown", "managed_integrity_not_checked", installation, binding.ClientID, "managed-directory integrity evidence is unavailable", repairAction(installation, binding))}
	}
	if err := app.Lifecycle.Stager.Verify(ctx, target.ActivePath, expected); err != nil {
		var verification *ports.VerificationError
		if errors.As(err, &verification) && verification.Kind == ports.VerificationExcludedMarker {
			return []doctorFinding{scopedFinding("unknown", "excluded_ownership_marker", installation, binding.ClientID, "the managed package contains an ownership marker excluded from digest verification", "manually review and remove the excluded ownership marker, then rerun doctor; automatic repair is intentionally blocked")}
		}
		if errors.As(err, &verification) && (verification.Kind == ports.VerificationAbsent || verification.Kind == ports.VerificationDigestMismatch) {
			return []doctorFinding{scopedFinding("degraded", "managed_directory_changed", installation, binding.ClientID, "the managed package directory is missing or differs from its recorded digest", repairAction(installation, binding))}
		}
		return []doctorFinding{scopedFinding("unknown", "managed_integrity_check_failed", installation, binding.ClientID, "managed-directory integrity could not be checked because verification infrastructure failed", "retry doctor after resolving the filesystem or temporary verification error")}
	}
	return checkNativeProjectionIntegrity(ctx, app, client, target, installation, binding, expected)
}

func checkNativeProjectionIntegrity(ctx context.Context, app App, client domain.DetectedClient, target domain.DeliveryTarget, installation domain.Installation, binding domain.ClientBinding, expectedDigest string) []doctorFinding {
	switch domain.ClientID(binding.ClientID) {
	case domain.ClientGemini, domain.ClientOpenCode, domain.ClientCline, domain.ClientWindsurf:
	default:
		return nil
	}
	if !hasOwnedNativeProjection(binding.NativeObjects) {
		return nil
	}
	if app.Lifecycle.Activator == nil || strings.TrimSpace(client.ConfigRoot) == "" {
		return []doctorFinding{scopedFinding("unknown", "native_projection_not_checked", installation, binding.ClientID, "the exact owned native client projection could not be checked", "restore client configuration visibility and rerun doctor before relying on this installation")}
	}
	// The provider activator owns the exact config codecs and skill receipt
	// verification. VerifyOnly routes through those same verifiers without
	// starting the client or mutating its configuration.
	outcome, err := app.Lifecycle.Activator.Activate(ctx, domain.ActivationRequest{
		Client: client,
		Plan: domain.DeliveryPlan{
			ClientID: domain.ClientID(binding.ClientID), Scope: domain.InstallScope(binding.Scope),
			PhysicalArtifactID: binding.PhysicalArtifact, DeclaredName: installation.DeclaredName,
			TargetRoot: target.TargetRoot, ActivePath: target.ActivePath,
			Components: []domain.ComponentDecision{{Kind: domain.ComponentSkill, Support: domain.SupportProjected}},
		},
		Delivery: domain.StagedDelivery{
			ClientID: domain.ClientID(binding.ClientID), OwnedBase: target.TargetRoot,
			ActivePath: target.ActivePath, ArtifactDigest: expectedDigest,
			NativeObjects: append([]domain.NativeObjectOwnership(nil), binding.NativeObjects...),
		},
		DeclaredName: installation.DeclaredName, Replacing: true,
		PreviousNativeObjects: append([]domain.NativeObjectOwnership(nil), binding.NativeObjects...),
		VerifyOnly:            true,
	})
	if err != nil {
		return []doctorFinding{scopedFinding("degraded", "native_projection_changed", installation, binding.ClientID, "an owned native client configuration entry or skill is missing or differs from its recorded projection", repairAction(installation, binding))}
	}
	if outcome.Activation != domain.ActivationActive || outcome.Verification != domain.VerificationInstalled {
		return []doctorFinding{scopedFinding("unknown", "native_projection_not_checked", installation, binding.ClientID, "the exact owned native client projection could not be checked", "restore client configuration visibility and rerun doctor before relying on this installation")}
	}
	return nil
}

func hasOwnedNativeProjection(objects []domain.NativeObjectOwnership) bool {
	for _, object := range objects {
		if object.Kind != "managed_package_directory" {
			return true
		}
	}
	return false
}

func scopedFinding(status, code string, installation domain.Installation, clientID, message, action string) doctorFinding {
	return doctorFinding{
		Status: status, Code: code, InstallationName: installation.DeclaredName,
		InstallationID: installation.InstallationID, ClientID: clientID,
		Message: message, RecoveryAction: action,
	}
}

func repairAction(installation domain.Installation, binding domain.ClientBinding) string {
	return fmt.Sprintf("run `agentplugins repair %s --target %s%s`", installation.InstallationID, binding.ClientID, doctorScopeFlag(binding))
}

func doctorScopeFlag(binding domain.ClientBinding) string {
	if binding.Scope == string(domain.ScopeProject) {
		return " --scope project"
	}
	return ""
}

func degradedFinding(code, subject, message, action string) doctorFinding {
	return doctorFinding{Status: "degraded", Code: code, Subject: subject, Message: message, RecoveryAction: action}
}

func newVersionCommand(app App, opts *options) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the agentplugins binary version",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := validateCommonOptions(opts); err != nil {
				return err
			}
			version := strings.TrimSpace(app.Version)
			if version == "" {
				version = "development"
			}
			if opts.format == "json" {
				return writeJSONOutput(cmd.OutOrStdout(), "version", map[string]string{"version": version})
			}
			_, err := fmt.Fprintf(cmd.OutOrStdout(), "agentplugins %s\n", version)
			return err
		},
	}
}

func publicInstallations(state domain.StateFileV2, includeAbsent bool) []publicInstallation {
	values := make([]publicInstallation, 0, len(state.Installations))
	for _, installation := range state.Installations {
		value := publicInstallationView(installation, includeAbsent)
		if len(value.Clients) == 0 && !includeAbsent {
			continue
		}
		values = append(values, value)
	}
	sort.Slice(values, func(i, j int) bool {
		if values[i].Name == values[j].Name {
			return values[i].InstallationID < values[j].InstallationID
		}
		return values[i].Name < values[j].Name
	})
	return values
}

func publicInstallationView(installation domain.Installation, includeAbsent bool) publicInstallation {
	value := publicInstallation{
		InstallationID: installation.InstallationID,
		Name:           installation.DeclaredName, Version: installation.Package.Version,
		Source: publicSource(installation.Source), NeedsRebind: installation.NeedsRebind,
	}
	if installation.Directory != nil {
		origin := installation.Directory
		value.Directory = &publicInstalledDirectory{ProductID: origin.ProductID,
			RecordedDistribution: origin.DistributionID, CurrentDistribution: origin.DistributionID,
			RecordedRevision: installation.Source.ResolvedRevision, CurrentRevision: installation.Source.ResolvedRevision,
			RecordedReleaseSequence: origin.DesiredReleaseSequence, CurrentReleaseSequence: origin.DesiredReleaseSequence,
			RecordedSnapshotSequence: origin.SnapshotSequence}
	}
	for _, client := range installation.Clients {
		if client.Materialization == domain.MaterializationAbsent && !includeAbsent {
			continue
		}
		affectedSurfaces := append([]string(nil), client.AffectedSurfaces...)
		sort.Strings(affectedSurfaces)
		value.Clients = append(value.Clients, publicClient{
			BindingID: client.ClientBindingID,
			ClientID:  client.ClientID, Scope: client.Scope, Materialization: client.Materialization,
			Activation: client.Activation, Authentication: client.Authentication,
			Policy: client.Policy, Verification: client.Verification,
			PackageRevision:  publicPackageRevision(client.PackageRevision),
			AffectedSurfaces: affectedSurfaces,
		})
	}
	sort.Slice(value.Clients, func(i, j int) bool { return value.Clients[i].ClientID < value.Clients[j].ClientID })
	value.MixedVersion, value.ConvergenceAction = convergenceState(installation)
	return value
}

func publicPackageRevision(revision *domain.ClientPackageRevision) *publicClientRevision {
	if revision == nil {
		return nil
	}
	return &publicClientRevision{Version: revision.Version, ResolvedRevision: publicImmutableRevision(revision.ResolvedRevision),
		DistributionID: revision.DistributionID, ReleaseSequence: revision.ReleaseSequence,
		TreeDigest: revision.TreeDigest, ManifestDigest: revision.ManifestDigest}
}

func publicImmutableRevision(value string) string {
	value = strings.TrimSpace(value)
	if len(value) != 40 {
		return ""
	}
	for _, character := range value {
		if !((character >= '0' && character <= '9') || (character >= 'a' && character <= 'f')) {
			return ""
		}
	}
	return value
}

func publicSource(source domain.SourceBinding) string {
	if source.Repository != "" {
		value := source.Repository
		if source.PackageSubpath != "" {
			value += "//" + source.PackageSubpath
		}
		return value
	}
	canonical := strings.TrimSpace(source.CanonicalSource)
	if canonical == "" || filepath.IsAbs(canonical) {
		return "local"
	}
	parsed, err := url.Parse(canonical)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "local"
	}
	parsed.User = nil
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String()
}

func selectInstallation(state domain.StateFileV2, selector string) (domain.Installation, error) {
	selector = strings.TrimSpace(selector)
	var matches []domain.Installation
	for _, installation := range state.Installations {
		if installation.InstallationID == selector {
			return installation, nil
		}
		if installationMatchesSelector(installation, selector) {
			matches = append(matches, installation)
		}
	}
	if len(matches) == 0 {
		return domain.Installation{}, fmt.Errorf("installation %q was not found", selector)
	}
	if len(matches) > 1 {
		return domain.Installation{}, fmt.Errorf("installation name %q is ambiguous; use installation_id", selector)
	}
	return matches[0], nil
}

func renderInstallationList(writer io.Writer, installations []publicInstallation) error {
	if len(installations) == 0 {
		_, _ = fmt.Fprintln(writer, "No Agent Plugins installations are tracked.")
		return nil
	}
	for _, installation := range installations {
		if err := renderInstallation(writer, installation); err != nil {
			return err
		}
	}
	return nil
}

func renderInstallation(writer io.Writer, installation publicInstallation) error {
	_, _ = fmt.Fprintf(writer, "%s %s (%s)\n", installation.Name, installation.Version, installation.InstallationID)
	_, _ = fmt.Fprintf(writer, "  Source: %s\n", installation.Source)
	for _, client := range installation.Clients {
		_, _ = fmt.Fprintf(writer, "  %s: materialization=%s activation=%s auth=%s verification=%s\n",
			client.ClientID, client.Materialization, client.Activation, client.Authentication, client.Verification)
		if client.ReceiptReconciled != nil && client.NativeDiscoveryReconciled != nil {
			_, _ = fmt.Fprintf(writer, "    native_identity=%s receipt_reconciled=%t native_discovery_reconciled=%t client_version=%s\n",
				client.NativeIdentityState, *client.ReceiptReconciled, *client.NativeDiscoveryReconciled, client.ClientVersion)
		}
	}
	if installation.Directory != nil {
		value := installation.Directory
		_, _ = fmt.Fprintf(writer, "  Directory: recorded=%s@%s release=%d current=%s@%s release=%d\n",
			value.RecordedDistribution, value.RecordedRevision, value.RecordedReleaseSequence,
			value.CurrentDistribution, value.CurrentRevision, value.CurrentReleaseSequence)
	}
	if installation.MixedVersion {
		_, _ = fmt.Fprintf(writer, "  Mixed version: true\n  Convergence: %s\n", installation.ConvergenceAction)
	}
	for _, warning := range installation.Warnings {
		_, _ = fmt.Fprintf(writer, "  WARNING [%s]: %s\n", warning.Code, warning.Message)
		if warning.Action != "" {
			_, _ = fmt.Fprintf(writer, "    Action: %s\n", warning.Action)
		}
	}
	return nil
}
