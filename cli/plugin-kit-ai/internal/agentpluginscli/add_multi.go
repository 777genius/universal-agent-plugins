package agentpluginscli

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/prompt"
	"github.com/777genius/plugin-kit-ai/cli/internal/terminaltheme"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
	"github.com/spf13/cobra"
)

type addTargetResult struct {
	Target       string                      `json:"target"`
	Status       string                      `json:"status"`
	NextAction   string                      `json:"next_action,omitempty"`
	Error        *usecase.GroupTargetFailure `json:"error,omitempty"`
	RetryCommand string                      `json:"retry_command,omitempty"`
	Output       addResultData               `json:"output"`
	displayName  string
}

type addMultiResult struct {
	OperationID    string                     `json:"operation_id,omitempty"`
	Batch          bool                       `json:"batch"`
	Status         string                     `json:"status"`
	Succeeded      int                        `json:"succeeded"`
	Failed         int                        `json:"failed"`
	ActionRequired int                        `json:"action_required,omitempty"`
	Plugin         string                     `json:"plugin"`
	Version        string                     `json:"version,omitempty"`
	Source         string                     `json:"source"`
	Revision       string                     `json:"revision,omitempty"`
	TreeDigest     string                     `json:"tree_digest,omitempty"`
	ManifestDigest string                     `json:"manifest_digest,omitempty"`
	Security       *domain.SecurityAssessment `json:"security,omitempty"`
	Directory      *domain.DirectoryOrigin    `json:"directory,omitempty"`
	DryRun         bool                       `json:"dry_run"`
	Targets        []addTargetResult          `json:"targets"`
	Acquisition    *addAcquisitionProof       `json:"acquisition,omitempty"`
	TargetOutcomes map[string]addTargetProof  `json:"target_outcomes,omitempty"`
}

type batchPresentationKind string

const (
	batchPresentationInstalled      batchPresentationKind = "Installed"
	batchPresentationSetupRequired  batchPresentationKind = "Setup required"
	batchPresentationSignInRequired batchPresentationKind = "Sign-in required"
	batchPresentationFailed         batchPresentationKind = "Failed"
	batchPresentationNotCompleted   batchPresentationKind = "Not completed"
	batchPresentationRolledBack     batchPresentationKind = "Rolled back"
)

type addAcquisitionProof struct {
	AcquisitionID    string `json:"acquisition_id"`
	AcquisitionCount int    `json:"acquisition_count"`
	TreeDigest       string `json:"tree_digest"`
	ManifestDigest   string `json:"manifest_digest"`
	ClosureDigest    string `json:"closure_digest"`
	SourceKind       string `json:"source_kind"`
	Fetched          bool   `json:"fetched"`
	Validated        bool   `json:"validated"`
}

type addTargetProof struct {
	Outcome        string `json:"outcome"`
	AcquisitionID  string `json:"acquisition_id"`
	TreeDigest     string `json:"tree_digest"`
	ManifestDigest string `json:"manifest_digest"`
	ClosureDigest  string `json:"closure_digest"`
}

// runAddMany is deliberately not implemented as repeated CLI invocations. It
// resolves and validates one package, detects clients once, builds every plan,
// and only begins applying after the complete selected set passes preflight.
func runAddMany(ctx context.Context, cmd *cobra.Command, app App, opts *options, source string, targets []domain.ClientID, activationComplete, authComplete bool) error {
	return runAddManyWithClients(ctx, cmd, app, opts, source, targets, activationComplete, authComplete, nil)
}

func runAddManyWithClients(ctx context.Context, cmd *cobra.Command, app App, opts *options, source string, targets []domain.ClientID, activationComplete, authComplete bool, clients []domain.DetectedClient) error {
	_, detected, err := preflightAddTargets(ctx, app, opts, source, targets, clients)
	if err != nil {
		return err
	}
	writeProgress(app, opts.format, "Resolving and validating one Agent Plugin package for every selected target...")
	loaded, err := app.loadPackageFor(ctx, source, withDetectedClients(app.addResolutionRequest(source, expandAffectedSurfaceTargets(targets)), detected))
	if err != nil {
		return err
	}
	if loaded.cleanup != nil {
		defer loaded.cleanup()
	}
	return runAddManyLoaded(ctx, cmd, app, opts, loaded, targets, activationComplete, authComplete, detectedClientValues(detected), false)
}

