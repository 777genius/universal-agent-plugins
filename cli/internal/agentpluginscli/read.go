package agentpluginscli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/prompt"
	"github.com/777genius/plugin-kit-ai/cli/internal/terminaltheme"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/dirswap"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	clientplanner "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/planner"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
	"github.com/spf13/cobra"
)

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
		ReadOnly           bool                     `json:"read_only"`
		ToolVersion        string                   `json:"tool_version"`
		SupportedClients   []supportedClient        `json:"supported_clients"`
		Clients            []publicDetectedClient   `json:"clients"`
		InstallationCount  int                      `json:"installation_count"`
		OpenOperationCount int                      `json:"open_operation_count"`
		Installation       *publicInstallation      `json:"installation,omitempty"`
		ProjectReadiness   []projectReadinessRecord `json:"project_readiness"`
		Findings           []doctorFinding          `json:"findings"`
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
	report.ProjectReadiness = projectReadinessRecords(state, selected)
	report.Findings = doctorFindings(ctx, app, clients, state, open, selected)
	report.Findings = append(report.Findings, doctorCodexNativeFindings(state, selected)...)
	safety, err := doctorDirectorySafetyFindings(ctx, app, state, selected, report.Installation)
	if err != nil {
		return fmt.Errorf("inspect signed Directory: %w", err)
	}
	report.Findings = append(report.Findings, safety...)
	report.Findings = dropDoctorHealthySentinel(report.Findings)
	sortDoctorFindings(report.Findings)
	if opts.format == "json" {
		return writeJSONOutput(cmd.OutOrStdout(), "doctor", report)
	}
	_, _ = fmt.Fprintln(cmd.OutOrStdout(), "agentplugins doctor (read-only)")
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s: %s\n", terminaltheme.For(cmd.OutOrStdout()).Text(terminaltheme.Label, "Tool version"), report.ToolVersion)
	for _, client := range clients {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  %s: %s\n", client.DisplayName, client.Status)
	}
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s: %d\n", terminaltheme.For(cmd.OutOrStdout()).Text(terminaltheme.Label, "Tracked installations"), report.InstallationCount)
	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s: %d\n", terminaltheme.For(cmd.OutOrStdout()).Text(terminaltheme.Label, "Open recovery operations"), report.OpenOperationCount)
	renderProjectReadiness(cmd.OutOrStdout(), report.ProjectReadiness)
	for _, finding := range report.Findings {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  [%s] %s: %s\n", finding.Status, finding.Code, finding.Message)
		if finding.InstallationID != "" {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "    %s: %s (%s)\n", terminaltheme.For(cmd.OutOrStdout()).Text(terminaltheme.Label, "Installation"), finding.InstallationName, finding.InstallationID)
		}
		if finding.ClientID != "" {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "    %s: %s\n", terminaltheme.For(cmd.OutOrStdout()).Text(terminaltheme.Label, "Client"), finding.ClientID)
		}
		if finding.RecoveryAction != "" {
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "    %s: %s\n", terminaltheme.For(cmd.OutOrStdout()).Text(terminaltheme.Label, "Recovery"), finding.RecoveryAction)
		}
	}
	if report.Installation != nil {
		return renderInstallation(cmd.OutOrStdout(), *report.Installation)
	}
	return nil
}

// renderProjectReadiness is the only human-output boundary for project records.
// State-derived identifiers and guidance must never inject terminal control
// sequences; JSON output retains its usual escaping and structured values.
func renderProjectReadiness(out io.Writer, records []projectReadinessRecord) {
	if len(records) == 0 {
		return
	}
	_, _ = fmt.Fprintln(out, "Project readiness (recorded facts, not live client execution):")
	for _, item := range records {
		_, _ = fmt.Fprintf(out, "  %s/%s: %s (native: %s)\n",
			prompt.SafeText(item.InstallationID), prompt.SafeText(item.ClientID),
			prompt.SafeText(item.Status), prompt.SafeText(item.NativeEvidence))
		for _, action := range item.NextActions {
			_, _ = fmt.Fprintln(out, "    Next:", prompt.SafeText(action))
		}
	}
}

