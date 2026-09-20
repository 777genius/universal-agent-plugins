package agentpluginscli

import (
	"context"
	"errors"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

// useInstallerFacadeForLocalAdd is intentionally narrow: one explicit local
// standard package, one qualified facade client, and a non-interactive user
// scope operation. Remote, Directory, multi-target, interactive lifecycle, and
// attestation flows continue through their existing contracts.
func useInstallerFacadeForLocalAdd(app App, opts *options, loaded loadedPackage, targets []domain.ClientID, activationComplete, authComplete, needsInstallConfirmation bool, clients []domain.DetectedClient) bool {
	if app.Installer == nil || opts == nil || len(targets) != 1 || activationComplete || authComplete || needsInstallConfirmation {
		return false
	}
	if !automatedMutation(app, opts) || opts.scope != string(domain.ScopeUser) || loaded.origin != domain.OriginModeDirect {
		return false
	}
	if loaded.envelope.FormatID != domain.FormatIDAgentPluginsV1 || loaded.envelope.Source.SourceBindingHint != "direct-local" || loaded.envelope.SnapshotRoot == "" {
		return false
	}
	// Offline planning can use synthetic detected clients with no executable.
	// Keep that supported CLI path outside the facade's strict host contract.
	for _, client := range clients {
		if client.ClientID == targets[0] {
			return filepath.IsAbs(client.ExecutablePath) && filepath.Clean(client.ExecutablePath) == client.ExecutablePath && app.Installer.SupportsClient(string(targets[0]))
		}
	}
	return false
}

func runInstallerFacadeLocalAdd(ctx context.Context, cmd *cobra.Command, app App, opts *options, loaded loadedPackage, clients []domain.DetectedClient) error {
	selected, detected, err := selectClient(cmd, app, opts, clients)
	if err != nil {
		return err
	}
	state, err := app.StateStore.Load()
	if err != nil {
		return err
	}
	installation, exists := locallyMatchedInstallation(state, loaded.envelope.Manifest.Name)
	groupOutput := !exists || !installationHasTarget(installation, selected.ClientID, string(domain.ScopeUser))
	operationID, err := newOperationGroupID()
	if err != nil {
		return err
	}
	assessment := installer.Assessment{
		TreeDigest: loaded.envelope.TreeDigest,
		Outcome:    installer.AssessmentAllow,
		Reason:     "authorized by the CLI security policy",
	}
	request := installer.Request{
		Operation: installer.OpInstall, PackageRoot: loaded.envelope.SnapshotRoot,
		SourceRoot:      loaded.envelope.Source.CanonicalSource,
		ExecutableFiles: append([]string{}, loaded.envelope.ExecutableFiles...),
		ClientID:        string(selected.ClientID), ClientConfigRoot: selected.ConfigRoot,
		ClientExecutable: backendExecutable(selected, detected), OperationID: operationID,
		RequiredComponents: requiredFacadeComponents(loaded.envelope), Assessment: &assessment,
	}
	prepared, err := app.Installer.Prepare(ctx, request)
	if err != nil {
		return err
	}
	defer func() { _ = prepared.Close() }()
	plan := prepared.Plan()
	preview := facadePlanAddResult(plan, loaded.envelope)
	preview.Receipt.OperationID = operationID
	if opts.dryRun || plan.NoChange {
		return renderInstallerFacadeAdd(cmd, opts, loaded, preview, groupOutput, nil)
	}
	if opts.format == "human" {
		if err := renderHumanPlan(cmd.OutOrStdout(), loaded.envelope, preview); err != nil {
			return err
		}
	}
	writeProgress(app, opts.format, "Applying transactional client package through the public installer facade...")
	result, applyErr := app.Installer.Apply(ctx, prepared, installer.Decision{Confirmed: true})
	output := facadeResultAddResult(plan, result, loaded.envelope)
	if applyErr != nil {
		output.Failure = facadeApplyFailure(result, applyErr)
	}
	output.Receipt.OperationID = operationID
	if renderErr := renderInstallerFacadeAdd(cmd, opts, loaded, output, groupOutput, applyErr); renderErr != nil && applyErr == nil {
		applyErr = renderErr
	}
	return applyErr
}

func requiredFacadeComponents(envelope domain.PackageEnvelope) []string {
	required := make([]string, 0, 2)
	if len(envelope.MCP.Servers) > 0 {
		required = append(required, "mcp")
	}
	if len(envelope.Skills) > 0 {
		required = append(required, "skills")
	}
	return required
}

func facadePlanAddResult(plan installer.Plan, envelope domain.PackageEnvelope) usecase.AddResult {
	clientID := domain.ClientID(plan.ClientID)
	delivery := plan.Delivery
	out := usecase.AddResult{
		InstallationID: plan.InstallationID,
		Plan: domain.DeliveryPlan{
			ClientID: clientID, Scope: domain.ScopeUser, Status: domain.PlanStatus(delivery.Status),
			PackageMode: domain.PackageMode(delivery.PackageMode), DeclaredName: envelope.Manifest.Name,
			DeclaredVersion: envelope.Manifest.Version, ActivePath: plan.TargetPath,
			InstallIntent: domain.InstallIntent(delivery.InstallIntent), PhysicalArtifactID: delivery.PhysicalArtifactID,
			Activation: domain.ActivationState(delivery.Activation), Authentication: domain.AuthenticationState(delivery.Authentication),
			Policy: domain.PolicyState(delivery.Policy), Verification: domain.VerificationState(delivery.Verification),
			UserActions: append([]string(nil), delivery.UserActions...), LocalActions: append([]string(nil), delivery.LocalActions...),
			Warnings: append([]string(nil), delivery.Warnings...),
		},
		Activation: domain.ActivationOutcome{Activation: domain.ActivationState(plan.Client.Activation),
			Authentication: domain.AuthenticationState(plan.Client.Authentication), Policy: domain.PolicyState(plan.Client.Policy), Verification: domain.VerificationState(plan.Client.Verification)},
		RequiresConfirmation: plan.RequiresConfirmation,
		NoChange:             plan.NoChange,
	}
	for _, component := range delivery.Components {
		out.Plan.Components = append(out.Plan.Components, domain.ComponentDecision{Kind: domain.ComponentKind(component.Kind), Name: component.Name, Support: domain.SupportLevel(component.Support), Reason: component.Reason})
	}
	for _, diagnostic := range delivery.Diagnostics {
		out.Plan.Diagnostics = append(out.Plan.Diagnostics, domain.Diagnostic{Severity: domain.Severity(diagnostic.Severity), Boundary: domain.FailureBoundary(diagnostic.Boundary), Code: diagnostic.Code, Path: diagnostic.Path, Item: diagnostic.Item, Message: diagnostic.Message})
	}
	return out
}

func facadeResultAddResult(plan installer.Plan, result installer.Result, envelope domain.PackageEnvelope) usecase.AddResult {
	// Prepare is a preview: Apply may allocate a new identity and resolve
	// platform readiness. Publish the lifecycle's actual decisions when present.
	if result.Delivery != nil {
		plan.Delivery = *result.Delivery
		plan.TargetPath = result.Delivery.ActivePath
	}
	out := facadePlanAddResult(plan, envelope)
	out.InstallationID = firstNonEmptyCLI(result.InstallationID, plan.InstallationID)
	out.NoChange = result.NoChange || result.Outcome == installer.OutcomeUnchanged
	out.Mutated = result.Mutated
	out.RequiresConfirmation = result.RequiresConfirmation
	out.Activation = domain.ActivationOutcome{
		Activation:     domain.ActivationState(result.Client.Activation),
		Authentication: domain.AuthenticationState(result.Client.Authentication),
		Policy:         domain.PolicyState(result.Client.Policy),
		Verification:   domain.VerificationState(result.Client.Verification),
		UserActions:    append([]string(nil), result.ManualActions...),
	}
	return out
}

func facadeApplyFailure(result installer.Result, err error) *usecase.GroupTargetFailure {
	stage := "preflight"
	switch {
	case len(result.Recovery.Unknown) > 0, result.Outcome == installer.OutcomeRecovery, errors.Is(err, installer.ErrRecoveryRequired):
		stage = "persist"
	case result.Mutated:
		// Apply does not expose a typed state-save error. Unless the committed
		// client view proves a lifecycle failure, mutation plus error leaves
		// persistence uncertain and must not suggest a blind retry.
		stage = "persist"
		switch {
		case result.Client.Activation == string(domain.ActivationFailed):
			stage = "activation"
		case result.Client.Verification == string(domain.VerificationFailed):
			stage = "verification"
		case result.Client.Authentication == string(domain.AuthenticationFailed):
			stage = "authentication"
		}
	}
	message := err.Error()
	if result.Reason != "" && result.Reason != message {
		message = result.Reason + ": " + message
	}
	return &usecase.GroupTargetFailure{Stage: stage, Message: message}
}

// First-add uses the existing per-target envelope even for one target. Resume
// and repeat preserve their established single-result shape.
func renderInstallerFacadeAdd(cmd *cobra.Command, opts *options, loaded loadedPackage, result usecase.AddResult, grouped bool, commandErr error) error {
	if !grouped {
		return renderAddResultErrorWithSecurity(cmd.OutOrStdout(), opts.format, loaded.envelope, loaded.security, result, opts.dryRun, commandErr)
	}
	status := string(usecase.GroupPhasePlanned)
	result.GroupPhase = usecase.GroupTargetPlanned
	if !opts.dryRun {
		status = string(usecase.GroupPhaseCompleted)
		result.GroupPhase = usecase.GroupTargetExternalCompleted
		if commandErr != nil {
			status = "apply_failed"
			result.GroupPhase = usecase.GroupTargetExternalNotAttempted
			if result.Mutated {
				status = string(usecase.GroupPhaseExternalPartialFailure)
				result.GroupPhase = usecase.GroupTargetExternalFailed
			}
			if result.Failure != nil && result.Failure.Stage == "persist" {
				status = string(usecase.GroupPhaseManagedCommitUnknown)
				result.GroupPhase = usecase.GroupTargetManagedUnknown
			}
		}
	}
	output := newAddResultData(loaded.envelope, result, opts.dryRun)
	output.Security = loaded.security
	output.envelopeResult = addEnvelopeResult(result, commandErr)
	target := addTargetResult{Target: string(result.Plan.ClientID), Status: groupTargetStatus(result), Output: output, NextAction: nextLifecycleAction(result), Error: result.Failure}
	target.RetryCommand = batchRetryCommandFor(target, batchRetrySource(loaded.envelope, loaded.origin, output.Source))
	combined := addMultiResult{
		OperationID: result.Receipt.OperationID, Batch: true, Status: status,
		Plugin: output.Plugin, Version: output.Version, Source: output.Source, Revision: output.Revision,
		TreeDigest: output.TreeDigest, ManifestDigest: output.ManifestDigest, Security: loaded.security,
		DryRun: opts.dryRun, Targets: []addTargetResult{target},
	}
	if opts.dryRun {
		combined.Succeeded = 1
	} else {
		countBatchTarget(&combined, target)
	}
	return renderAddMultiResult(cmd, opts, combined, loaded.envelope)
}

func firstNonEmptyCLI(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
