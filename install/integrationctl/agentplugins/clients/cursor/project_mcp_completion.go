package cursor

import (
	"context"
	"errors"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// cursorMCPApprovalPendingAction records a read-only observation in which the
// first selected server that is not ready awaits approval, and no policy or
// profile refusal applied. It is the only state prepared-setup completion
// admits, so a repeated add finishes an interrupted approval exactly once.
const cursorMCPApprovalPendingAction = "Cursor MCP servers await approval; repeat this add to approve them"

func completeCursorProjectMCP(ctx context.Context, env clients.Env, request domain.ActivationRequest, outcome domain.ActivationOutcome) (domain.ActivationOutcome, error) {
	observed, observationErr := observeCursorProjectMCP(ctx, env, request)
	var effect *shared.NativeEffectError
	if domain.IsUnclosedClientProbe(observationErr) || errors.As(observationErr, &effect) && effect.Effect == domain.NativeEffectUncertain {
		outcome.NativeEffect = domain.NativeEffectUncertain
		return outcome, observationErr
	}
	if ctx.Err() != nil {
		return outcome, ctx.Err()
	}
	if observationErr != nil {
		outcome.UserActions = shared.AppendUnique(outcome.UserActions, observationErr.Error())
	} else if observed.ApprovalPending {
		outcome.UserActions = shared.AppendUnique(outcome.UserActions, cursorMCPApprovalPendingAction)
	}
	if observed.Initialized {
		outcome.UserActions = cursorObservedMCPActions(outcome.UserActions)
		if !observed.Complete {
			outcome.UserActions = shared.AppendUnique(outcome.UserActions, "Cursor MCP servers initialized; native Skill discovery remains pending in a signed-in Cursor session")
		}
	}
	if observed.Complete {
		outcome.Activation = domain.ActivationActive
		if cursorObservedLocalMCPAuthentication(request.Plan) {
			outcome.Authentication = domain.AuthenticationNotRequired
		}
		outcome.UserActions = cursorObservedMCPActions(outcome.UserActions)
	}
	return outcome, nil
}