func runAddManyLoaded(ctx context.Context, cmd *cobra.Command, app App, opts *options, loaded loadedPackage, targets []domain.ClientID, activationComplete, authComplete bool, clients []domain.DetectedClient, needsInstallConfirmation bool) error {
	applyLoadedGuidedIntents(opts, loaded, targets)
	if opts.chatGPTAppID != "" && !loaded.chatGPTPreparation {
		return fmt.Errorf("--chatgpt-app-id requires the signed Context7 Directory source and --target chatgpt")
	}
	if err := authorizeSecurityAssessment(cmd, app, opts, &loaded); err != nil {
		return err
	}
	deferredChatGPT := loaded.chatGPTPreparation && loaded.localChatGPTMapping == nil && containsClientID(targets, domain.ClientChatGPT)
	if deferredChatGPT && len(targets) == 1 {
		// Setup-required ChatGPT registration is not an install failure.
		action := chatGPTRegistrationResumeAction(cmd, loaded.envelope.Source.RequestedSource, []domain.ClientID{domain.ClientChatGPT})
		if opts.format == "json" {
			return writeJSONResult(cmd.OutOrStdout(), "add", outputResultSuccess, map[string]any{
				"status": "action_required", "target": "chatgpt", "mcp_url": domain.Context7ChatGPTURL,
				"authentication": "none", "next_action": action, "remote_verified": true, "mutated": false,
			})
		}
		_, err := fmt.Fprintln(cmd.OutOrStdout(), action)
		return err
	}
	executionTargets := targets
	if deferredChatGPT {
		executionTargets = withoutClientID(targets, domain.ClientChatGPT)
	}
	if len(executionTargets) == 1 && !deferredChatGPT {
		selectedOptions := *opts
		selectedOptions.target = string(executionTargets[0])
		if activationComplete || authComplete {
			return runAddLoaded(ctx, cmd, app, &selectedOptions, loaded, activationComplete, authComplete, clients, needsInstallConfirmation)
		}
		if state, err := app.StateStore.Load(); err == nil {
			if installation, ok := locallyMatchedInstallation(state, loaded.envelope.Manifest.Name); ok && installationHasTarget(installation, executionTargets[0], string(domain.ScopeUser)) {
				return runAddLoaded(ctx, cmd, app, &selectedOptions, loaded, false, false, clients, needsInstallConfirmation)
			}
		}
	}
	combined := addMultiResult{
		Batch: true, Status: "planned", Plugin: loaded.envelope.Manifest.Name,
		Version: loaded.envelope.Manifest.Version, Source: publicPackageSource(loaded.envelope.Source),
		Revision: loaded.envelope.Source.ResolvedRevision, TreeDigest: loaded.envelope.TreeDigest, ManifestDigest: loaded.envelope.ManifestDigest,
		Security:  loaded.security,
		Directory: cloneDirectoryOrigin(loaded.directory),
		DryRun:    opts.dryRun, Targets: make([]addTargetResult, 0, len(targets)),
	}
	selected, detected, err := preflightSelectedTargets(ctx, app, executionTargets, clients, false)
	if err != nil {
		return err
	}
	combined.Targets = combined.Targets[:0]
	service := lifecycleService(app, detected)
	inputs := make([]usecase.AddInput, len(selected))
	for index, client := range selected {
		clientPackage := cloneLoadedPackage(loaded)
		if err := prepareLoadedPackageForClient(&clientPackage, client.ClientID); err != nil {
			return fmt.Errorf("preflight target %s: %w; no target was changed", client.ClientID, err)
		}
		input := usecase.AddInput{
			InstallIntent: opts.installIntents[client.ClientID],
			Envelope:      clientPackage.envelope, Client: client, Scope: domain.ScopeUser,
			DryRun: true, Interactive: app.Terminal, Hints: clientPackage.hints,
			BackendExecutable:  backendExecutable(client, detected),
			ActivationComplete: activationComplete, AuthComplete: authComplete,
			PersistAuthoritativeObservations: false,
			OriginMode:                       loaded.origin, DirectoryResolution: cloneDirectoryOrigin(loaded.directory),
			DistributionSuspended: loaded.distributionSuspended, ReleaseRevoked: loaded.releaseRevoked,
		}
		inputs[index] = input
	}
	operationID, err := newOperationGroupID()
	if err != nil {
		return err
	}
	combined.OperationID = operationID
	groupInput := usecase.GroupInput{Targets: inputs, OperationGroupID: operationID, DryRun: true}
	planned, err := service.AddGroup(ctx, groupInput)
	combined.Targets = combined.Targets[:0]
	for index, result := range planned.Targets {
		output := newAddResultData(inputs[index].Envelope, result, true)
		output.OperationID = operationID
		combined.Targets = append(combined.Targets, addTargetResult{
			Target: string(selected[index].ClientID), Status: groupTargetStatus(result), Output: output,
			NextAction: nextLifecycleAction(result), displayName: clientDisplayName(selected[index]),
		})
		combined.setTargetProof(selected[index].ClientID, "not_run")
	}
	combined.Succeeded = len(planned.Targets)
	if err != nil {
		combined.Status, combined.Failed, combined.Succeeded = "preflight_failed", len(inputs), 0
		setPreflightNextActions(combined.Targets)
		_ = renderAddMultiResult(cmd, opts, combined, loaded.envelope)
		return fmt.Errorf("group preflight failed; no target was changed (selected targets: %v): %w%s", targets, err, addGroupNextAction(combined.Targets))
	}
	if opts.dryRun {
		if deferredChatGPT {
			appendDeferredChatGPT(&combined, cmd, loaded)
		}
		return renderAddMultiResult(cmd, opts, combined, loaded.envelope)
	}
	if needsInstallConfirmation {
		accepted, err := confirmInstall(ctx, cmd, app, loaded, humanAddGroupPlans(combined.Targets))
		if err != nil {
			return err
		}
		if !accepted {
			_, err = fmt.Fprintln(cmd.OutOrStdout(), "Installation not applied.")
			return err
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(targets) > 1 {
		proof, proofErr := newAddAcquisitionProof(loaded)
		if proofErr != nil {
			return proofErr
		}
		combined.Acquisition = &proof
		combined.TargetOutcomes = make(map[string]addTargetProof, len(selected))
		for _, client := range selected {
			combined.setTargetProof(client.ClientID, "not_run")
		}
	}
	writeProgress(app, opts.format, "Applying the completely preflighted multi-target plan...")
	groupInput.DryRun, groupInput.Confirmed = false, true
	applied, err := service.AddGroup(ctx, groupInput)
	if len(applied.Targets) != len(selected) || len(applied.Targets) != len(inputs) {
		if err != nil {
			return fmt.Errorf("group apply returned %d targets for %d selected clients: %w", len(applied.Targets), len(selected), err)
		}
		return fmt.Errorf("group apply returned %d targets for %d selected clients", len(applied.Targets), len(selected))
	}
	combined.Status, combined.Targets, combined.Succeeded, combined.Failed, combined.ActionRequired = string(applied.Phase), combined.Targets[:0], 0, 0, 0
	sourceArg := batchRetrySource(loaded.envelope, loaded.origin, combined.Source)
	for index, result := range applied.Targets {
		output := newAddResultData(inputs[index].Envelope, result, false)
		output.OperationID = operationID
		entry := addTargetResult{
			Target: string(selected[index].ClientID), Status: groupTargetStatus(result), Output: output,
			NextAction: nextLifecycleAction(result), displayName: clientDisplayName(selected[index]),
			Error: result.Failure,
		}
		if retry := batchRetryCommandFor(entry, sourceArg); retry != "" {
			entry.RetryCommand = retry
		}
		combined.Targets = append(combined.Targets, entry)
		combined.setTargetProof(selected[index].ClientID, addTargetProofOutcome(result))
		countBatchTarget(&combined, entry)
	}
	if err != nil {
		if applied.Phase == usecase.GroupPhasePlanned && !applied.Mutated {
			combined.Status, combined.Failed, combined.Succeeded, combined.ActionRequired = "preflight_failed", len(inputs), 0, 0
			setPreflightNextActions(combined.Targets)
			_ = renderAddMultiResult(cmd, opts, combined, loaded.envelope)
			return fmt.Errorf("group apply preflight failed; no target was changed (selected targets: %v): %w%s", targets, err, addGroupNextAction(combined.Targets))
		}
		combined.Status = groupFailureStatus(applied.Phase)
		// Always report selected ChatGPT setup, even when installable peers failed.
		if deferredChatGPT {
			appendDeferredChatGPT(&combined, cmd, loaded)
		}
		_ = renderAddMultiResult(cmd, opts, combined, loaded.envelope)
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return err
		}
		if applied.Phase == usecase.GroupPhaseManagedActivationFailed || applied.Phase == usecase.GroupPhaseExternalPartialFailure {
			return batchActivationAggregateError(combined)
		}
		return err
	}
	if deferredChatGPT {
		// Manual ChatGPT registration stays outside the atomic local-client group.
		appendDeferredChatGPT(&combined, cmd, loaded)
	}
	if err := renderAddMultiResult(cmd, opts, combined, loaded.envelope); err != nil {
		return err
	}
	if combined.Failed > 0 {
		return batchActivationAggregateError(combined)
	}
	if app.Terminal && opts.format == "human" && len(applied.Targets) == 1 && !deferredChatGPT {
		input := inputs[0]
		input.DryRun = false
		input.InstallationID = applied.Targets[0].InstallationID
		return resumeInteractiveLifecycle(ctx, cmd, service, input, inputs[0].Envelope, applied.Targets[0])
	}
	return nil
}

func containsClientID(targets []domain.ClientID, wanted domain.ClientID) bool {
	for _, target := range targets {
		if target == wanted {
			return true
		}
	}
	return false
}

func withoutClientID(targets []domain.ClientID, excluded domain.ClientID) []domain.ClientID {
	filtered := make([]domain.ClientID, 0, len(targets))
	for _, target := range targets {
		if target != excluded {
			filtered = append(filtered, target)
		}
	}
	return filtered
}

func appendDeferredChatGPT(result *addMultiResult, cmd *cobra.Command, loaded loadedPackage) {
	action := chatGPTRegistrationResumeAction(cmd, loaded.envelope.Source.RequestedSource, []domain.ClientID{domain.ClientChatGPT})
	if result.DryRun {
		result.Status = "planned_with_action_required"
	} else if result.Failed == 0 {
		// Preserve failure statuses such as external_partial_failure.
		result.Status = "completed_with_action_required"
	}
	entry := addTargetResult{
		Target: string(domain.ClientChatGPT), Status: "action_required", NextAction: action,
		displayName: "ChatGPT",
	}
	result.Targets = append(result.Targets, entry)
	countBatchTarget(result, entry)
	result.setTargetProof(domain.ClientChatGPT, "not_run")
}

func newAddAcquisitionProof(loaded loadedPackage) (addAcquisitionProof, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return addAcquisitionProof{}, fmt.Errorf("create acquisition ID: %w", err)
	}
	sourceKind := acquisitionSourceKind(loaded)
	return addAcquisitionProof{
		AcquisitionID:    "acq-" + hex.EncodeToString(value[:]),
		AcquisitionCount: 1,
		TreeDigest:       loaded.envelope.TreeDigest,
		ManifestDigest:   loaded.envelope.ManifestDigest,
		ClosureDigest:    groupedAcquisitionClosureDigest(sourceKind, loaded.envelope.Source, loaded.envelope.TreeDigest, loaded.envelope.ManifestDigest),
		SourceKind:       sourceKind,
		Fetched:          loaded.envelope.Source.Repository != "",
		Validated:        true,
	}, nil
}

