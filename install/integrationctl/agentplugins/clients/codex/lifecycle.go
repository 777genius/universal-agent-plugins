package codex

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

const codexCLIName = "Codex CLI"

var (
	_ clients.Lifecycle          = (*Adapter)(nil)
	_ clients.AutomaticActivator = (*Adapter)(nil)
	_ clients.ReadOnlyVerifier   = (*Adapter)(nil)
)

// AutomaticallyActivates reports that Activate will drive the Codex CLI.
func (*Adapter) AutomaticallyActivates(env clients.Env, request domain.ActivationRequest) bool {
	return shared.HasClientCLI(env, request.BackendExecutable)
}

// VerifierAvailable reports that an exact Codex listing can be observed.
func (*Adapter) VerifierAvailable(_ domain.DetectedClient, _ domain.DeliveryPlan, backendExecutable string) bool {
	return strings.TrimSpace(backendExecutable) != ""
}

// Activate registers the managed marketplace and plugin through the Codex CLI,
// or leaves a manual follow-up when that CLI is missing.
func (*Adapter) Activate(ctx context.Context, env clients.Env, request domain.ActivationRequest) (domain.ActivationOutcome, error) {
	if err := shared.ActivationIdentityMismatch(request); err != nil {
		return domain.ActivationOutcome{}, err
	}
	outcome := shared.StartedActivation(request)
	if !shared.HasClientCLI(env, request.BackendExecutable) {
		outcome.Activation = domain.ActivationManual
		outcome.UserActions = append(outcome.UserActions, "install the prepared plugin in Codex Plugins, then verify it appears in Plugins > Personal")
		outcome.LocalActions = append(outcome.LocalActions, fmt.Sprintf("in Codex, install %s from %s, then verify it appears in Plugins > Personal", request.DeclaredName, request.Delivery.ActivePath))
		return outcome, nil
	}
	return completeCodexActivation(ctx, env, request, outcome)
}

func completeCodexActivation(ctx context.Context, env clients.Env, request domain.ActivationRequest, outcome domain.ActivationOutcome) (domain.ActivationOutcome, error) {
	var err error
	if request.VerifyOnly {
		err = verifyCodexPlugin(ctx, env, request)
	} else {
		err = activateCodexPlugin(ctx, env, request)
	}
	if err == nil {
		outcome.Activation = domain.ActivationActive
		outcome.Verification = domain.VerificationInstalled
		return outcome, nil
	}
	if errors.Is(err, ErrListContractUnknown) {
		return manualCodexVerification(outcome, request), nil
	}
	next := fmt.Sprintf("verify with `%s plugin list --json`", request.BackendExecutable)
	if !request.VerifyOnly {
		next = fmt.Sprintf("run Codex activation again for the prepared package at %s, then verify with `%s plugin list --json`", request.Delivery.ActivePath, request.BackendExecutable)
	}
	return shared.FailedActivation(outcome, next, err)
}

func activateCodexPlugin(ctx context.Context, env clients.Env, request domain.ActivationRequest) error {
	marketplace := shared.ManagedMarketplaceName(request.Plan.PhysicalArtifactID)
	if err := registerCodexMarketplace(ctx, env, request, marketplace); err != nil {
		return err
	}
	pluginSpec := request.DeclaredName + "@" + marketplace
	if _, err := runCodex(ctx, env, request.BackendExecutable, "plugin", "add", pluginSpec, "--json"); err != nil {
		if !request.Replacing {
			_, _ = runCodex(ctx, env, request.BackendExecutable, "plugin", "marketplace", "remove", marketplace, "--json")
		}
		return fmt.Errorf("activate Codex plugin: %w", err)
	}
	return verifyCodexPlugin(ctx, env, request)
}

func registerCodexMarketplace(ctx context.Context, env clients.Env, request domain.ActivationRequest, marketplace string) error {
	if request.Replacing {
		if _, err := runCodex(ctx, env, request.BackendExecutable, "plugin", "marketplace", "update", marketplace, "--json"); err != nil {
			if _, fallbackErr := runCodex(ctx, env, request.BackendExecutable, "plugin", "marketplace", "add", request.Delivery.ActivePath, "--json"); fallbackErr != nil {
				return fmt.Errorf("refresh Codex marketplace: %v; fallback registration: %w", err, fallbackErr)
			}
		}
		return nil
	}
	if _, err := runCodex(ctx, env, request.BackendExecutable, "plugin", "marketplace", "add", request.Delivery.ActivePath, "--json"); err != nil {
		return fmt.Errorf("register Codex marketplace: %w", err)
	}
	return nil
}

func verifyCodexPlugin(ctx context.Context, env clients.Env, request domain.ActivationRequest) error {
	marketplace := shared.ManagedMarketplaceName(request.Plan.PhysicalArtifactID)
	pluginSpec := request.DeclaredName + "@" + marketplace
	listed, err := runCodex(ctx, env, request.BackendExecutable, "plugin", "list", "--json")
	if err != nil {
		return fmt.Errorf("verify Codex plugin listing: %w", err)
	}
	switch PluginStatusFromList(listed.Stdout, request.DeclaredName, marketplace) {
	case StatusInstalled:
		return nil
	case StatusAbsent:
		return fmt.Errorf("%w: verify Codex plugin listing: %s is not installed and enabled", shared.ErrRecognizedNegativeEvidence, pluginSpec)
	default:
		return fmt.Errorf("%w: verify Codex plugin listing", ErrListContractUnknown)
	}
}

func manualCodexVerification(outcome domain.ActivationOutcome, request domain.ActivationRequest) domain.ActivationOutcome {
	if attested, ok := shared.AttestedUnknownVerification(outcome, request); ok {
		return attested
	}
	outcome.Activation = domain.ActivationManual
	outcome.UserActions = []string{"confirm the managed plugin is installed and enabled in Codex"}
	outcome.LocalActions = []string{fmt.Sprintf("the `%s plugin list --json` output contract was not recognized; inspect %s manually", request.BackendExecutable, request.DeclaredName)}
	return outcome
}

func runCodex(ctx context.Context, env clients.Env, executable string, args ...string) (legacyports.CommandResult, error) {
	return shared.RunClientCommand(ctx, env.Runner, codexCLIName, executable, args...)
}
