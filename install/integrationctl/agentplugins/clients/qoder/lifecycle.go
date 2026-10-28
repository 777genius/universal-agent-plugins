package qoder

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
	if projectRequested(request.Plan) {
		return false
	}
	return shared.HasClientCLI(env, request.BackendExecutable)
}

func (*Adapter) VerifierAvailable(client domain.DetectedClient, plan domain.DeliveryPlan, backendExecutable string) bool {
	if projectRequested(plan) {
		return validateProjectPlan(client, plan) == nil
	}
	return shared.HasListingCLI(backendExecutable)
}

func (*Adapter) Activate(ctx context.Context, env clients.Env, request domain.ActivationRequest) (domain.ActivationOutcome, error) {
	if projectRequested(request.Plan) {
		return activateProject(ctx, env, request)
	}
	if err := shared.ActivationIdentityMismatch(request); err != nil {
		return domain.ActivationOutcome{}, err
	}
	if err := validateProfile(request.Client.ConfigRoot, request.Plan.NativeRegistryRoot); err != nil {
		return domain.ActivationOutcome{}, err
	}
	// Stored bindings predate every plan gate, so refuse rather than address
	// an id that could belong to a different plugin.
	if !qualifiedPluginName(request.DeclaredName) {
		return domain.ActivationOutcome{}, fmt.Errorf("plugin name %q cannot form an unambiguous Qoder plugin id", request.DeclaredName)
	}
	outcome := shared.StartedActivation(request)
	if !shared.HasClientCLI(env, request.BackendExecutable) {
		outcome.Activation = domain.ActivationManual
		outcome.UserActions = []string{"install Qoder CLI, then rerun agentplugins update to register and verify the prepared plugin"}
		return outcome, nil
	}
	if request.VerifyOnly {
		return verifyActivation(ctx, env, request, outcome)
	}
	entries, err := listPlugins(ctx, env, request.Client.ConfigRoot, request.BackendExecutable)
	if err != nil {
		return activationError(outcome, request, err)
	}
	if findForeign(entries, request.DeclaredName) {
		return activationError(outcome, request, fmt.Errorf("%w: Qoder plugin %q is registered from a different source", shared.ErrRecognizedNegativeEvidence, request.DeclaredName))
	}
	entry, found := findEntry(entries, request.DeclaredName)
	if found && !request.Replacing && versionMatches(entry, request.Plan.DeclaredVersion) {
		outcome.Activation, outcome.Verification = domain.ActivationActive, domain.VerificationInstalled
		return outcome, nil
	}
	if found && !request.Replacing && entry.Version == "" {
		// Registration is visible but the listed version is absent, so the
		// delivered package is unproven. Like an unrecognized listing, this
		// never claims success and never mutates Qoder on its own.
		return unprovenVersion(outcome, request)
	}
	if found {
		if _, err := runQoder(ctx, env, request.Client.ConfigRoot, request.BackendExecutable, "plugins", "uninstall", request.DeclaredName, "--scope", "user"); err != nil {
			return activationError(outcome, request, fmt.Errorf("uninstall previous Qoder plugin: %w", err))
		}
		if err := verifyAbsent(ctx, env, request.Client.ConfigRoot, request.BackendExecutable, request.DeclaredName); err != nil {
			return activationError(outcome, request, fmt.Errorf("verify previous Qoder plugin removal: %w", err))
		}
	}
	_, installErr := runQoder(ctx, env, request.Client.ConfigRoot, request.BackendExecutable, "plugins", "install", request.Delivery.ActivePath, "--scope", "user")
	verified, verifyErr := verifyActivation(ctx, env, request, outcome)
	if installErr != nil {
		// A same-version update may change package content, so a stale listing
		// cannot prove a failed install delivered the new package bytes.
		return shared.FailedActivation(outcome, "inspect `qoder plugins list --json` and retry installation", errors.Join(fmt.Errorf("install qoder plugin: %w", installErr), verifyErr))
	}
	if verifyErr == nil && verified.Verification == domain.VerificationInstalled {
		return verified, nil
	}
	return verified, verifyErr
}

