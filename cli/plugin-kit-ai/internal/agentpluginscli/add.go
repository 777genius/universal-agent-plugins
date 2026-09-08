package agentpluginscli

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
	"github.com/spf13/cobra"
)

func newAddCommand(app App, opts *options) *cobra.Command {
	var activationComplete, authComplete bool
	command := &cobra.Command{
		Use:     "add <name-or-source>",
		Aliases: []string{"install"},
		Short:   "Plan and install one Agent Plugins 1.0 package for one or more clients",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateCommonOptions(opts); err != nil {
				return err
			}
			var detectedClients []domain.DetectedClient
			if strings.TrimSpace(opts.target) == "" && app.Terminal {
				selection, clients, preloaded, err := promptCompatibleDetectedTargets(cmd.Context(), cmd, app, args[0])
				if err != nil {
					return err
				}
				if preloaded != nil && preloaded.cleanup != nil {
					defer preloaded.cleanup()
				}
				detectedClients = clients
				opts.target = selection
				defer func() { opts.target = "" }()
				targets, err := parseTargetOption(opts.target)
				if err != nil {
					return err
				}
				detectedClients, err = detectSelectedTargetsForLifecycleResolution(cmd.Context(), app.Detector, targets, detectedClients, !opts.dryRun && isDirectorySelector(args[0]))
				if err != nil {
					return fmt.Errorf("detect selected AI clients: %w", err)
				}
				_, detected, err := preflightSelectedTargets(cmd.Context(), app, targets, detectedClients, false)
				if err != nil {
					return err
				}
				var loaded loadedPackage
				if preloaded != nil {
					loaded = *preloaded
				} else {
					writeProgress(app, opts.format, "Resolving and validating Agent Plugin...")
					loaded, err = app.loadPackageFor(cmd.Context(), args[0], withDetectedClients(app.addResolutionRequest(args[0], targets), detected))
					if err != nil {
						return err
					}
					if loaded.cleanup != nil {
						defer loaded.cleanup()
					}
				}
				return runAddManyLoaded(cmd.Context(), cmd, app, opts, loaded, targets, activationComplete, authComplete, detectedClientValues(detected))
			}
			targets, err := parseTargetOption(opts.target)
			if err != nil {
				return err
			}
			if len(targets) == 0 {
				return fmt.Errorf("automated installation requires --target")
			}
			return runAddManyWithClients(cmd.Context(), cmd, app, opts, args[0], targets, activationComplete, authComplete, detectedClients)
		},
	}
	command.Flags().BoolVar(&activationComplete, "activation-complete", false, "attest that manual client activation is complete")
	command.Flags().BoolVar(&authComplete, "auth-complete", false, "attest that required authentication is complete or none is required after review")
	return command
}

func runAdd(ctx context.Context, cmd *cobra.Command, app App, opts *options, source string, activationComplete, authComplete bool) error {
	return runAddWithClients(ctx, cmd, app, opts, source, activationComplete, authComplete, nil)
}

func runAddWithClients(ctx context.Context, cmd *cobra.Command, app App, opts *options, source string, activationComplete, authComplete bool, clients []domain.DetectedClient) error {
	targets, err := parseTargetOption(opts.target)
	if err != nil {
		return err
	}
	_, detected, err := preflightSelectedTargets(ctx, app, targets, clients, !opts.dryRun && isDirectorySelector(source))
	if err != nil {
		return err
	}
	writeProgress(app, opts.format, "Resolving and validating Agent Plugin...")
	loaded, err := app.loadPackageFor(ctx, source, withDetectedClients(app.addResolutionRequest(source, targets), detected))
	if err != nil {
		return err
	}
	if loaded.cleanup != nil {
		defer loaded.cleanup()
	}
	return runAddLoaded(ctx, cmd, app, opts, loaded, activationComplete, authComplete, detectedClientValues(detected))
}

