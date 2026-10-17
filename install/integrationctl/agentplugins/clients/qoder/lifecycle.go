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
	return shared.HasClientCLI(env, request.BackendExecutable)
}

func (*Adapter) VerifierAvailable(_ domain.DetectedClient, _ domain.DeliveryPlan, backendExecutable string) bool {
	return shared.HasListingCLI(backendExecutable)
}

func (*Adapter) Activate(ctx context.Context, env clients.Env, request domain.ActivationRequest) (domain.ActivationOutcome, error) {
	if err := shared.ActivationIdentityMismatch(request); err != nil {
		return domain.ActivationOutcome{}, err
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
	entries, err := listPlugins(ctx, env, request.BackendExecutable)
	if err != nil {
		return activationError(outcome, request, err)
	}
	if _, foreign := findForeign(entries, request.DeclaredName); foreign {
		return activationError(outcome, request, fmt.Errorf("%w: Qoder plugin %q is registered from a different source", shared.ErrRecognizedNegativeEvidence, request.DeclaredName))
	}
	entry, found := findEntry(entries, request.DeclaredName)
	if found && !request.Replacing && versionMatches(entry, request.Plan.DeclaredVersion) {
		outcome.Activation, outcome.Verification = domain.ActivationActive, domain.VerificationInstalled
		return outcome, nil
	}
	if found {
		if _, err := shared.RunClientCommand(ctx, env.Runner, "Qoder CLI", request.BackendExecutable, "plugins", "uninstall", request.DeclaredName); err != nil {
			return activationError(outcome, request, fmt.Errorf("uninstall previous Qoder plugin: %w", err))
		}
		if err := verifyAbsent(ctx, env, request.BackendExecutable, request.DeclaredName); err != nil {
			return activationError(outcome, request, fmt.Errorf("verify previous Qoder plugin removal: %w", err))
		}
	}
	_, installErr := shared.RunClientCommand(ctx, env.Runner, "Qoder CLI", request.BackendExecutable, "plugins", "install", request.Delivery.ActivePath, "--scope", "user")
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
	entries, err := listPlugins(ctx, env, request.BackendExecutable)
	if err != nil {
		return activationError(outcome, request, err)
	}
	entry, found := findEntry(entries, request.DeclaredName)
	if !found || !versionMatches(entry, request.Plan.DeclaredVersion) {
		return activationError(outcome, request, fmt.Errorf("%w: Qoder plugin %q is absent or has a different version", shared.ErrRecognizedNegativeEvidence, request.DeclaredName))
	}
	if _, foreign := findForeign(entries, request.DeclaredName); foreign {
		return activationError(outcome, request, fmt.Errorf("%w: Qoder plugin %q is registered from a different source", shared.ErrRecognizedNegativeEvidence, request.DeclaredName))
	}
	outcome.Activation, outcome.Verification = domain.ActivationActive, domain.VerificationInstalled
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

func verifyAbsent(ctx context.Context, env clients.Env, executable, name string) error {
	entries, err := listPlugins(ctx, env, executable)
	if err != nil {
		return err
	}
	if _, found := findEntry(entries, name); found {
		return fmt.Errorf("qoder plugin %q remains registered", name)
	}
	return nil
}

func (*Adapter) Deactivate(ctx context.Context, env clients.Env, request domain.DeactivationRequest) (domain.DeactivationOutcome, error) {
	outcome := shared.StartedDeactivation()
	if request.ExternalUninstalled {
		outcome.ExternalRemovalComplete = true
		return outcome, nil
	}
	if !shared.HasClientCLI(env, request.BackendExecutable) {
		return shared.RequireExternalUninstall(outcome, false, "uninstall the managed plugin in Qoder, then rerun remove with --external-uninstalled"), nil
	}
	if !request.Confirmed {
		outcome.UserActions = []string{"agentplugins will uninstall the managed Qoder plugin automatically"}
		return outcome, nil
	}
	entries, err := listPlugins(ctx, env, request.BackendExecutable)
	if err != nil {
		return outcome, err
	}
	if _, found := findEntry(entries, request.DeclaredName); !found {
		outcome.ExternalRemovalComplete = true
		return outcome, nil
	}
	if _, foreign := findForeign(entries, request.DeclaredName); foreign {
		return outcome, fmt.Errorf("qoder plugin %q is registered from a different source; refusing to uninstall it", request.DeclaredName)
	}
	_, uninstallErr := shared.RunClientCommand(ctx, env.Runner, "Qoder CLI", request.BackendExecutable, "plugins", "uninstall", request.DeclaredName)
	if err := verifyAbsent(ctx, env, request.BackendExecutable, request.DeclaredName); err != nil {
		if uninstallErr != nil {
			return outcome, errors.Join(fmt.Errorf("uninstall qoder plugin: %w", uninstallErr), fmt.Errorf("verification: %w", err))
		}
		return outcome, err
	}
	outcome.ExternalRemovalComplete = true
	return outcome, nil
}
