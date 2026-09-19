package claude

import (
	"context"
	"fmt"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

var (
	_ clients.Lifecycle             = (*Adapter)(nil)
	_ clients.ActivationPreflighter = (*Adapter)(nil)
	_ clients.AutomaticActivator    = (*Adapter)(nil)
	_ clients.ReadOnlyVerifier      = (*Adapter)(nil)
)

// AutomaticallyActivates reports that Activate will drive the Claude Code CLI.
func (*Adapter) AutomaticallyActivates(env clients.Env, request domain.ActivationRequest) bool {
	return shared.HasClientCLI(env, request.BackendExecutable)
}

// VerifierAvailable reports that an exact Claude listing can be observed.
func (*Adapter) VerifierAvailable(_ domain.DetectedClient, _ domain.DeliveryPlan, backendExecutable string) bool {
	return shared.HasListingCLI(backendExecutable)
}

// PreflightActivation rejects a request whose Claude path layout cannot be
// probed through the trusted CLI.
func (*Adapter) PreflightActivation(_ clients.Env, request domain.ActivationRequest) error {
	_, err := PrepareClaudeActivationProbe(request)
	return err
}

// Activate verifies the managed @skills-dir plugin through the trusted Claude
// Code CLI. The official layout is delivered by the transaction kernel.
func (*Adapter) Activate(ctx context.Context, env clients.Env, request domain.ActivationRequest) (domain.ActivationOutcome, error) {
	if err := shared.ActivationIdentityMismatch(request); err != nil {
		return domain.ActivationOutcome{}, err
	}
	if err := (*Adapter)(nil).PreflightActivation(env, request); err != nil {
		return domain.ActivationOutcome{}, err
	}
	outcome := shared.StartedActivation(request)
	if !shared.HasClientCLI(env, request.BackendExecutable) {
		return shared.FailedActivation(outcome, "install Claude Code CLI and retry exact @skills-dir verification", fmt.Errorf("trusted Claude Code CLI is required"))
	}
	if err := verifyClaudePlugin(ctx, env, request); err != nil {
		return shared.FailedActivation(outcome, fmt.Sprintf("verify with `%s plugin list --json`", request.BackendExecutable), err)
	}
	outcome.Activation = domain.ActivationActive
	outcome.Verification = domain.VerificationInstalled
	return outcome, nil
}

// Deactivate verifies the exact managed identity before the kernel removes the
// owned directory. Previews stay inert.
func (*Adapter) Deactivate(ctx context.Context, env clients.Env, request domain.DeactivationRequest) (domain.DeactivationOutcome, error) {
	outcome := shared.StartedDeactivation()
	if !request.Confirmed {
		outcome.UserActions = append(outcome.UserActions, "agentplugins will verify and remove its managed Claude Code @skills-dir plugin")
		return outcome, nil
	}
	if !shared.HasClientCLI(env, request.BackendExecutable) {
		return outcome, fmt.Errorf("trusted Claude Code CLI is required for exact removal verification")
	}
	listed, err := runClaudeList(ctx, env, request.BackendExecutable, request.Client.ConfigRoot, request.ManagedArtifactPath)
	if err != nil {
		return outcome, fmt.Errorf("verify Claude Code plugin before removal: %w", err)
	}
	return finishClaudeDeactivation(outcome, listed.Stdout, request)
}

func finishClaudeDeactivation(outcome domain.DeactivationOutcome, stdout []byte, request domain.DeactivationRequest) (domain.DeactivationOutcome, error) {
	switch PluginStatusFromList(stdout, request.DeclaredName, request.ManagedArtifactPath) {
	case StatusInstalled:
	case StatusAbsent:
		if request.CurrentActivation == domain.ActivationActive {
			return outcome, fmt.Errorf("%w: managed Claude Code plugin is absent before removal", shared.ErrRecognizedNegativeEvidence)
		}
	default:
		return outcome, fmt.Errorf("the Claude Code plugin identity is not exact before removal")
	}
	outcome.ExternalRemovalComplete = true
	return outcome, nil
}

func verifyClaudePlugin(ctx context.Context, env clients.Env, request domain.ActivationRequest) error {
	probe, err := PrepareClaudeActivationProbe(request)
	if err != nil {
		return fmt.Errorf("prepare Claude Code plugin listing: %w", err)
	}
	listed, err := runPreparedClaudeList(ctx, env, probe.Command)
	if err != nil {
		return fmt.Errorf("verify Claude Code plugin listing: %w", err)
	}
	switch PluginStatusFromList(listed.Stdout, request.DeclaredName, probe.ActivePath) {
	case StatusInstalled:
		return nil
	case StatusAbsent, StatusCollision:
		return fmt.Errorf("%w: verify Claude Code plugin listing: %s@skills-dir is not enabled at the managed path", shared.ErrRecognizedNegativeEvidence, request.DeclaredName)
	default:
		return fmt.Errorf("the Claude Code plugin list output is not recognized")
	}
}

func runClaudeList(ctx context.Context, env clients.Env, executable, configRoot, activePath string) (legacyports.CommandResult, error) {
	if env.Runner == nil {
		return legacyports.CommandResult{}, fmt.Errorf("the Claude Code CLI runner is unavailable")
	}
	command, err := ClaudeListCommand(executable, configRoot, activePath)
	if err != nil {
		return legacyports.CommandResult{}, err
	}
	return runPreparedClaudeList(ctx, env, command)
}

func runPreparedClaudeList(ctx context.Context, env clients.Env, command legacyports.Command) (legacyports.CommandResult, error) {
	result, err := RunClaudeListCommand(ctx, env.Runner, command)
	if err != nil {
		return result, fmt.Errorf("start Claude Code CLI: %w", err)
	}
	if result.ExitCode != 0 {
		return result, fmt.Errorf("the Claude Code CLI command failed with exit code %d", result.ExitCode)
	}
	return result, nil
}
