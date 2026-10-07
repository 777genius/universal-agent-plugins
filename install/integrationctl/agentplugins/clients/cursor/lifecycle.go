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

// Activate reports manual preparation. A reload or operator attestation cannot
// establish native discovery, CLI capability, or genuine Stop qualification.
func (*Adapter) Activate(ctx context.Context, _ clients.Env, request domain.ActivationRequest) (domain.ActivationOutcome, error) {
	if err := ctx.Err(); err != nil {
		return domain.ActivationOutcome{}, err
	}
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
	outcome.UserActions = append(outcome.UserActions, "reload Cursor and verify the prepared plugin and selected components are visible and enabled")
	outcome.LocalActions = append(outcome.LocalActions, fmt.Sprintf("reload Cursor with Developer: Reload Window, then verify %s from %s is visible; native discovery remains unverified", request.DeclaredName, request.Delivery.ActivePath))
	return outcome, nil
}

// Deactivate has nothing to unwind: Cursor never received a managed CLI entry.
func (*Adapter) Deactivate(ctx context.Context, _ clients.Env, _ domain.DeactivationRequest) (domain.DeactivationOutcome, error) {
	if err := ctx.Err(); err != nil {
		return domain.DeactivationOutcome{}, err
	}
	return shared.StartedDeactivation(), nil
}