// A generic "healthy" sentinel is valid only when no more-specific observer
// found any issue, including native project drift or Directory safety findings.
func dropDoctorHealthySentinel(findings []doctorFinding) []doctorFinding {
	if len(findings) <= 1 {
		return findings
	}
	kept := findings[:0]
	for _, finding := range findings {
		if finding.Code != "no_degradation_detected" {
			kept = append(kept, finding)
		}
	}
	return kept
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
		if client.DetectionError != nil {
			findings = append(findings, doctorFinding{Status: "unknown", Code: "client_detection_failed", ClientID: string(client.ClientID), Message: "client configuration could not be inspected safely", RecoveryAction: "correct the client profile configuration and rerun doctor"})
		}
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
			bindingClient := domain.ClientID(binding.ClientID)
			if plansWithoutHostPresence(bindingClient) && !visible {
				client = syntheticUndetectedClient(bindingClient)
			}
			if !plansWithoutHostPresence(bindingClient) && (!visible || (client.Status != domain.DetectionDetected && client.DetectionError == nil)) {
				findings = append(findings, scopedFinding("degraded", "client_not_visible", installation, binding.ClientID, "the package is tracked but no current client visibility evidence was detected", "install or launch the client so its CLI, desktop application, or configuration directory is visible, then rerun doctor"))
			}
			if requiresPersonalMapping(bindingClient) {
				inventoryCurrent := packageInventoryAppliesToBinding(installation, binding)
				if inventoryCurrent && installation.Package.Inventory.MCPPresent && (!installation.Package.Inventory.AppPresent || len(installation.Package.Inventory.AppBindings) == 0) {
					findings = append(findings, scopedFinding("degraded", "chatgpt_app_binding_missing", installation, binding.ClientID, "the ChatGPT target has MCP content but no valid registered app mapping", "register the MCP connection in ChatGPT Developer Mode, add a valid root .app.json mapping, then update this target"))
				} else if !inventoryCurrent {
					findings = append(findings, scopedFinding("unknown", "chatgpt_registration_unverified", installation, binding.ClientID, "remote ChatGPT registration for this earlier client revision cannot be inferred from the latest package inventory", "verify the installed plugin revision and connection status in ChatGPT Plugins; update this target before relying on current package metadata"))
				} else if installation.Package.Inventory.AppPresent {
					findings = append(findings, scopedFinding("unknown", "chatgpt_registration_unverified", installation, binding.ClientID, "the .app.json mapping is package-valid, but remote ChatGPT registration cannot be observed locally", "verify the mapped connection and plugin status in ChatGPT Plugins; rerun add with --activation-complete only after checking it in a new chat"))
				}
			}
			if visible && client.Status == domain.DetectionDetected && bindingClient == nativeCLIRegistryOwner(bindingClient, domain.InstallScope(binding.Scope)) && sharesPhysicalBackend(bindingClient, domain.InstallScope(binding.Scope)) && strings.TrimSpace(client.ExecutablePath) == "" {
				findings = append(findings, scopedFinding("degraded", "copilot_cli_missing", installation, binding.ClientID, "GitHub Copilot CLI is unavailable for automatic Copilot activation", "install GitHub Copilot CLI, ensure copilot is on PATH, and rerun doctor"))
			}
			if binding.Scope == string(domain.ScopeProject) {
				switch binding.Policy {
				case domain.PolicyBlocked:
					findings = append(findings, scopedFinding("degraded", "project_policy_blocked", installation, binding.ClientID, "native policy blocks this project binding", projectReadinessActions(binding.ClientID, "policy_blocked")[0]))
				case domain.PolicyApprovalRequired:
					findings = append(findings, scopedFinding("unknown", "project_policy_approval_required", installation, binding.ClientID, "native policy requires explicit approval", projectReadinessActions(binding.ClientID, "policy_approval_required")[0]))
				}
			}
			switch binding.Activation {
			case domain.ActivationPrepared:
				if binding.Scope == string(domain.ScopeProject) {
					findings = append(findings, projectPreparedFinding(installation, binding))
				} else {
					findings = append(findings, scopedFinding("degraded", "activation_pending", installation, binding.ClientID, "client activation is not complete", fmt.Sprintf("complete the displayed external activation step, then rerun the same `agentplugins add ... --target %s%s` command", publicCLITargetName(domain.ClientID(binding.ClientID)), doctorScopeFlag(binding))))
				}
			case domain.ActivationManual:
				findings = append(findings, scopedFinding("degraded", "activation_pending", installation, binding.ClientID, "client activation is not complete", fmt.Sprintf("complete the displayed external activation step, then rerun the same `agentplugins add ... --target %s%s` command", publicCLITargetName(domain.ClientID(binding.ClientID)), doctorScopeFlag(binding))))
			case domain.ActivationFailed:
				findings = append(findings, scopedFinding("degraded", "activation_failed", installation, binding.ClientID, "client activation failed", fmt.Sprintf("resolve the client-reported activation error, then rerun the same `agentplugins add ... --target %s%s` command", publicCLITargetName(domain.ClientID(binding.ClientID)), doctorScopeFlag(binding))))
			}
			switch binding.Authentication {
			case domain.AuthenticationPending:
				findings = append(findings, scopedFinding("degraded", "authentication_pending", installation, binding.ClientID, "plugin authentication is pending", fmt.Sprintf("complete the displayed external authentication step, then rerun the same `agentplugins add ... --target %s%s` command", publicCLITargetName(domain.ClientID(binding.ClientID)), doctorScopeFlag(binding))))
			case domain.AuthenticationFailed:
				findings = append(findings, scopedFinding("degraded", "authentication_failed", installation, binding.ClientID, "plugin authentication failed", fmt.Sprintf("reauthorize the plugin in the selected client, then rerun the same `agentplugins add ... --target %s%s` command", publicCLITargetName(domain.ClientID(binding.ClientID)), doctorScopeFlag(binding))))
			case domain.AuthenticationNotChecked:
				findings = append(findings, scopedFinding("unknown", "authentication_not_checked", installation, binding.ClientID, "authentication requirements have not been verified", "check the package's authentication instructions and verify access in the selected client"))
			}
			if binding.Materialization == domain.MaterializationDegraded || binding.Verification == domain.VerificationFailed {
				findings = append(findings, scopedFinding("degraded", "installation_verification_failed", installation, binding.ClientID, "the managed package is marked degraded or failed verification", repairAction(installation, binding)))
			}
			if plansWithoutHostPresence(bindingClient) || (visible && client.Status == domain.DetectionDetected) {
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
	if app.Targets == nil {
		return []doctorFinding{scopedFinding("degraded", "managed_target_unverifiable", installation, binding.ClientID, "the managed target resolver is not configured", blockedStateRecovery)}
	}
	target, err := app.Targets.ResolveTarget(ctx, client, domain.InstallScope(binding.Scope), physicalID)
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
	if !nativeConfigLifecycle(domain.ClientID(binding.ClientID), domain.InstallScope(binding.Scope)) {
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
			ClientID: domain.ClientID(binding.ClientID), Scope: domain.InstallScope(binding.Scope), TargetContext: target.TargetContext, NativeRegistryRoot: target.NativeRegistryRoot,
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
		var verification *ports.VerificationError
		if errors.As(err, &verification) && verification.Kind == ports.VerificationIndeterminate {
			return []doctorFinding{scopedFinding("unknown", "native_projection_not_checked", installation, binding.ClientID, "the exact owned native client projection could not be checked", "restore client configuration visibility and rerun doctor before relying on this installation")}
		}
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
	return fmt.Sprintf("run `agentplugins repair %s --target %s%s`", installation.InstallationID, publicCLITargetName(domain.ClientID(binding.ClientID)), doctorScopeFlag(binding))
}

func doctorScopeFlag(binding domain.ClientBinding) string {
	if binding.Scope == string(domain.ScopeProject) {
		return " --scope project --project-root \"<PROJECT_ROOT>\""
	}
	return ""
}

func degradedFinding(code, subject, message, action string) doctorFinding {
	return doctorFinding{Status: "degraded", Code: code, Subject: subject, Message: message, RecoveryAction: action}
}
