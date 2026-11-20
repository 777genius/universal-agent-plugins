package agentpluginscli

import (
	"fmt"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/prompt"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

const undetectedAutomaticInstallReason = "client is not detected; install or configure it before selecting it"

// Only this neutral missing-client fact is folded. Every actionable reason stays.
func humanSkippedTargetLabels(skipped []targetSkip) []string {
	var labels []string
	missing := map[domain.ClientID]bool{}
	for _, target := range skipped {
		if target.Reason == undetectedAutomaticInstallReason {
			missing[target.Client] = true
			continue
		}
		labels = append(labels, prompt.SafeText(reviewClientName(target.Client)+": "+target.Reason))
	}
	if len(missing) > 0 {
		noun := "agents"
		if len(missing) == 1 {
			noun = "agent"
		}
		labels = append(labels, fmt.Sprintf("%d %s not detected; install or configure an agent before selecting it", len(missing), noun))
	}
	return labels
}

func humanDependencyVerb(action string) string {
	if action == "" {
		return "Operation"
	}
	return prompt.SafeText(strings.ToUpper(action[:1]) + action[1:])
}

func humanDependencyPlanLine(action dependencyAction) string {
	reason := action.Reason
	if action.Action == environment.ActionInstall && reason == "no matching environment-owned binding" {
		reason = ""
	}
	if reason != "" {
		reason = ": " + reason
	}
	return fmt.Sprintf("%s %s for %s%s", humanDependencyVerb(string(action.Action)), prompt.SafeText(action.Alias), prompt.SafeText(reviewClientName(domain.ClientID(action.ClientID))), reason)
}

// Results can omit not-attempted bindings. Match the actual paired identity,
// never a planned/result position or a client alone; ambiguous names stay IDs.
func dependencyResultAlias(outcome *installOutcome, result dependencyResultDTO) string {
	alias := ""
	if outcome != nil && result.InstallationID != "" {
		for _, item := range outcome.Items {
			if item.installationID != result.InstallationID || item.ClientID != result.ClientID || item.Alias == "" {
				continue
			}
			if alias != "" && alias != item.Alias {
				return result.InstallationID
			}
			alias = item.Alias
		}
	}
	if alias != "" {
		return alias
	}
	return result.InstallationID
}
