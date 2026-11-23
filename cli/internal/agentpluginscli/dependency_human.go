package agentpluginscli

import (
	"fmt"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/prompt"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

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

// Each footer ID names its actual aliases without repeating the ID per client.
func dependencyInstallationAliasSuffix(outcome *installOutcome, results []dependencyResultDTO, installationID string) string {
	var aliases []string
	seen := map[string]bool{}
	for _, result := range results {
		if result.InstallationID != installationID {
			continue
		}
		alias := dependencyResultAlias(outcome, result)
		if alias == "" || alias == installationID || seen[alias] {
			continue
		}
		seen[alias] = true
		aliases = append(aliases, alias)
	}
	if len(aliases) == 0 {
		return ""
	}
	return " (" + strings.Join(aliases, ", ") + ")"
}
