package grok

import (
	"context"
	"errors"
	"fmt"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

var (
	_ clients.Lifecycle          = (*Adapter)(nil)
	_ clients.AutomaticActivator = (*Adapter)(nil)
	_ clients.ReadOnlyVerifier   = (*Adapter)(nil)
)

func (*Adapter) AutomaticallyActivates(env clients.Env, request domain.ActivationRequest) bool {
	if request.Plan.Scope == domain.ScopeProject || request.Plan.TargetContext.Scope == domain.ScopeProject || request.Plan.TargetContext.ProjectRoot != "" {
		return false
	}
	return shared.HasClientCLI(env, request.BackendExecutable)
}

func (*Adapter) VerifierAvailable(client domain.DetectedClient, plan domain.DeliveryPlan, backendExecutable string) bool {
	if plan.Scope == domain.ScopeProject || plan.TargetContext.Scope == domain.ScopeProject || plan.TargetContext.ProjectRoot != "" {
		return projectSkillsVerifiable(client, plan, backendExecutable)
	}
	return shared.HasListingCLI(backendExecutable)
}

func (*Adapter) Activate(ctx context.Context, env clients.Env, request domain.ActivationRequest) (domain.ActivationOutcome, error) {
	if request.ConfirmPluginEnable && !New().SupportsPluginEnablement(request.Plan.Scope) {
		return domain.ActivationOutcome{}, errEnablementUnqualified
	}
	if request.Plan.Scope == domain.ScopeProject || request.Plan.TargetContext.Scope == domain.ScopeProject || request.Plan.TargetContext.ProjectRoot != "" {
		return activateProject(ctx, env, request)
	}
	return activateUser(ctx, env, request)
}
func activateUser(ctx context.Context, env clients.Env, request domain.ActivationRequest) (domain.ActivationOutcome, error) {
	if err := shared.ActivationIdentityMismatch(request); err != nil {
		return domain.ActivationOutcome{}, err
	}
	outcome := shared.StartedActivation(request)
	if !shared.HasClientCLI(env, request.BackendExecutable) {
		outcome.Activation = domain.ActivationManual
		outcome.UserActions = []string{"install Grok Build CLI, then rerun agentplugins update to register and verify the prepared plugin"}
		return outcome, nil
	}
	if request.VerifyOnly {
		return verifyActivation(ctx, env, request, outcome, false)
	}
	entries, err := listPlugins(ctx, env, request.BackendExecutable)
	if err != nil {
		return activationError(outcome, request, err)
	}
	entry, found := findEntry(entries, request.DeclaredName)
	if found && !expectedEntry(entry, request.DeclaredName, request.Delivery.ActivePath) {
		return activationError(outcome, request, fmt.Errorf("%w: Grok plugin %q is registered from a different source", shared.ErrRecognizedNegativeEvidence, request.DeclaredName))
	}
	if found && entry.Status == "disabled" {
		outcome.Activation = domain.ActivationManual
		outcome.UserActions = []string{"enable the managed plugin in Grok Build, then rerun agentplugins update"}
		return outcome, nil
	}
	if found && !request.Replacing && entry.Status == "installed" && entry.Version == request.Plan.DeclaredVersion {
		outcome.Activation, outcome.Verification = domain.ActivationActive, domain.VerificationInstalled
		return outcome, nil
	}
	if found {
		if _, err := shared.RunClientCommand(ctx, env.Runner, "grok Build", request.BackendExecutable, "plugin", "uninstall", "--confirm", request.DeclaredName); err != nil {
			// A failed native command may still have changed its registry.
			outcome.NativeEffect = domain.NativeEffectUncertain
			return activationError(outcome, request, fmt.Errorf("uninstall previous Grok plugin: %w", err))
		}
		if err := verifyAbsent(ctx, env, request.BackendExecutable, request.DeclaredName); err != nil {
			// A successful native removal with unreadable or negative readback
			// may have changed the registration. Never begin a second mutation
			// or accept a stale manual attestation after this point.
			outcome.NativeEffect = domain.NativeEffectUncertain
			return shared.FailedActivation(outcome, "inspect Grok native registry before retrying replacement",
				fmt.Errorf("verify previous Grok plugin removal: %w", err))
		}
	}
	if _, err := shared.RunClientCommand(ctx, env.Runner, "grok Build", request.BackendExecutable, "plugin", "install", "--trust", request.Delivery.ActivePath); err != nil {
		// A failed or unclosed native installation may have partially changed
		// its registry. Never re-probe or assume no external effect.
		outcome.NativeEffect = domain.NativeEffectUncertain
		return activationError(outcome, request, fmt.Errorf("install grok plugin: %w", err))
	}
	return verifyActivation(ctx, env, request, outcome, true)
}

func verifyActivation(ctx context.Context, env clients.Env, request domain.ActivationRequest, outcome domain.ActivationOutcome, installedNow bool) (domain.ActivationOutcome, error) {
	entries, err := listPlugins(ctx, env, request.BackendExecutable)
	if err != nil {
		if installedNow {
			// A previous manual attestation cannot prove this new native install.
			// Unknown readback after a successful mutation has uncertain effect:
			// do not claim Ready or retry native operations in the same session.
			outcome.NativeEffect = domain.NativeEffectUncertain
			return shared.FailedActivation(outcome, "inspect `grok plugin list --json` before retrying installation",
				fmt.Errorf("verify newly installed Grok plugin: %w", shared.CompletionProbeFailure(err)))
		}
		return activationError(outcome, request, err)
	}
	entry, found := findEntry(entries, request.DeclaredName)
	if !found || !expectedEntry(entry, request.DeclaredName, request.Delivery.ActivePath) || entry.Status != "installed" || entry.Version != request.Plan.DeclaredVersion {
		if installedNow {
			// The install command completed, but native evidence contradicts
			// the requested registration; its external effect is unresolved.
			outcome.NativeEffect = domain.NativeEffectUncertain
		}
		return activationError(outcome, request, fmt.Errorf("%w: Grok plugin %q is absent, disabled, foreign, or has a different version", shared.ErrRecognizedNegativeEvidence, request.DeclaredName))
	}
	outcome.Activation, outcome.Verification = domain.ActivationActive, domain.VerificationInstalled
	return outcome, nil
}

func activationError(outcome domain.ActivationOutcome, request domain.ActivationRequest, err error) (domain.ActivationOutcome, error) {
	err = shared.CompletionProbeFailure(err)
	if domain.IsUnclosedClientProbe(err) {
		outcome.NativeEffect = domain.NativeEffectUncertain
		failed, cause := shared.FailedActivation(outcome, "resolve the unclosed Grok process before retrying", err)
		failed.UserActions = []string{"resolve uncertain Grok process cleanup before attempting another installation"}
		return failed, cause
	}
	if errors.Is(err, errUnknownList) {
		if attested, ok := shared.AttestedUnknownVerification(outcome, request); ok {
			return attested, nil
		}
		outcome.Activation = domain.ActivationManual
		outcome.UserActions = []string{"verify that the managed plugin is installed and enabled in Grok Build"}
		return outcome, nil
	}
	return shared.FailedActivation(outcome, "inspect `grok plugin list --json` and rerun agentplugins update", err)
}

func verifyAbsent(ctx context.Context, env clients.Env, executable, name string) error {
	entries, err := listPlugins(ctx, env, executable)
	if err != nil {
		return err
	}
	if _, found := findEntry(entries, name); found {
		return fmt.Errorf("grok plugin %q remains registered", name)
	}
	return nil
}

func (adapter *Adapter) Deactivate(ctx context.Context, env clients.Env, request domain.DeactivationRequest) (domain.DeactivationOutcome, error) {
	if request.TargetContext.Scope == domain.ScopeProject || request.TargetContext.ProjectRoot != "" {
		return deactivateProject(ctx, env, request)
	}
	outcome := shared.StartedDeactivation()
	if request.ExternalUninstalled {
		// External confirmation is necessary when Grok is unavailable, but
		// must not bypass fresh native evidence when it can be queried.
		if request.Confirmed && shared.HasClientCLI(env, request.BackendExecutable) {
			outcome.ArtifactRemovalAllowed = false
			if err := verifyAbsent(ctx, env, request.BackendExecutable, request.DeclaredName); err != nil {
				if domain.IsUnclosedClientProbe(shared.CompletionProbeFailure(err)) {
					outcome.NativeEffect = domain.NativeEffectUncertain
				}
				return outcome, fmt.Errorf("verify externally uninstalled Grok plugin: %w", err)
			}
		}
		outcome.ExternalRemovalComplete = true
		outcome.ArtifactRemovalAllowed = true
		return outcome, nil
	}
	if adapter.RequiresExternalUninstalled(env, request) {
		return shared.RequireExternalUninstall(outcome, false, "uninstall the managed plugin in Grok Build, then rerun remove with --external-uninstalled"), nil
	}
	if !request.Confirmed {
		outcome.UserActions = []string{"agentplugins will uninstall the managed Grok Build plugin automatically"}
		return outcome, nil
	}
	// An unknown native listing is not an absence receipt. Keep the managed artifact
	// until an exact absence or completed owned unregister is verified.
	outcome.ArtifactRemovalAllowed = false
	entries, err := listPlugins(ctx, env, request.BackendExecutable)
	if err != nil {
		return outcome, err
	}
	entry, found := findEntry(entries, request.DeclaredName)
	if !found {
		outcome.ExternalRemovalComplete = true
		outcome.ArtifactRemovalAllowed = true
		return outcome, nil
	}
	if !expectedEntry(entry, request.DeclaredName, request.ManagedArtifactPath) {
		return outcome, fmt.Errorf("grok plugin %q is registered from a different source; refusing to uninstall it", request.DeclaredName)
	}
	if _, err := shared.RunClientCommand(ctx, env.Runner, "grok Build", request.BackendExecutable, "plugin", "uninstall", "--confirm", request.DeclaredName); err != nil {
		outcome.ArtifactRemovalAllowed = false
		outcome.NativeEffect = domain.NativeEffectUncertain
		// A failed native unregister may have partial effects; do not issue
		// another native query or allow managed artifact deletion.
		return outcome, fmt.Errorf("uninstall grok plugin: %w", err)
	}
	if err := verifyAbsent(ctx, env, request.BackendExecutable, request.DeclaredName); err != nil {
		outcome.ArtifactRemovalAllowed = false
		// Unregister has already completed: any failed readback leaves
		// uncertain external state, even if the listing process closed.
		outcome.NativeEffect = domain.NativeEffectUncertain
		return outcome, err
	}
	outcome.ExternalRemovalComplete = true
	outcome.ArtifactRemovalAllowed = true
	return outcome, nil
}
