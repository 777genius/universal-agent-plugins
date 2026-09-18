package cursor

import (
	"context"
	"fmt"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

var (
	_ clients.Lifecycle = (*Adapter)(nil)
)

// Activate leaves Cursor as a manual registration. There is no managed CLI.
func (*Adapter) Activate(_ context.Context, _ clients.Env, request domain.ActivationRequest) (domain.ActivationOutcome, error) {
	if err := shared.ActivationIdentityMismatch(request); err != nil {
		return domain.ActivationOutcome{}, err
	}
	outcome := shared.StartedActivation(request)
	outcome.Activation = domain.ActivationManual
	if request.VerifyOnly {
		outcome.UserActions = []string{"confirm that the plugin is visible and enabled in Cursor"}
		outcome.LocalActions = []string{fmt.Sprintf("open Cursor and verify the plugin from %s is visible and enabled", request.Delivery.ActivePath)}
		return outcome, nil
	}
	outcome.UserActions = append(outcome.UserActions, "register the prepared package in Cursor and verify it is visible before using its components")
	outcome.LocalActions = append(outcome.LocalActions, fmt.Sprintf("open Cursor and register %s, then reload Cursor and verify %s is visible", request.Delivery.ActivePath, request.DeclaredName))
	return outcome, nil
}

// Deactivate has nothing to unwind: Cursor never received a managed CLI
// registration.
func (*Adapter) Deactivate(_ context.Context, _ clients.Env, _ domain.DeactivationRequest) (domain.DeactivationOutcome, error) {
	return shared.StartedDeactivation(), nil
}