func acquisitionSourceKind(loaded loadedPackage) string {
	if loaded.origin == domain.OriginModeDirectory {
		return "directory"
	}
	if loaded.envelope.Source.Repository != "" {
		return "github"
	}
	return "local"
}

// groupedAcquisitionClosureDigest is the domain-separated SHA-256 of a
// length-prefixed tuple:
//
//	source kind, repository, package subpath, resolved revision,
//	validated tree digest, validated manifest digest
//
// The agentplugins/grouped-acquisition-closure/v1 domain makes this a distinct
// identity from a package tree digest. Requested and canonical source strings
// are deliberately excluded because they may contain local, host-specific
// paths. For local acquisitions, source kind plus the two validated package
// identities form the closure; immutable remote acquisitions additionally bind
// repository, subpath, and revision.
func groupedAcquisitionClosureDigest(sourceKind string, source domain.SourceIdentity, treeDigest, manifestDigest string) string {
	hash := sha256.New()
	fields := []string{
		"agentplugins/grouped-acquisition-closure/v1",
		sourceKind,
		strings.TrimSpace(source.Repository),
		strings.TrimSpace(source.PackageSubpath),
		strings.TrimSpace(source.ResolvedRevision),
		strings.TrimSpace(treeDigest),
		strings.TrimSpace(manifestDigest),
	}
	var size [8]byte
	for _, field := range fields {
		binary.BigEndian.PutUint64(size[:], uint64(len(field)))
		_, _ = hash.Write(size[:])
		_, _ = hash.Write([]byte(field))
	}
	return "sha256:" + hex.EncodeToString(hash.Sum(nil))
}