func runAddLoaded(ctx context.Context, cmd *cobra.Command, app App, opts *options, loaded loadedPackage, activationComplete, authComplete bool, clients []domain.DetectedClient) error {
	if err := authorizeSecurityAssessment(cmd, app, opts, &loaded); err != nil {
		return err
	}
	if automatedMutation(app, opts) && strings.TrimSpace(opts.target) == "" {
		return fmt.Errorf("automated installation requires --target")
	}
	selected, detectedMap, err := selectClient(cmd, app, opts, clients)
	if err != nil {
		return err
	}
	if err := prepareLoadedPackageForClient(&loaded, selected.ClientID); err != nil {
		return err
	}
	service := lifecycleService(app, detectedMap)
	input := usecase.AddInput{
		Envelope: loaded.envelope, Client: selected, Scope: domain.InstallScope(opts.scope),
		DryRun: opts.dryRun, Confirmed: false, Interactive: app.Terminal,
		Hints: loaded.hints, BackendExecutable: backendExecutable(selected, detectedMap),
		ActivationComplete: activationComplete, AuthComplete: authComplete,
		PersistAuthoritativeObservations: true,
		OriginMode:                       loaded.origin, DirectoryResolution: cloneDirectoryOrigin(loaded.directory),
		DistributionSuspended: loaded.distributionSuspended, ReleaseRevoked: loaded.releaseRevoked,
	}
	planned, err := service.Add(ctx, input)
	if err != nil {
		if planned.Plan.Status == domain.PlanUnsupported {
			if renderErr := renderAddResultErrorWithSecurity(cmd.OutOrStdout(), opts.format, loaded.envelope, loaded.security, planned, opts.dryRun, err); renderErr != nil {
				return renderErr
			}
		}
		return err
	}
	if opts.dryRun || planned.NoChange {
		return renderAddResultWithSecurity(cmd.OutOrStdout(), opts.format, loaded.envelope, loaded.security, planned, opts.dryRun)
	}
	if opts.format == "human" {
		if err := renderHumanPlan(cmd.OutOrStdout(), loaded.envelope, planned); err != nil {
			return err
		}
	}
	freshInstall := planned.Activation.Activation == ""
	if !freshInstall && opts.format == "human" && app.Terminal && !activationComplete && !authComplete {
		if err := renderAddResultWithSecurity(cmd.OutOrStdout(), opts.format, loaded.envelope, loaded.security, planned, false); err != nil {
			return err
		}
		return resumeInteractiveLifecycle(ctx, cmd, service, input, loaded.envelope, planned)
	}
	confirmed := mutationConfirmed(app, opts)
	if !confirmed && opts.format == "human" && app.Terminal {
		prompt := "Apply this plan? [y/N]"
		if !freshInstall {
			prompt = "Apply these explicit lifecycle attestations? [y/N]"
		}
		confirmed, err = promptYesNo(cmd.InOrStdin(), cmd.OutOrStdout(), prompt)
		if err != nil {
			return err
		}
	}
	if !confirmed {
		if opts.format == "json" {
			return renderAddResultWithSecurity(cmd.OutOrStdout(), opts.format, loaded.envelope, loaded.security, planned, false)
		}
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No changes made.")
		return nil
	}
	writeProgress(app, opts.format, "Applying transactional client package...")
	input.Confirmed = true
	input.InstallationID = planned.InstallationID
	result, err := service.Add(ctx, input)
	if renderErr := renderAddResultErrorWithSecurity(cmd.OutOrStdout(), opts.format, loaded.envelope, loaded.security, result, false, err); renderErr != nil && err == nil {
		err = renderErr
	}
	if err == nil && freshInstall && opts.format == "human" && app.Terminal {
		return resumeInteractiveLifecycle(ctx, cmd, service, input, loaded.envelope, result)
	}
	return err
}

