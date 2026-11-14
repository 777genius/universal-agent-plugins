package vscode

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

// AutomaticallyActivates reports that Activate will drive the shared Copilot
// CLI that VS Code uses as its plugin backend.
func (*Adapter) AutomaticallyActivates(env clients.Env, request domain.ActivationRequest) bool {
	return shared.CopilotAutomaticallyActivates(env, request)
}

// VerifierAvailable reports that an exact Copilot-family listing can be
// observed.
func (*Adapter) VerifierAvailable(_ domain.DetectedClient, _ domain.DeliveryPlan, backendExecutable string) bool {
	return shared.CopilotVerifierAvailable(backendExecutable)
}

// Activate installs through the shared Copilot CLI, or leaves a VS Code
// settings follow-up when that CLI is missing.
func (*Adapter) Activate(ctx context.Context, env clients.Env, request domain.ActivationRequest) (domain.ActivationOutcome, error) {
	if err := shared.ActivationIdentityMismatch(request); err != nil {
		return domain.ActivationOutcome{}, err
	}
	outcome := shared.StartedActivation(request)
	if !shared.HasClientCLI(env, request.BackendExecutable) {
		outcome.Activation = domain.ActivationManual
		outcome.UserActions = append(outcome.UserActions, "register the prepared local plugin in VS Code")
		outcome.LocalActions = append(outcome.LocalActions, fmt.Sprintf(
			"VS Code: add %q to the `chat.pluginLocations` setting with value `true`, then reload VS Code and verify %s appears in the Plugins view",
			request.Delivery.ActivePath,
			request.DeclaredName,
		))
		return outcome, nil
	}
	return shared.CompleteCopilotActivation(ctx, env, request, outcome)
}

// Deactivate uninstalls through the shared Copilot CLI after confirmation.
func (*Adapter) Deactivate(ctx context.Context, env clients.Env, request domain.DeactivationRequest) (domain.DeactivationOutcome, error) {
	return shared.CompleteCopilotDeactivation(ctx, env, request, "remove the plugin in VS Code, then rerun remove with `--external-uninstalled`")
}