func verifyActivation(ctx context.Context, env clients.Env, request domain.ActivationRequest, outcome domain.ActivationOutcome) (domain.ActivationOutcome, error) {
	entries, err := listPlugins(ctx, env, request.Client.ConfigRoot, request.BackendExecutable)
	if err != nil {
		return activationError(outcome, request, err)
	}
	// A foreign claim on the name outranks any version evidence: attestation
	// must never accept a registration that is not ours to verify.
	if findForeign(entries, request.DeclaredName) {
		return activationError(outcome, request, fmt.Errorf("%w: Qoder plugin %q is registered from a different source", shared.ErrRecognizedNegativeEvidence, request.DeclaredName))
	}
	entry, found := findEntry(entries, request.DeclaredName)
	if !found || (entry.Version != "" && !versionMatches(entry, request.Plan.DeclaredVersion)) {
		return activationError(outcome, request, fmt.Errorf("%w: Qoder plugin %q is absent or has a different version", shared.ErrRecognizedNegativeEvidence, request.DeclaredName))
	}
	if !versionMatches(entry, request.Plan.DeclaredVersion) {
		return unprovenVersion(outcome, request)
	}
	outcome.Activation, outcome.Verification = domain.ActivationActive, domain.VerificationInstalled
	return outcome, nil
}

// unprovenVersion reports a registered plugin whose listing carries no
// version member: the CLI cannot prove the delivered version, so only the
// operator's attestation can, and until then activation stays manual.
func unprovenVersion(outcome domain.ActivationOutcome, request domain.ActivationRequest) (domain.ActivationOutcome, error) {
	if attested, ok := shared.AttestedUnknownVerification(outcome, request); ok {
		return attested, nil
	}
	outcome.Activation = domain.ActivationManual
	outcome.UserActions = []string{"confirm the registered Qoder plugin is version " + request.Plan.DeclaredVersion + ", then run /plugins reload or restart Qoder"}
	return outcome, nil
}

func activationError(outcome domain.ActivationOutcome, request domain.ActivationRequest, err error) (domain.ActivationOutcome, error) {
	if errors.Is(err, errUnknownList) {
		if attested, ok := shared.AttestedUnknownVerification(outcome, request); ok {
			return attested, nil
		}
		outcome.Activation = domain.ActivationManual
		outcome.UserActions = []string{"verify that the managed plugin is installed in Qoder, then run /plugins reload or restart Qoder"}
		return outcome, nil
	}
	return shared.FailedActivation(outcome, "inspect `qoder plugins list --json` and rerun agentplugins update", err)
}

func verifyAbsent(ctx context.Context, env clients.Env, root, executable, name string) error {
	entries, err := listPlugins(ctx, env, root, executable)
	if err != nil {
		return err
	}
	if _, found := findEntry(entries, name); found {
		return fmt.Errorf("qoder plugin %q remains registered", name)
	}
	return nil
}

func (*Adapter) Deactivate(ctx context.Context, env clients.Env, request domain.DeactivationRequest) (domain.DeactivationOutcome, error) {
	if request.TargetContext.Scope == domain.ScopeProject || request.TargetContext.ProjectRoot != "" {
		return deactivateProject(ctx, request)
	}
	outcome := shared.StartedDeactivation()
	if request.ExternalUninstalled {
		outcome.ExternalRemovalComplete = true
		return outcome, nil
	}
	if !qualifiedPluginName(request.DeclaredName) {
		return outcome, fmt.Errorf("plugin name %q cannot form an unambiguous Qoder plugin id", request.DeclaredName)
	}
	if err := validateProfile(request.Client.ConfigRoot, ""); err != nil {
		return outcome, err
	}
	if !shared.HasClientCLI(env, request.BackendExecutable) {
		return shared.RequireExternalUninstall(outcome, false, "uninstall the managed plugin in Qoder, then rerun remove with --external-uninstalled"), nil
	}
	if !request.Confirmed {
		outcome.UserActions = []string{"agentplugins will uninstall the managed Qoder plugin automatically"}
		return outcome, nil
	}
	entries, err := listPlugins(ctx, env, request.Client.ConfigRoot, request.BackendExecutable)
	if err != nil {
		return outcome, err
	}
	if _, found := findEntry(entries, request.DeclaredName); !found {
		outcome.ExternalRemovalComplete = true
		return outcome, nil
	}
	if findForeign(entries, request.DeclaredName) {
		return outcome, fmt.Errorf("qoder plugin %q is registered from a different source; refusing to uninstall it", request.DeclaredName)
	}
	_, uninstallErr := runQoder(ctx, env, request.Client.ConfigRoot, request.BackendExecutable, "plugins", "uninstall", request.DeclaredName, "--scope", "user")
	if err := verifyAbsent(ctx, env, request.Client.ConfigRoot, request.BackendExecutable, request.DeclaredName); err != nil {
		if uninstallErr != nil {
			return outcome, errors.Join(fmt.Errorf("uninstall qoder plugin: %w", uninstallErr), fmt.Errorf("verification: %w", err))
		}
		return outcome, err
	}
	outcome.ExternalRemovalComplete = true
	return outcome, nil
}