func promptTargetChoices(cmd *cobra.Command, detected, skipped, allClients []domain.DetectedClient) (string, []domain.DetectedClient, error) {
	if len(detected) == 0 {
		return "", nil, fmt.Errorf("no supported local AI client was detected; use --target chatgpt for ChatGPT, or install/detect another client")
	}
	sort.SliceStable(detected, func(i, j int) bool { return targetOrder(detected[i].ClientID) < targetOrder(detected[j].ClientID) })
	sort.SliceStable(skipped, func(i, j int) bool { return targetOrder(skipped[i].ClientID) < targetOrder(skipped[j].ClientID) })
	if len(skipped) > 0 {
		names := make([]string, len(skipped))
		for index, client := range skipped {
			names[index] = client.DisplayName
		}
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Skipped installed clients that this package cannot install together: %s\n", strings.Join(names, ", "))
	}
	if len(detected) == 1 {
		return string(detected[0].ClientID), allClients, nil
	}
	_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Detected supported clients (all selected by default):")
	for index, client := range detected {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  %d. [x] %s (%s)\n", index+1, client.DisplayName, client.ClientID)
	}
	_, _ = fmt.Fprint(cmd.OutOrStdout(), "Choose targets by number, comma-separated [all]: ")
	line, readErr := readInputLine(cmd.InOrStdin())
	if readErr != nil && readErr != io.EOF {
		return "", nil, readErr
	}
	line = strings.TrimSpace(line)
	if line == "" {
		values := make([]string, len(detected))
		for index, client := range detected {
			values[index] = string(client.ClientID)
		}
		return strings.Join(values, ","), allClients, nil
	}
	seen := make(map[int]struct{})
	var values []string
	for _, raw := range strings.Split(line, ",") {
		choice, err := strconv.Atoi(strings.TrimSpace(raw))
		if err != nil || choice < 1 || choice > len(detected) {
			return "", nil, fmt.Errorf("invalid client multiselect")
		}
		if _, duplicate := seen[choice]; duplicate {
			return "", nil, fmt.Errorf("duplicate client multiselect choice %d", choice)
		}
		seen[choice] = struct{}{}
		values = append(values, string(detected[choice-1].ClientID))
	}
	return strings.Join(values, ","), allClients, nil
}

// detectSelectedTargetsForLifecycleResolution keeps ambient discovery strictly
// read-only. Version execution is an opt-in capability invoked only after the
// user has selected the complete target set. Detectors without targeted probing
// retain the read-only observations rather than falling back to executing every
// discovered client binary.
func detectSelectedTargetsForLifecycleResolution(ctx context.Context, detector ports.ClientDetector, targets []domain.ClientID, detected []domain.DetectedClient, probeVersion bool) ([]domain.DetectedClient, error) {
	if !probeVersion {
		return detected, nil
	}
	if targeted, ok := detector.(ports.TargetedVersionProbingClientDetector); ok {
		return targeted.DetectTargetsWithVersionProbe(ctx, targets)
	}
	return detected, nil
}

func resumeInteractiveLifecycle(
	ctx context.Context,
	cmd *cobra.Command,
	service usecase.Service,
	input usecase.AddInput,
	envelope domain.PackageEnvelope,
	current usecase.AddResult,
) error {
	if current.Activation.Activation != domain.ActivationActive || current.Activation.Verification != domain.VerificationInstalled {
		complete, err := promptYesNo(cmd.InOrStdin(), cmd.OutOrStdout(), "Have you completed activation and verified the plugin is enabled in the client? [y/N]")
		if err != nil {
			return err
		}
		if !complete {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Activation remains unconfirmed. Rerun add when it is complete.")
			return nil
		}
		resume := input
		resume.Confirmed = true
		resume.InstallationID = current.InstallationID
		resume.ActivationComplete = true
		resume.AuthComplete = false
		current, err = service.Add(ctx, resume)
		if err != nil {
			if renderErr := renderAddResultError(cmd.OutOrStdout(), "human", envelope, current, false, err); renderErr != nil {
				return renderErr
			}
			return err
		}
	}

	if current.Activation.Authentication == domain.AuthenticationPending || current.Activation.Authentication == domain.AuthenticationNotChecked {
		complete, err := promptYesNo(cmd.InOrStdin(), cmd.OutOrStdout(), "Have you completed required authentication, or reviewed the package and confirmed none is required? [y/N]")
		if err != nil {
			return err
		}
		if !complete {
			if current.Activation.ActivationAttested {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Activation is user-attested. Authentication remains unconfirmed; rerun add after completing or reviewing it.")
			} else {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Authentication remains unconfirmed. Rerun add after completing or reviewing it.")
			}
			return nil
		}
		resume := input
		resume.Confirmed = true
		resume.InstallationID = current.InstallationID
		resume.ActivationComplete = false
		resume.AuthComplete = true
		current, err = service.Add(ctx, resume)
		if err != nil {
			if renderErr := renderAddResultError(cmd.OutOrStdout(), "human", envelope, current, false, err); renderErr != nil {
				return renderErr
			}
			return err
		}
	}

	return renderAddResult(cmd.OutOrStdout(), "human", envelope, current, false)
}

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
		if !ok && target == domain.ClientChatGPT {
			client = domain.DetectedClient{ClientID: domain.ClientChatGPT, DisplayName: "ChatGPT", Status: domain.DetectionNotDetected}
			detectedMap[target] = client
			ok = true
		}
		if !ok || (client.Status != domain.DetectionDetected && target != domain.ClientChatGPT) {
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
	line, err := readInputLine(cmd.InOrStdin())
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
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "codex":
		return domain.ClientCodex
	case "chatgpt":
		return domain.ClientChatGPT
	case "cursor":
		return domain.ClientCursor
	case "copilot", "github-copilot":
		return domain.ClientCopilot
	case "vscode", "vs-code":
		return domain.ClientVSCode
	case "kiro":
		return domain.ClientKiro
	case "claude", "claude-code":
		return domain.ClientClaude
	case "gemini", "gemini-cli":
		return domain.ClientGemini
	case "opencode", "open-code":
		return domain.ClientOpenCode
	case "cline":
		return domain.ClientCline
	case "windsurf", "devin":
		return domain.ClientWindsurf
	default:
		return domain.ClientID(strings.ToLower(strings.TrimSpace(value)))
	}
}

