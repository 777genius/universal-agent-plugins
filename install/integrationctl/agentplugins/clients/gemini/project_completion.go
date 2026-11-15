package gemini

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func completeGeminiProject(ctx context.Context, env clients.Env, request domain.ActivationRequest, outcome domain.ActivationOutcome) (domain.ActivationOutcome, error) {
	var err error
	if request.Plan.Scope == domain.ScopeProject && outcome.Verification == domain.VerificationInstalled {
		if trustErr := trustGeminiProject(env, request); trustErr != nil {
			outcome.UserActions = shared.AppendUnique(outcome.UserActions, "Gemini project trust remains unchanged: "+trustErr.Error())
		}
	}
	outcome, err = geminiProjectStatus(env, request, outcome)
	if err != nil || request.Plan.Scope != domain.ScopeProject || outcome.Verification != domain.VerificationInstalled {
		return outcome, err
	}
	observed, probeErr := observeGeminiProjectComponents(ctx, env, request)
	if domain.IsUnclosedClientProbe(probeErr) {
		outcome.NativeEffect = domain.NativeEffectUncertain
		return outcome, probeErr
	}
	if ctx.Err() != nil {
		return outcome, ctx.Err()
	}
	if observed && probeErr == nil {
		outcome.Activation = domain.ActivationActive
		if geminiObservedLocalMCPAuthentication(request.Plan) {
			outcome.Authentication = domain.AuthenticationNotRequired
		}
		actions := outcome.UserActions[:0]
		for _, action := range outcome.UserActions {
			if action != "in a running Gemini CLI session use `/mcp reload` and `/skills reload`, or restart Gemini CLI" && action != "Gemini project configuration is installed; native discovery, workspace trust, system policy and MCP initialization remain unobserved; start Gemini CLI in the selected project and inspect skills list and mcp list" {
				actions = append(actions, action)
			}
		}
		outcome.UserActions = actions
	}
	return outcome, err
}