func (result *addMultiResult) setTargetProof(target domain.ClientID, outcome string) {
	if result.Acquisition == nil || result.TargetOutcomes == nil {
		return
	}
	proof := result.Acquisition
	result.TargetOutcomes[string(target)] = addTargetProof{
		Outcome: outcome, AcquisitionID: proof.AcquisitionID,
		TreeDigest: proof.TreeDigest, ManifestDigest: proof.ManifestDigest, ClosureDigest: proof.ClosureDigest,
	}
}

func addTargetProofOutcome(result usecase.AddResult) string {
	switch result.GroupPhase {
	case usecase.GroupTargetExternalCompleted:
		return "passed"
	case usecase.GroupTargetExternalFailed:
		return "failed"
	case usecase.GroupTargetManagedRolledBack:
		return "rolled_back"
	case usecase.GroupTargetManagedUnknown:
		return "unknown"
	case usecase.GroupTargetExternalPartial:
		return "partial"
	case usecase.GroupTargetExternalNotAttempted:
		return "not_completed"
	default:
		return "not_completed"
	}
}

func groupFailureStatus(phase usecase.GroupPhase) string {
	switch phase {
	case usecase.GroupPhaseManagedRolledBack, usecase.GroupPhaseManagedCommitUnknown,
		usecase.GroupPhaseManagedActivationFailed, usecase.GroupPhaseExternalPartialFailure:
		return string(phase)
	case usecase.GroupPhaseManagedCommitted:
		return "managed_committed_finalization_failed"
	default:
		return "apply_failed"
	}
}

