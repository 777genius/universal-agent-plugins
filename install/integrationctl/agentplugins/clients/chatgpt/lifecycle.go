package chatgpt

import (
	"context"
	"fmt"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

var (
	_ clients.Lifecycle             = (*Adapter)(nil)
	_ clients.ActivationPreflighter = (*Adapter)(nil)
)

// PreflightActivation rejects a prepare request that has not presented a
// validated personal mapping. Other intents have nothing extra to check.
func (*Adapter) PreflightActivation(_ clients.Env, request domain.ActivationRequest) error {
	if request.Plan.InstallIntent != domain.InstallIntentPrepare {
		return nil
	}
	if request.Plan.Scope != domain.ScopeUser || !request.Plan.PersonalChatGPTPreparation {
		return fmt.Errorf("ChatGPT preparation requires validated personal mapping")
	}
	return nil
}

// Activate either records a prepared personal mapping or leaves ChatGPT as a
// remote manual install. There is no managed executable.
func (*Adapter) Activate(_ context.Context, _ clients.Env, request domain.ActivationRequest) (domain.ActivationOutcome, error) {
	if err := shared.ActivationIdentityMismatch(request); err != nil {
		return domain.ActivationOutcome{}, err
	}
	outcome := shared.StartedActivation(request)
	if request.Plan.InstallIntent == domain.InstallIntentPrepare {
		outcome.Activation = domain.ActivationPrepared
		outcome.Authentication = domain.AuthenticationNotRequired
		outcome.LocalActions = append(outcome.LocalActions, domain.ChatGPTPreparedAction(request.Delivery.ActivePath, request.DeclaredName))
		outcome.UserActions = append(outcome.UserActions, request.Plan.UserActions...)
		return outcome, nil
	}
	outcome.Activation = domain.ActivationManual
	if shared.ComponentKindPresent(request.Plan.Components, domain.ComponentApp) {
		outcome.UserActions = append(outcome.UserActions, "in ChatGPT Developer Mode, verify the registered connection referenced by .app.json, install the plugin from Plugins, then confirm it is enabled in a new chat")
		outcome.LocalActions = append(outcome.LocalActions, fmt.Sprintf("open ChatGPT Plugins and install %s from the prepared marketplace at %s; verify every .app.json connection before confirming activation", request.DeclaredName, request.Delivery.ActivePath))
		return outcome, nil
	}
	outcome.UserActions = append(outcome.UserActions, "install the prepared skills-only plugin from ChatGPT Plugins, then confirm it is enabled in a new chat")
	outcome.LocalActions = append(outcome.LocalActions, fmt.Sprintf("open ChatGPT Plugins and install %s from the prepared marketplace at %s, then confirm it is enabled in a new chat", request.DeclaredName, request.Delivery.ActivePath))
	return outcome, nil
}

// Deactivate requires the operator to uninstall in ChatGPT Plugins first.
func (*Adapter) Deactivate(_ context.Context, _ clients.Env, request domain.DeactivationRequest) (domain.DeactivationOutcome, error) {
	return shared.RequireExternalUninstall(shared.StartedDeactivation(), request.ExternalUninstalled, "uninstall the plugin in ChatGPT Plugins, then rerun remove with `--external-uninstalled` (also use the flag if it was never activated)"), nil
}
