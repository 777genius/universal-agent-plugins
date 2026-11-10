package agentpluginscli

import (
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

// A readiness summary is an interpretation of existing evidence only. It
// never persists state or attests that a client executed an MCP tool.
func deriveAddReadiness(result usecase.AddResult, dryRun bool) (string, []AgentAction) {
	if dryRun {
		return "unknown", nil
	}
	if addFailureStatus(result, nil) != "" {
		return "failed", nil
	}
	if fullyInstalled(result.Activation) {
		if result.Activation.ActivationAttested || result.Activation.AuthenticationAttested {
			return "needs_action", []AgentAction{{
				Code: "verify_native", Actor: "native_client",
				Message: "A lifecycle phase was attested, not observed; verify it in the selected native client.",
			}}
		}
		return "ready", nil
	}
	if result.Activation.Activation == "" && !result.Mutated && !result.NoChange {
		return "unknown", nil
	}
	message := nextLifecycleAction(result)
	if message == "" {
		return "needs_action", nil
	}
	action := AgentAction{Code: "verify_native", Actor: "native_client", Message: message}
	switch result.Activation.Authentication {
	case domain.AuthenticationPending, domain.AuthenticationFailed:
		action.Code, action.Actor = "authenticate_client", "user"
	default:
		if result.Activation.Activation == domain.ActivationManual {
			action.Code, action.Actor = "activate_client", "user"
		}
	}
	return "needs_action", []AgentAction{action}
}

func deriveSkillsReadiness(status string) (string, []AgentAction) {
	switch status {
	case "failed", "partial", "interrupted":
		return "failed", nil
	case "completed", "succeeded", "unchanged":
		return "needs_action", []AgentAction{{
			Code: "verify_native", Actor: "native_client",
			Message: "Start a fresh selected client session and verify that the installed Skill is discovered.",
		}}
	default:
		return "unknown", nil
	}
}

func deriveDependencyReadiness(facts []string) string {
	if len(facts) == 0 {
		return "ready"
	}
	for _, fact := range facts {
		switch fact {
		case "auth_failed", "store_mismatch", "store_unreadable", "store_missing":
			return "failed"
		}
	}
	for _, fact := range facts {
		switch fact {
		case "activation_manual", "activation_prepared", "auth_unverified", "auth_pending", "partial", "not_synced", "pending_recovery":
			return "needs_action"
		}
		if strings.HasPrefix(fact, "projection_") {
			return "needs_action"
		}
	}
	return "unknown"
}

func dependencyAgentActions(actions []dependencyNextActionDTO) []AgentAction {
	result := make([]AgentAction, 0, len(actions))
	for _, next := range actions {
		actor := "agent"
		if next.Kind == "review_project_activation" || next.Kind == "attest_activation" || next.Kind == "attest_authentication" {
			actor = "user"
		}
		result = append(result, AgentAction{Code: next.Kind, Actor: actor, Message: next.Reason})
	}
	return result
}
