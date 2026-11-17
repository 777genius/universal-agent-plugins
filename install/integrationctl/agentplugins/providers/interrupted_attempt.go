package providers

import (
	"context"
	"fmt"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
)

var _ ports.InterruptedAttemptConfirmer = Activator{}

// ConfirmsInterruptedAttempt reports whether the client adapter can confirm a
// native attempt left by a killed run in this scope.
func (activator Activator) ConfirmsInterruptedAttempt(id domain.ClientID, scope domain.InstallScope) bool {
	_, ok := activator.interruptedAttemptConfirmer(id, scope)
	return ok
}

// ConfirmInterruptedAttempt delegates to the client adapter; see
// ports.InterruptedAttemptConfirmer.
func (activator Activator) ConfirmInterruptedAttempt(ctx context.Context, request domain.ActivationRequest) (domain.ActivationOutcome, error) {
	confirmer, ok := activator.interruptedAttemptConfirmer(request.Client.ClientID, request.Plan.Scope)
	if !ok {
		return domain.ActivationOutcome{NativeEffect: domain.NativeEffectUncertain}, fmt.Errorf("%s cannot confirm an interrupted client change in %s scope", request.Client.ClientID, request.Plan.Scope)
	}
	return confirmer.ConfirmInterruptedAttempt(ctx, activator.env(request.Client.ClientID), request)
}

func (activator Activator) interruptedAttemptConfirmer(id domain.ClientID, scope domain.InstallScope) (clients.InterruptedAttemptConfirmer, bool) {
	if activator.requireRegistry() != nil {
		return nil, false
	}
	confirmer, ok := clients.As[clients.InterruptedAttemptConfirmer](activator.Registry, id)
	return confirmer, ok && confirmer.ConfirmsInterruptedAttempt(scope)
}
