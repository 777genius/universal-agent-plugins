package shared

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

const copilotCLIName = "GitHub Copilot CLI"

// CompleteCopilotActivation runs verify-or-install against the Copilot CLI.
// Callers handle the no-CLI manual path themselves because Copilot and VS Code
// differ only in those operator messages.
func CompleteCopilotActivation(ctx context.Context, env clients.Env, request domain.ActivationRequest, outcome domain.ActivationOutcome) (domain.ActivationOutcome, error) {
	var err error
	if request.VerifyOnly {
		err = VerifyCopilotPlugin(ctx, env, request)
	} else {
		err = ActivateCopilotPlugin(ctx, env, request)
	}
	if err == nil {
		outcome.Activation = domain.ActivationActive
		outcome.Verification = domain.VerificationInstalled
		return outcome, nil
	}
	if errors.Is(err, ErrCopilotListContractUnknown) {
		return ManualCopilotVerification(outcome, request), nil
	}
	next := fmt.Sprintf("verify with `%s plugin list`", request.BackendExecutable)
	if !request.VerifyOnly {
		next = fmt.Sprintf("rerun add for the prepared package at %s, then verify with `%s plugin list`", request.Delivery.ActivePath, request.BackendExecutable)
	}
	return FailedActivation(outcome, next, err)
}

// CompleteCopilotDeactivation is the shared Copilot / VS Code removal path.
// unavailableAction is the no-CLI operator instruction, which is the only
// wording the two clients do not share.
func CompleteCopilotDeactivation(ctx context.Context, env clients.Env, request domain.DeactivationRequest, unavailableAction string) (domain.DeactivationOutcome, error) {
	outcome := StartedDeactivation()
	if request.ExternalUninstalled {
		outcome.ExternalRemovalComplete = true
		return outcome, nil
	}
	if !HasClientCLI(env, request.BackendExecutable) {
		return RequireExternalUninstall(outcome, false, unavailableAction), nil
	}
	if request.CurrentActivation != domain.ActivationActive {
		action := fmt.Sprintf("verify `%s@%s` is absent from GitHub Copilot CLI, then rerun remove with `--external-uninstalled`", request.DeclaredName, ManagedMarketplaceName(request.PhysicalArtifactID))
		return RequireExternalUninstall(outcome, false, action), nil
	}
	if !request.Confirmed {
		outcome.UserActions = append(outcome.UserActions, "agentplugins will uninstall the plugin from GitHub Copilot CLI and VS Code automatically")
		return outcome, nil
	}
	if err := DeactivateCopilotPlugin(ctx, env, request); err != nil {
		return outcome, err
	}
	outcome.ExternalRemovalComplete = true
	return outcome, nil
}

// CopilotAutomaticallyActivates reports that Activate will drive the shared
// Copilot CLI for this request. Copilot and VS Code share this backend.
func CopilotAutomaticallyActivates(env clients.Env, request domain.ActivationRequest) bool {
	return HasClientCLI(env, request.BackendExecutable)
}

// CopilotVerifierAvailable reports that a Copilot-family client can be
// observed with an exact read-only listing.
func CopilotVerifierAvailable(backendExecutable string) bool {
	return strings.TrimSpace(backendExecutable) != ""
}

// ActivateCopilotPlugin registers the managed marketplace and installs the
// plugin through the Copilot CLI, then verifies the listing.
func ActivateCopilotPlugin(ctx context.Context, env clients.Env, request domain.ActivationRequest) error {
	marketplace := ManagedMarketplaceName(request.Plan.PhysicalArtifactID)
	updated, err := registerCopilotMarketplace(ctx, env, request, marketplace)
	if err != nil {
		return err
	}
	if err := installCopilotPlugin(ctx, env, request, marketplace, updated); err != nil {
		return err
	}
	return VerifyCopilotPlugin(ctx, env, request)
}