func backendExecutable(selected domain.DetectedClient, clients map[domain.ClientID]domain.DetectedClient) string {
	if selected.ClientID == domain.ClientVSCode {
		return clients[domain.ClientCopilot].ExecutablePath
	}
	return selected.ExecutablePath
}

// detectedSharedClient resolves the VS Code logical surface through an already
// detected Copilot backend. It deliberately does not invent a backend when
// neither shared surface is present.
func detectedSharedClient(target domain.ClientID, clients map[domain.ClientID]domain.DetectedClient) (domain.DetectedClient, bool) {
	client, exact := clients[target]
	if exact && (target != domain.ClientVSCode || client.Status == domain.DetectionDetected) {
		return client, true
	}
	if target != domain.ClientVSCode {
		return client, exact
	}
	copilot, ok := clients[domain.ClientCopilot]
	if !ok || copilot.Status != domain.DetectionDetected {
		return client, exact
	}
	client = copilot
	client.ClientID = domain.ClientVSCode
	client.DisplayName = "VS Code (shared GitHub Copilot backend)"
	client.ExecutablePath = ""
	client.ConfigRoot = ""
	clients[target] = client
	return client, true
}

func promptYesNo(reader io.Reader, writer io.Writer, prompt string) (bool, error) {
	if _, err := fmt.Fprint(writer, prompt+" "); err != nil {
		return false, err
	}
	line, err := readInputLine(reader)
	if err != nil && err != io.EOF {
		return false, err
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == "yes", nil
}

func readInputLine(reader io.Reader) (string, error) {
	var line strings.Builder
	var buffer [1]byte
	for {
		read, err := reader.Read(buffer[:])
		if read > 0 {
			if buffer[0] == '\n' {
				return line.String(), nil
			}
			line.WriteByte(buffer[0])
		}
		if err != nil {
			return line.String(), err
		}
	}
}

func renderHumanPlan(writer io.Writer, envelope domain.PackageEnvelope, result usecase.AddResult) error {
	result = withOpenCodeRuntimeNotice(result)
	_, _ = fmt.Fprintf(writer, "Plugin: %s %s\n", envelope.Manifest.Name, envelope.Manifest.Version)
	_, _ = fmt.Fprintf(writer, "Target: %s\n", result.Plan.ClientID)
	_, _ = fmt.Fprintf(writer, "Package: %s\n", result.Plan.PackageMode)
	_, _ = fmt.Fprintf(writer, "Result: %s\n", result.Plan.Status)
	_, _ = fmt.Fprintf(writer, "Authentication: %s\n", result.Plan.Authentication)
	_, _ = fmt.Fprintf(writer, "Verification: %s\n", result.Plan.Verification)
	for _, component := range result.Plan.Components {
		_, _ = fmt.Fprintf(writer, "  - %s %s: %s\n", component.Kind, component.Name, component.Support)
	}
	for _, diagnostic := range result.Plan.Diagnostics {
		_, _ = fmt.Fprintf(writer, "  Warning: %s: %s\n", diagnostic.Code, diagnostic.Message)
	}
	for _, warning := range result.Plan.Warnings {
		_, _ = fmt.Fprintf(writer, "  Warning: %s\n", warning)
	}
	for _, action := range result.Plan.UserActions {
		_, _ = fmt.Fprintf(writer, "  Planned action: %s\n", action)
	}
	for _, action := range result.Plan.LocalActions {
		_, _ = fmt.Fprintf(writer, "  Planned action: %s\n", action)
	}
	return nil
}

func renderAddResult(writer io.Writer, format string, envelope domain.PackageEnvelope, result usecase.AddResult, dryRun bool) error {
	return renderAddResultError(writer, format, envelope, result, dryRun, nil)
}

func renderAddResultWithSecurity(writer io.Writer, format string, envelope domain.PackageEnvelope, security *domain.SecurityAssessment, result usecase.AddResult, dryRun bool) error {
	return renderAddResultErrorWithSecurity(writer, format, envelope, security, result, dryRun, nil)
}

func renderAddResultError(writer io.Writer, format string, envelope domain.PackageEnvelope, result usecase.AddResult, dryRun bool, commandErr error) error {
	return renderAddResultErrorWithSecurity(writer, format, envelope, nil, result, dryRun, commandErr)
}

func renderAddResultErrorWithSecurity(writer io.Writer, format string, envelope domain.PackageEnvelope, security *domain.SecurityAssessment, result usecase.AddResult, dryRun bool, commandErr error) error {
	data := newAddResultData(envelope, result, dryRun)
	data.Security = security
	data.envelopeResult = addEnvelopeResult(result, commandErr)
	if format == "json" {
		return writeJSONOutput(writer, "add", data)
	}
	if failure := addFailureStatus(result, commandErr); failure != "" {
		if result.Plan.Status == domain.PlanUnsupported {
			if _, err := fmt.Fprintf(writer, "Add: %s\n", failure); err != nil {
				return err
			}
			return renderHumanPlan(writer, envelope, result)
		}
		_, err := fmt.Fprintf(writer, "Add: %s\n", failure)
		return err
	}
	if dryRun {
		return renderHumanPlan(writer, envelope, result)
	}
	if err := renderOpenCodeRuntimeNotice(writer, result); err != nil {
		return err
	}
	if result.NoChange {
		_, _ = fmt.Fprintln(writer, "Already installed and lifecycle verification is complete. No changes made.")
		return nil
	}
	if result.Mutated && fullyInstalled(result.Activation) {
		if result.Activation.ActivationAttested || result.Activation.AuthenticationAttested {
			_, _ = fmt.Fprintln(writer, "Lifecycle is user-attested for the explicitly confirmed phase; it was not observed from the client.")
		} else {
			if result.Plan.ClientID == domain.ClientOpenCode && len(domain.SelectedMCPNames(result.Plan)) > 0 {
				_, _ = fmt.Fprintln(writer, "OpenCode MCP configuration installed and verified.")
			} else {
				_, _ = fmt.Fprintln(writer, "Installed and verified for the selected client.")
			}
		}
		return nil
	}
	if result.Mutated {
		if result.Activation.Authentication == domain.AuthenticationPending {
			if result.Activation.Activation == domain.ActivationActive {
				_, _ = fmt.Fprintln(writer, "Package materialized and client activation completed. Authentication is pending.")
			} else {
				_, _ = fmt.Fprintln(writer, "Package prepared. Authentication and client activation are pending.")
			}
		} else if result.Activation.Authentication == domain.AuthenticationNotChecked && result.Activation.Activation == domain.ActivationActive {
			_, _ = fmt.Fprintln(writer, "Package materialized and client activation verified. Authentication requirements have not been checked.")
		} else {
			_, _ = fmt.Fprintln(writer, "Package prepared. Activation is not complete yet.")
		}
	}
	if action := nextLocalLifecycleAction(result); action != "" && !fullyInstalled(result.Activation) {
		_, _ = fmt.Fprintf(writer, "Next: %s\n", action)
	}
	return nil
}

type addResultData struct {
	OperationID    string                     `json:"operation_id,omitempty"`
	Plugin         string                     `json:"plugin"`
	Version        string                     `json:"version,omitempty"`
	Source         string                     `json:"source"`
	Revision       string                     `json:"revision,omitempty"`
	TreeDigest     string                     `json:"tree_digest"`
	ManifestDigest string                     `json:"manifest_digest"`
	Security       *domain.SecurityAssessment `json:"security,omitempty"`
	NextAction     string                     `json:"next_action,omitempty"`
	DryRun         bool                       `json:"dry_run"`
	Result         usecase.AddResult          `json:"result"`
	envelopeResult string
}

func (data addResultData) outputResult() string {
	return data.envelopeResult
}

func addEnvelopeResult(result usecase.AddResult, commandErr error) string {
	if addFailureStatus(result, commandErr) != "" {
		return outputResultFailure
	}
	return outputResultSuccess
}

func addFailureStatus(result usecase.AddResult, commandErr error) string {
	if result.GroupPhase == usecase.GroupTargetManagedRolledBack ||
		result.GroupPhase == usecase.GroupTargetManagedUnknown ||
		result.GroupPhase == usecase.GroupTargetExternalFailed ||
		result.GroupPhase == usecase.GroupTargetExternalPartial {
		return string(result.GroupPhase)
	}
	if result.Plan.Status == domain.PlanUnsupported {
		return string(domain.PlanUnsupported)
	}
	if result.Activation.Activation == domain.ActivationFailed {
		return "activation_failed"
	}
	if result.Activation.Authentication == domain.AuthenticationFailed {
		return "authentication_failed"
	}
	if result.Activation.Verification == domain.VerificationFailed {
		return "verification_failed"
	}
	if commandErr != nil {
		return "failed"
	}
	return ""
}

func newAddResultData(envelope domain.PackageEnvelope, result usecase.AddResult, dryRun bool) addResultData {
	result = withOpenCodeRuntimeNotice(result)
	return addResultData{
		OperationID: result.Receipt.OperationID, Plugin: envelope.Manifest.Name,
		Version: envelope.Manifest.Version, Source: publicPackageSource(envelope.Source),
		Revision: envelope.Source.ResolvedRevision, TreeDigest: envelope.TreeDigest,
		ManifestDigest: envelope.ManifestDigest, NextAction: nextLifecycleAction(result),
		DryRun: dryRun, Result: result, envelopeResult: addEnvelopeResult(result, nil),
	}
}

func nextLifecycleAction(result usecase.AddResult) string {
	return lifecycleAction(result, false)
}

// nextLocalLifecycleAction is private terminal guidance. Provider LocalActions
// may contain host paths, so public JSON must use nextLifecycleAction instead.
func nextLocalLifecycleAction(result usecase.AddResult) string {
	return lifecycleAction(result, true)
}

func lifecycleAction(result usecase.AddResult, includePrivate bool) string {
	action := ""
	if includePrivate && len(result.Activation.LocalActions) > 0 {
		action = result.Activation.LocalActions[0]
	} else if len(result.Activation.UserActions) > 0 {
		action = result.Activation.UserActions[0]
	} else if includePrivate && len(result.Plan.LocalActions) > 0 {
		action = result.Plan.LocalActions[0]
	} else if len(result.Plan.UserActions) > 0 {
		action = result.Plan.UserActions[0]
	}
	if result.Activation.Authentication == domain.AuthenticationPending {
		if action != "" {
			return fmt.Sprintf("%s; complete authentication, then rerun add to verify activation and authentication", action)
		}
		return "complete authentication for the prepared plugin in the selected client, then rerun add to verify activation and authentication"
	}
	if result.Activation.Authentication == domain.AuthenticationNotChecked {
		if action != "" {
			if strings.Contains(strings.ToLower(action), "verify this plugin's authentication requirements") {
				return action
			}
			return action + "; verify this plugin's authentication requirements before using it"
		}
		return "verify this plugin's authentication requirements before using it"
	}
	if action != "" {
		return action
	}
	return "start a new client session and verify the plugin is available"
}

func fullyInstalled(outcome domain.ActivationOutcome) bool {
	authComplete := outcome.Authentication == domain.AuthenticationNotRequired || outcome.Authentication == domain.AuthenticationComplete
	return outcome.Activation == domain.ActivationActive && outcome.Verification == domain.VerificationInstalled && authComplete
}
