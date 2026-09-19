package agentpluginscli

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
	clientplanner "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/planner"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
	"github.com/spf13/cobra"
)

// useInstallerFacadeForLocalAdd is intentionally narrow: one explicit local
// standard package, one qualified facade client, and a non-interactive user
// scope operation. Remote, Directory, multi-target, interactive lifecycle, and
// attestation flows continue through their existing contracts.
func useInstallerFacadeForLocalAdd(app App, opts *options, loaded loadedPackage, targets []domain.ClientID, activationComplete, authComplete, needsInstallConfirmation bool) bool {
	if app.Installer == nil || opts == nil || len(targets) != 1 || activationComplete || authComplete || needsInstallConfirmation {
		return false
	}
	if !automatedMutation(app, opts) || opts.scope != string(domain.ScopeUser) || loaded.origin != domain.OriginModeDirect {
		return false
	}
	if loaded.envelope.FormatID != domain.FormatIDAgentPluginsV1 || loaded.envelope.Source.SourceBindingHint != "direct-local" || loaded.envelope.SnapshotRoot == "" {
		return false
	}
	return app.Installer.SupportsClient(string(targets[0]))
}

func runInstallerFacadeLocalAdd(ctx context.Context, cmd *cobra.Command, app App, opts *options, loaded loadedPackage, clients []domain.DetectedClient) error {
	selected, detected, err := selectClient(cmd, app, opts, clients)
	if err != nil {
		return err
	}
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
		SourceRoot: loaded.envelope.Source.CanonicalSource,
		ClientID:   string(selected.ClientID), ClientConfigRoot: selected.ConfigRoot,
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
		return renderAddResultWithSecurity(cmd.OutOrStdout(), opts.format, loaded.envelope, loaded.security, preview, opts.dryRun)
	}
	if opts.format == "human" {
		if err := renderHumanPlan(cmd.OutOrStdout(), loaded.envelope, preview); err != nil {
			return err
		}
	}
	writeProgress(app, opts.format, "Applying transactional client package through the public installer facade...")
	result, applyErr := app.Installer.Apply(ctx, prepared, installer.Decision{Confirmed: true})
	output := facadeResultAddResult(plan, result, loaded.envelope)
	output.Receipt.OperationID = operationID
	if renderErr := renderAddResultErrorWithSecurity(cmd.OutOrStdout(), opts.format, loaded.envelope, loaded.security, output, false, applyErr); renderErr != nil && applyErr == nil {
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
	capabilities, _ := clientplanner.Capabilities(clientID)
	status := domain.PlanReady
	if plan.NoChange {
		status = domain.PlanReady
	}
	return usecase.AddResult{
		InstallationID: plan.InstallationID,
		Plan: domain.DeliveryPlan{
			ClientID: clientID, Scope: domain.ScopeUser, Status: status,
			PackageMode: capabilities.PackageMode, DeclaredName: envelope.Manifest.Name,
			DeclaredVersion: envelope.Manifest.Version, ActivePath: plan.TargetPath,
		},
		RequiresConfirmation: !plan.NoChange,
		NoChange:             plan.NoChange,
	}
}

func facadeResultAddResult(plan installer.Plan, result installer.Result, envelope domain.PackageEnvelope) usecase.AddResult {
	out := facadePlanAddResult(plan, envelope)
	out.InstallationID = firstNonEmptyCLI(result.InstallationID, plan.InstallationID)
	out.NoChange = result.NoChange || result.Outcome == installer.OutcomeUnchanged
	out.Mutated = result.Outcome == installer.OutcomeCompleted && !out.NoChange
	out.Activation = domain.ActivationOutcome{
		Activation:     domain.ActivationState(result.Client.Activation),
		Authentication: domain.AuthenticationState(result.Client.Authentication),
		Verification:   domain.VerificationState(result.Client.Verification),
		UserActions:    append([]string(nil), result.ManualActions...),
	}
	return out
}

func firstNonEmptyCLI(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