func registerCopilotMarketplace(ctx context.Context, env clients.Env, request domain.ActivationRequest, marketplace string) (bool, error) {
	if request.Replacing {
		if err := runCopilot(ctx, env, request.BackendExecutable, "plugin", "marketplace", "update", marketplace); err != nil {
			if fallbackErr := runCopilot(ctx, env, request.BackendExecutable, "plugin", "marketplace", "add", request.Delivery.ActivePath); fallbackErr != nil {
				return false, fmt.Errorf("refresh managed Copilot marketplace: %v; fallback registration: %w", err, fallbackErr)
			}
		}
		return runCopilot(ctx, env, request.BackendExecutable, "plugin", "update", request.DeclaredName+"@"+marketplace) == nil, nil
	}
	if err := runCopilot(ctx, env, request.BackendExecutable, "plugin", "marketplace", "add", request.Delivery.ActivePath); err != nil {
		if fallbackErr := runCopilot(ctx, env, request.BackendExecutable, "plugin", "marketplace", "update", marketplace); fallbackErr != nil {
			return false, fmt.Errorf("register managed Copilot marketplace: %v; fallback refresh: %w", err, fallbackErr)
		}
	}
	return false, nil
}

func installCopilotPlugin(ctx context.Context, env clients.Env, request domain.ActivationRequest, marketplace string, updated bool) error {
	if updated {
		return nil
	}
	pluginSpec := request.DeclaredName + "@" + marketplace
	if err := runCopilot(ctx, env, request.BackendExecutable, "plugin", "install", pluginSpec); err != nil {
		if !request.Replacing {
			_ = runCopilot(ctx, env, request.BackendExecutable, "plugin", "marketplace", "remove", marketplace)
		}
		return err
	}
	return nil
}

// VerifyCopilotPlugin reads the Copilot listing and requires an exact enabled
// identity at the managed path.
func VerifyCopilotPlugin(ctx context.Context, env clients.Env, request domain.ActivationRequest) error {
	pluginSpec := request.DeclaredName + "@" + ManagedMarketplaceName(request.Plan.PhysicalArtifactID)
	listed, err := runCopilotResult(ctx, env, request.BackendExecutable, "plugin", "list")
	if err != nil {
		return fmt.Errorf("verify Copilot plugin listing: %w", err)
	}
	switch CopilotPluginStatus(listed.Stdout, pluginSpec, MarketplaceVersion(request.Plan.DeclaredVersion), request.Delivery.ActivePath) {
	case CopilotStatusInstalled:
		return nil
	case CopilotStatusAbsent:
		return fmt.Errorf("%w: verify Copilot plugin listing: %s is not listed", ErrRecognizedNegativeEvidence, pluginSpec)
	default:
		return fmt.Errorf("%w: verify Copilot plugin listing", ErrCopilotListContractUnknown)
	}
}

// DeactivateCopilotPlugin uninstalls the plugin and removes the managed
// marketplace. Already-absent is success.
func DeactivateCopilotPlugin(ctx context.Context, env clients.Env, request domain.DeactivationRequest) error {
	if strings.TrimSpace(request.PhysicalArtifactID) == "" {
		return fmt.Errorf("managed Copilot marketplace identity is missing")
	}
	marketplace := ManagedMarketplaceName(request.PhysicalArtifactID)
	uninstall, err := runCopilotResult(ctx, env, request.BackendExecutable, "plugin", "uninstall", request.DeclaredName+"@"+marketplace)
	if err != nil && !CommandOutputContains(uninstall, "is not installed") {
		return err
	}
	remove, err := runCopilotResult(ctx, env, request.BackendExecutable, "plugin", "marketplace", "remove", marketplace)
	if err != nil && !CommandOutputContains(remove, "is not registered") {
		return err
	}
	return nil
}

// ManualCopilotVerification is the unknown-listing fallback Copilot and VS Code
// share. Attested requests become active; otherwise the operator inspects.
func ManualCopilotVerification(outcome domain.ActivationOutcome, request domain.ActivationRequest) domain.ActivationOutcome {
	if attested, ok := AttestedUnknownVerification(outcome, request); ok {
		return attested
	}
	outcome.Activation = domain.ActivationManual
	outcome.UserActions = []string{"confirm the managed plugin is installed and enabled in GitHub Copilot"}
	outcome.LocalActions = []string{fmt.Sprintf("the `%s plugin list` output contract was not recognized; inspect %s manually", request.BackendExecutable, request.DeclaredName)}
	return outcome
}

func runCopilot(ctx context.Context, env clients.Env, executable string, args ...string) error {
	_, err := runCopilotResult(ctx, env, executable, args...)
	return err
}

func runCopilotResult(ctx context.Context, env clients.Env, executable string, args ...string) (legacyports.CommandResult, error) {
	return RunClientCommand(ctx, env.Runner, copilotCLIName, executable, args...)
}
