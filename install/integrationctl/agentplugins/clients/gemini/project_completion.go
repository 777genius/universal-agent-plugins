package gemini

import (
	"context"
	"errors"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func completeGeminiProject(ctx context.Context, env clients.Env, request domain.ActivationRequest, outcome domain.ActivationOutcome) (domain.ActivationOutcome, error) {
	var err error
	allowSkillAuthentication := !outcome.ActivationAttested && !outcome.AuthenticationAttested && !request.ActivationComplete
	if request.Plan.Scope == domain.ScopeProject && outcome.Verification == domain.VerificationInstalled {
		changed, trustErr := trustGeminiProject(env, request)
		if changed && outcome.NativeEffect != domain.NativeEffectUncertain {
			outcome.NativeEffect = domain.NativeEffectCommitted
		}
		if trustErr != nil {
			allowSkillAuthentication = false
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
		} else if allowSkillAuthentication {
			qualifyGeminiObservedSkills(request, &outcome)
		}
		outcome.UserActions = clearGeminiDiscoveryActions(outcome.UserActions)
	}
	return outcome, err
}

func clearGeminiDiscoveryActions(actions []string) []string {
	remaining := actions[:0]
	for _, action := range actions {
		if action != geminiMCPConfirmationAction && action != "in a running Gemini CLI session use `/mcp reload` and `/skills reload`, or restart Gemini CLI" && action != "Gemini project configuration is installed; native discovery, workspace trust, system policy and MCP initialization remain unobserved; start Gemini CLI in the selected project and inspect skills list and mcp list" {
			remaining = append(remaining, action)
		}
	}
	return remaining
}

func qualifyGeminiObservedSkills(request domain.ActivationRequest, outcome *domain.ActivationOutcome) {
	if request.Plan.Authentication != domain.AuthenticationNotChecked || outcome.Authentication != domain.AuthenticationNotChecked ||
		request.Plan.Policy != domain.PolicyAllowed || outcome.Policy != domain.PolicyAllowed ||
		outcome.Activation != domain.ActivationActive || outcome.Verification != domain.VerificationInstalled ||
		outcome.ActivationAttested || outcome.AuthenticationAttested || outcome.NativeEffect == domain.NativeEffectUncertain ||
		shared.ObservedSkillInstallationAuthentication(request.Plan) != domain.AuthenticationNotRequired {
		return
	}
	// The successful observer checked these receipts before and after the fresh
	// native inventory. Require complete selected coverage, not an empty subset.
	for _, component := range request.Plan.Components {
		matches := 0
		for _, object := range request.Delivery.NativeObjects {
			if object.Kind == GeminiSkillObjectKind && object.LogicalName == component.Name &&
				object.Path == filepath.Join(request.Plan.NativeRegistryRoot, "skills", component.Name) &&
				object.ManagedDigest != "" && object.ProtectionClass == "managed" {
				matches++
			}
		}
		if matches != 1 {
			return
		}
	}
	outcome.Authentication = domain.AuthenticationNotRequired
}
