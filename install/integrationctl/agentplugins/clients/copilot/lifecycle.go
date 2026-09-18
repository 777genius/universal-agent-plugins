package copilot

import (
	"context"
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

// AutomaticallyActivates reports that Activate will drive the Copilot CLI.
func (*Adapter) AutomaticallyActivates(env clients.Env, request domain.ActivationRequest) bool {
	return shared.CopilotAutomaticallyActivates(env, request)
}

// VerifierAvailable reports that an exact Copilot listing can be observed.
func (*Adapter) VerifierAvailable(_ domain.DetectedClient, _ domain.DeliveryPlan, backendExecutable string) bool {
	return shared.CopilotVerifierAvailable(backendExecutable)
}

// Activate installs through the Copilot CLI, or leaves a manual follow-up when
// that CLI is missing.
func (*Adapter) Activate(ctx context.Context, env clients.Env, request domain.ActivationRequest) (domain.ActivationOutcome, error) {
	if err := shared.ActivationIdentityMismatch(request); err != nil {
		return domain.ActivationOutcome{}, err
	}
	outcome := shared.StartedActivation(request)
	if !shared.HasClientCLI(env, request.BackendExecutable) {
		outcome.Activation = domain.ActivationManual
		outcome.UserActions = append(outcome.UserActions, "install GitHub Copilot CLI, then rerun `agentplugins update` for this plugin")
		outcome.LocalActions = append(outcome.LocalActions, fmt.Sprintf("install GitHub Copilot CLI, rerun add for the prepared package at %s, then verify with `copilot plugin list`", request.Delivery.ActivePath))
		return outcome, nil
	}
	return shared.CompleteCopilotActivation(ctx, env, request, outcome)
}

// Deactivate uninstalls through the Copilot CLI after confirmation.
func (*Adapter) Deactivate(ctx context.Context, env clients.Env, request domain.DeactivationRequest) (domain.DeactivationOutcome, error) {
	action := fmt.Sprintf("run `copilot plugin uninstall %s`, then rerun remove with `--external-uninstalled`", request.DeclaredName)
	return shared.CompleteCopilotDeactivation(ctx, env, request, action)
}
