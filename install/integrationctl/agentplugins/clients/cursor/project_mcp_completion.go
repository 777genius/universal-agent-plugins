package cursor

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func completeCursorProjectMCP(ctx context.Context, env clients.Env, request domain.ActivationRequest, outcome domain.ActivationOutcome) (domain.ActivationOutcome, error) {
	observed, observationErr := observeCursorProjectMCP(ctx, env, request)
	if domain.IsUnclosedClientProbe(observationErr) {
		outcome.NativeEffect = domain.NativeEffectUncertain
		return outcome, observationErr
	}
	if ctx.Err() != nil {
		return outcome, ctx.Err()
	}
	if observationErr != nil {
		outcome.UserActions = shared.AppendUnique(outcome.UserActions, observationErr.Error())
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