func groupTargetStatus(result usecase.AddResult) string {
	if result.GroupPhase != "" {
		return string(result.GroupPhase)
	}
	return string(result.Plan.Status)
}

func publicPackageSource(source domain.SourceIdentity) string {
	if source.Repository != "" {
		value := source.Repository
		if source.PackageSubpath != "" {
			value += "//" + source.PackageSubpath
		}
		return value
	}
	if source.ResolvedRevision != "" {
		return "direct immutable source"
	}
	return "direct local source"
}

func cloneLoadedPackage(source loadedPackage) loadedPackage {
	clone := source
	clone.cleanup = nil
	clone.envelope.Inventory.Skills = append([]string(nil), source.envelope.Inventory.Skills...)
	clone.envelope.Inventory.MCPServers = append([]string(nil), source.envelope.Inventory.MCPServers...)
	clone.envelope.Inventory.AppBindings = append([]string(nil), source.envelope.Inventory.AppBindings...)
	clone.envelope.App.Bindings = make(map[string]domain.AppBinding, len(source.envelope.App.Bindings))
	for key, value := range source.envelope.App.Bindings {
		clone.envelope.App.Bindings[key] = value
	}
	return clone
}

func renderAddMultiResult(cmd *cobra.Command, opts *options, result addMultiResult, envelope domain.PackageEnvelope) error {
	if opts.format == "json" {
		overall := "success"
		if result.Failed > 0 || batchStatusIndicatesFailure(result.Status) {
			overall = "failure"
		}
		return writeJSONResult(cmd.OutOrStdout(), "add", overall, result)
	}
	if result.Status == "preflight_failed" {
		if _, err := fmt.Fprintln(cmd.OutOrStdout(), "Nothing was installed: a selected client could not pass the checks before installation. See the error below; there is no new configuration to activate."); err != nil {
			return err
		}
		for _, target := range result.Targets {
			if target.Output.Result.Plan.Status == domain.PlanUnsupported {
				if err := renderHumanPlan(cmd.OutOrStdout(), envelope, target.Output.Result); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if result.DryRun {
		if len(result.Targets) == 1 {
			return renderAddResult(cmd.OutOrStdout(), "human", envelope, result.Targets[0].Output.Result, true)
		}
		return renderAddMultiDryRun(cmd.OutOrStdout(), result)
	}
	// Failed single-target applies use the same summary so error detail and retry
	// guidance stay visible instead of the legacy "Add: external_failed" line.
	if len(result.Targets) == 1 && result.Failed == 0 && !batchStatusIndicatesFailure(result.Status) {
		return renderAddResult(cmd.OutOrStdout(), "human", envelope, result.Targets[0].Output.Result, false)
	}
	return renderAddMultiApplySummary(cmd.OutOrStdout(), result, envelope)
}

func renderAddMultiDryRun(writer io.Writer, result addMultiResult) error {
	if _, err := fmt.Fprintf(writer, "Plugin: %s %s\n", result.Plugin, result.Version); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(writer, "Targets: %s\n", addResultTargets(result.Targets)); err != nil {
		return err
	}
	for _, target := range result.Targets {
		if target.Status == "action_required" {
			if _, err := fmt.Fprintf(writer, "  %s: setup required\n    Next: %s\n", target.Target, target.NextAction); err != nil {
				return err
			}
			continue
		}
		if err := renderOpenCodeRuntimeNotice(writer, target.Output.Result); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(writer, "  %s: %s\n", target.Target, target.Status); err != nil {
			return err
		}
		if target.NextAction != "" && !fullyInstalled(target.Output.Result.Activation) {
			if _, err := fmt.Fprintf(writer, "    Next: %s\n", localTargetLifecycleAction(target.Output.Result, target.NextAction)); err != nil {
				return err
			}
		}
	}
	if _, err := fmt.Fprintln(writer, "No changes made (dry run)."); err != nil {
		return err
	}
	return nil
}

func renderAddMultiApplySummary(writer io.Writer, result addMultiResult, envelope domain.PackageEnvelope) error {
	theme := terminaltheme.For(writer)
	title := batchApplyHeadline(result, envelope)
	if _, err := fmt.Fprintln(writer, theme.Text(terminaltheme.Label, title)); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(writer); err != nil {
		return err
	}
	width := utf8.RuneCountInString("Client")
	for _, target := range result.Targets {
		if name := batchTargetLabel(target); utf8.RuneCountInString(name) > width {
			width = utf8.RuneCountInString(name)
		}
	}
	if _, err := fmt.Fprintf(writer, "%-*s  %s\n", width, "Client", "Result"); err != nil {
		return err
	}
	for _, target := range result.Targets {
		label := batchTargetLabel(target)
		kind := classifyBatchPresentation(target)
		if _, err := fmt.Fprintf(writer, "%-*s  %s\n", width, prompt.SafeText(label), theme.Text(batchResultTone(kind), string(kind))); err != nil {
			return err
		}
	}
	attention := make([]addTargetResult, 0, len(result.Targets))
	for _, target := range result.Targets {
		switch classifyBatchPresentation(target) {
		case batchPresentationSetupRequired, batchPresentationSignInRequired, batchPresentationFailed, batchPresentationNotCompleted, batchPresentationRolledBack:
			attention = append(attention, target)
		}
	}
	if len(attention) == 0 {
		return nil
	}
	if _, err := fmt.Fprintln(writer); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(writer, theme.Text(terminaltheme.Warning, "Needs attention")); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(writer); err != nil {
		return err
	}
	for index, target := range attention {
		if index > 0 {
			if _, err := fmt.Fprintln(writer); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintln(writer, prompt.SafeText(batchTargetLabel(target))); err != nil {
			return err
		}
		if err := renderOpenCodeRuntimeNotice(writer, target.Output.Result); err != nil {
			return err
		}
		for _, line := range batchAttentionLines(target) {
			for _, part := range strings.Split(line, "\n") {
				if strings.TrimSpace(part) == "" {
					continue
				}
				if _, err := fmt.Fprintf(writer, "  %s\n", prompt.SafeText(part)); err != nil {
					return err
				}
			}
		}
		if target.RetryCommand != "" {
			if _, err := fmt.Fprintln(writer, "  Retry:"); err != nil {
				return err
			}
			if _, err := fmt.Fprintf(writer, "    %s\n", prompt.SafeText(target.RetryCommand)); err != nil {
				return err
			}
		}
	}
	return nil
}

func batchApplyHeadline(result addMultiResult, envelope domain.PackageEnvelope) string {
	name := strings.TrimSpace(envelope.Manifest.Name)
	if name == "" {
		name = result.Plugin
	}
	switch {
	case result.Failed > 0 && result.Succeeded > 0:
		return name + " installation finished with issues"
	case result.Failed > 0:
		return name + " installation failed"
	case result.ActionRequired > 0:
		return name + " installation prepared"
	default:
		return name + " installation completed"
	}
}

func batchTargetLabel(target addTargetResult) string {
	if strings.TrimSpace(target.displayName) != "" {
		return target.displayName
	}
	if definition, ok := domain.ClientDefinitionFor(domain.ClientID(target.Target)); ok && definition.DisplayName != "" {
		return definition.DisplayName
	}
	return target.Target
}

func classifyBatchPresentation(target addTargetResult) batchPresentationKind {
	if target.Status == "action_required" {
		return batchPresentationSetupRequired
	}
	phase := target.Output.Result.GroupPhase
	switch phase {
	case usecase.GroupTargetManagedRolledBack:
		return batchPresentationRolledBack
	case usecase.GroupTargetExternalNotAttempted:
		return batchPresentationNotCompleted
	case usecase.GroupTargetExternalFailed, usecase.GroupTargetExternalPartial, usecase.GroupTargetManagedUnknown:
		return batchPresentationFailed
	}
	activation := target.Output.Result.Activation
	if target.Error != nil || activation.Activation == domain.ActivationFailed || activation.Authentication == domain.AuthenticationFailed || activation.Verification == domain.VerificationFailed {
		return batchPresentationFailed
	}
	if activation.Authentication == domain.AuthenticationPending {
		return batchPresentationSignInRequired
	}
	if fullyInstalled(activation) {
		return batchPresentationInstalled
	}
	if activation.Authentication == domain.AuthenticationNotChecked {
		return batchPresentationSetupRequired
	}
	if activation.Activation == domain.ActivationPrepared || activation.Activation == domain.ActivationManual || target.Status == string(usecase.GroupTargetExternalCompleted) {
		return batchPresentationSetupRequired
	}
	if phase == usecase.GroupTargetExternalCompleted {
		return batchPresentationSetupRequired
	}
	return batchPresentationFailed
}

func batchResultTone(kind batchPresentationKind) terminaltheme.Role {
	switch kind {
	case batchPresentationInstalled:
		return terminaltheme.Success
	case batchPresentationSetupRequired, batchPresentationSignInRequired:
		return terminaltheme.Warning
	case batchPresentationFailed, batchPresentationNotCompleted, batchPresentationRolledBack:
		return terminaltheme.Error
	default:
		return terminaltheme.Muted
	}
}

func batchAttentionLines(target addTargetResult) []string {
	kind := classifyBatchPresentation(target)
	phase := target.Output.Result.GroupPhase
	switch kind {
	case batchPresentationRolledBack:
		return []string{"Managed installation was rolled back; no client changes were kept."}
	case batchPresentationFailed, batchPresentationNotCompleted:
		if phase == usecase.GroupTargetManagedUnknown {
			return []string{"Managed commit state is unknown; run agentplugins doctor before retrying."}
		}
		if batchFailureNeedsDoctor(target) {
			return []string{"Installation state could not be saved safely; run agentplugins doctor before retrying."}
		}
		if target.Error != nil && strings.TrimSpace(target.Error.Message) != "" {
			return []string{target.Error.Message}
		}
		if action := localTargetLifecycleAction(target.Output.Result, target.NextAction); action != "" {
			return []string{action}
		}
		if kind == batchPresentationNotCompleted {
			return []string{"Client installation was not attempted."}
		}
		return []string{"Client installation failed."}
	default:
		action := localTargetLifecycleAction(target.Output.Result, target.NextAction)
		if action == "" {
			action = nextLifecycleAction(target.Output.Result)
		}
		if action == "" {
			return []string{"Finish setup in the selected client."}
		}
		return []string{action}
	}
}

func countBatchTarget(result *addMultiResult, target addTargetResult) {
	switch classifyBatchPresentation(target) {
	case batchPresentationInstalled:
		result.Succeeded++
	case batchPresentationSetupRequired, batchPresentationSignInRequired:
		// Deferred ChatGPT (status action_required, no group phase) is reported
		// separately and must not inflate succeeded beyond prepared peers.
		if target.Output.Result.GroupPhase == usecase.GroupTargetExternalCompleted || fullyInstalled(target.Output.Result.Activation) {
			result.Succeeded++
		}
		result.ActionRequired++
	case batchPresentationFailed, batchPresentationNotCompleted, batchPresentationRolledBack:
		result.Failed++
	default:
		result.Failed++
	}
}

func batchRetrySource(envelope domain.PackageEnvelope, origin domain.OriginMode, public string) string {
	// Directory retries must keep the safe product selector (for example context7),
	// not a reconstructed repository//path that changes provenance.
	if origin == domain.OriginModeDirectory {
		if source := strings.TrimSpace(envelope.Source.RequestedSource); isBatchRetryableSource(source) {
			return source
		}
		return ""
	}
	// Direct GitHub retries require an immutable owner/repo@FULL_SHA[//path] selector.
	if repo := strings.TrimSpace(envelope.Source.Repository); repo != "" {
		return batchRetryGitHubSource(envelope.Source)
	}
	if source := strings.TrimSpace(envelope.Source.RequestedSource); isBatchRetryableSource(source) {
		return source
	}
	if source := strings.TrimSpace(public); isBatchRetryableSource(source) {
		return source
	}
	return ""
}

func batchRetryGitHubSource(source domain.SourceIdentity) string {
	repo := strings.TrimSpace(source.Repository)
	rev := publicImmutableRevision(strings.TrimSpace(source.ResolvedRevision))
	if repo == "" || rev == "" {
		return ""
	}
	value := repo + "@" + rev
	if sub := strings.TrimSpace(source.PackageSubpath); sub != "" {
		value += "//" + sub
	}
	if !isBatchRetrySafeArg(value) {
		return ""
	}
	return value
}

func isBatchPresentationSource(source string) bool {
	switch strings.TrimSpace(source) {
	case "", "direct local source", "direct immutable source":
		return true
	default:
		return false
	}
}

func isBatchRetryableSource(source string) bool {
	source = strings.TrimSpace(source)
	if source == "" || isBatchPresentationSource(source) || isLocalFilesystemSource(source) {
		return false
	}
	return true
}

func isLocalFilesystemSource(source string) bool {
	source = strings.TrimSpace(source)
	if source == "" || strings.Contains(source, "://") {
		return false
	}
	if strings.HasPrefix(source, ".") || strings.HasPrefix(source, "~") {
		return true
	}
	return filepath.IsAbs(source)
}

func batchRetryCommandFor(target addTargetResult, source string) string {
	if strings.TrimSpace(source) == "" || batchFailureNeedsDoctor(target) {
		return ""
	}
	switch target.Output.Result.GroupPhase {
	case usecase.GroupTargetExternalFailed, usecase.GroupTargetExternalNotAttempted:
		return batchRetryCommand(source, domain.ClientID(target.Target))
	default:
		return ""
	}
}

func batchFailureNeedsDoctor(target addTargetResult) bool {
	if target.Error == nil {
		return false
	}
	return target.Error.Stage == "persist"
}

func batchRetryCommand(source string, target domain.ClientID) string {
	source = strings.TrimSpace(source)
	targetID := strings.TrimSpace(string(target))
	if source == "" || targetID == "" {
		return ""
	}
	// Emit only shell-safe unquoted args so the same command works under npx on
	// POSIX shells and Windows CMD without POSIX-only quoting.
	if !isBatchRetrySafeArg(source) || !isBatchRetrySafeArg(targetID) {
		return ""
	}
	return "npx universal-agent-plugins add " + source + " --target " + targetID
}

func isBatchRetrySafeArg(value string) bool {
	return value != "" && strings.IndexFunc(value, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) && !strings.ContainsRune("_./:@,+-=", r)
	}) == -1
}

func batchActivationAggregateError(result addMultiResult) error {
	// Exclude deferred ChatGPT (and similar action_required-only rows) from the
	// denominator so partial-failure messaging stays tied to installable peers.
	total := result.Succeeded + result.Failed
	if total == 0 {
		total = countInstallableBatchTargets(result.Targets)
	}
	if total == 0 {
		return fmt.Errorf("client installations failed; see results above")
	}
	return fmt.Errorf("%d of %d client installations failed; see results above", result.Failed, total)
}

func countInstallableBatchTargets(targets []addTargetResult) int {
	count := 0
	for _, target := range targets {
		if target.Status == "action_required" && target.Output.Result.GroupPhase == "" {
			continue
		}
		count++
	}
	return count
}

func batchStatusIndicatesFailure(status string) bool {
	switch status {
	case "", string(usecase.GroupPhaseCompleted), string(usecase.GroupPhasePlanned),
		"completed_with_action_required", "planned_with_action_required", "action_required":
		return false
	default:
		return true
	}
}

func clientDisplayName(client domain.DetectedClient) string {
	if strings.TrimSpace(client.DisplayName) != "" {
		return client.DisplayName
	}
	if definition, ok := domain.ClientDefinitionFor(client.ClientID); ok {
		return definition.DisplayName
	}
	return string(client.ClientID)
}

func addResultTargets(results []addTargetResult) string {
	values := make([]string, len(results))
	for index, result := range results {
		values[index] = result.Target
	}
	return strings.Join(values, ",")
}

// The group result describes physical delivery. Review must also identify the
// selected logical surface when Copilot and VS Code share that delivery.
// Copy the presentation data so JSON, apply inputs and persisted IDs keep their
// physical ownership semantics.
func humanAddGroupPlans(targets []addTargetResult) []usecase.AddResult {
	results := make([]usecase.AddResult, len(targets))
	for index, target := range targets {
		result := target.Output.Result
		if target.Target != string(result.Plan.ClientID) {
			result.Plan.LocalActions = append(append([]string(nil), result.Plan.LocalActions...),
				fmt.Sprintf("Uses shared physical binding owned by %s", result.Plan.ClientID))
			result.Plan.ClientID = domain.ClientID(target.Target)
		}
		results[index] = result
	}
	return results
}

// A ready plan is not actionable when the selected group failed preflight.
// Preserve unsupported-package recovery instructions, but never promise
// activation from a plan whose requirements were rejected.
func setPreflightNextActions(targets []addTargetResult) {
	for index := range targets {
		if targets[index].Output.Result.Plan.Status != domain.PlanUnsupported {
			targets[index].NextAction = "resolve the reported client requirement and retry; nothing was installed"
			targets[index].Output.NextAction = targets[index].NextAction
		}
	}
}

func addGroupNextAction(targets []addTargetResult) string {
	for _, target := range targets {
		if target.NextAction != "" {
			return "; next action: " + target.NextAction
		}
	}
	return ""
}
