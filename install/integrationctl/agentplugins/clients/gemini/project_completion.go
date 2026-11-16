package gemini

import (
	"context"
	"errors"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func completeGeminiProject(ctx context.Context, env clients.Env, request domain.ActivationRequest, outcome domain.ActivationOutcome) (domain.ActivationOutcome, error) {
	if request.VerifyOnly && !geminiNativeDecisionPresent(request.Plan.Components) {
		// Doctor's read-only projection check selects no native component. It
		// asks only whether the exact owned objects are intact; project discovery
		// readiness is reported separately and is never observed by this request.
		return outcome, nil
	}
	var err error
	if request.Plan.Scope == domain.ScopeProject && outcome.Verification == domain.VerificationInstalled {
		changed, trustErr := trustGeminiProject(env, request)
		if changed && outcome.NativeEffect != domain.NativeEffectUncertain {
			outcome.NativeEffect = domain.NativeEffectCommitted
		}
		if trustErr != nil {
			var observed *shared.NativeEffectError
			if errors.As(trustErr, &observed) && observed.Effect == domain.NativeEffectUncertain {
				outcome.NativeEffect = domain.NativeEffectUncertain
				return outcome, trustErr
			}
			outcome.UserActions = shared.AppendUnique(outcome.UserActions, "Gemini project trust needs verification: "+trustErr.Error())
		}
	}
	outcome, err = geminiProjectStatus(env, request, outcome)
	if err != nil || request.Plan.Scope != domain.ScopeProject || outcome.Verification != domain.VerificationInstalled {
		return outcome, err
	}
	observed, probeErr := observeGeminiProjectComponents(ctx, env, request, &outcome)
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
			if action != geminiMCPConfirmationAction && action != "in a running Gemini CLI session use `/mcp reload` and `/skills reload`, or restart Gemini CLI" && action != "Gemini project configuration is installed; native discovery, workspace trust, system policy and MCP initialization remain unobserved; start Gemini CLI in the selected project and inspect skills list and mcp list" {
				actions = append(actions, action)
			}
		}
		outcome.UserActions = actions
	}
	return outcome, err
}

func geminiNativeDecisionPresent(components []domain.ComponentDecision) bool {
	for _, component := range components {
		if component.Support == domain.SupportNative {
			return true
		}
	}
	return false
}
