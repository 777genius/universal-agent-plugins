package agentpluginscli

import (
	"fmt"
	"strings"
	"unicode"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

func batchRetryCommandFor(target addTargetResult, source string) string {
	if strings.TrimSpace(source) == "" || batchFailureNeedsDoctor(target) {
		return ""
	}
	switch target.Output.Result.GroupPhase {
	case usecase.GroupTargetExternalFailed, usecase.GroupTargetExternalNotAttempted:
		return batchRetryCommand(source, domain.ClientID(target.Target))
	default:
		return ""
	}
}

func batchFailureNeedsDoctor(target addTargetResult) bool {
	if target.Error == nil {
		return false
	}
	return target.Error.Stage == "persist"
}

func batchRetryCommand(source string, target domain.ClientID) string {
	source = strings.TrimSpace(source)
	targetID := strings.TrimSpace(string(target))
	if source == "" || targetID == "" {
		return ""
	}
	// Emit only shell-safe unquoted args so the same command works under npx on
	// POSIX shells and Windows CMD without POSIX-only quoting.
	if !isBatchRetrySafeArg(source) || !isBatchRetrySafeArg(targetID) {
		return ""
	}
	return "npx universal-agent-plugins add " + source + " --target " + targetID
}

func isBatchRetrySafeArg(value string) bool {
	return value != "" && strings.IndexFunc(value, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) && !strings.ContainsRune("_./:@,+-=", r)
	}) == -1
}

func batchActivationAggregateError(result addMultiResult) error {
	// Exclude deferred ChatGPT (and similar action_required-only rows) from the
	// denominator so partial-failure messaging stays tied to installable peers.
	total := result.Succeeded + result.Failed
	if total == 0 {
		total = countInstallableBatchTargets(result.Targets)
	}
	if total == 0 {
		return fmt.Errorf("client installations failed; see results above")
	}
	return fmt.Errorf("%d of %d client installations failed; see results above", result.Failed, total)
}

func countInstallableBatchTargets(targets []addTargetResult) int {
	count := 0
	for _, target := range targets {
		if target.Status == "action_required" && target.Output.Result.GroupPhase == "" {
			continue
		}
		count++
	}
	return count
}

func batchStatusIndicatesFailure(status string) bool {
	switch status {
	case "", string(usecase.GroupPhaseCompleted), string(usecase.GroupPhasePlanned),
		"completed_with_action_required", "planned_with_action_required", "action_required":
		return false
	default:
		return true
	}
}

func clientDisplayName(client domain.DetectedClient) string {
	if strings.TrimSpace(client.DisplayName) != "" {
		return client.DisplayName
	}
	if definition, ok := domain.ClientDefinitionFor(client.ClientID); ok {
		return definition.DisplayName
	}
	return string(client.ClientID)
}

func addResultTargets(results []addTargetResult) string {
	values := make([]string, len(results))
	for index, result := range results {
		values[index] = result.Target
	}
	return strings.Join(values, ",")
}

// The group result describes physical delivery. Review must also identify the
// selected logical surface when Copilot and VS Code share that delivery.
// Copy the presentation data so JSON, apply inputs and persisted IDs keep their
// physical ownership semantics.
func humanAddGroupPlans(targets []addTargetResult) []usecase.AddResult {
	results := make([]usecase.AddResult, len(targets))
	for index, target := range targets {
		result := target.Output.Result
		if target.Target != string(result.Plan.ClientID) {
			result.Plan.LocalActions = append(append([]string(nil), result.Plan.LocalActions...),
				fmt.Sprintf("Uses shared physical binding owned by %s", result.Plan.ClientID))
			result.Plan.ClientID = domain.ClientID(target.Target)
		}
		results[index] = result
	}
	return results
}

// A ready plan is not actionable when the selected group failed preflight.
// Preserve unsupported-package recovery instructions, but never promise
// activation from a plan whose requirements were rejected.
func setPreflightNextActions(targets []addTargetResult) {
	for index := range targets {
		if targets[index].Output.Result.Plan.Status != domain.PlanUnsupported {
			targets[index].NextAction = "resolve the reported client requirement and retry; nothing was installed"
			targets[index].Output.NextAction = targets[index].NextAction
		}
	}
}

func addGroupNextAction(targets []addTargetResult) string {
	for _, target := range targets {
		if target.NextAction != "" {
			return "; next action: " + target.NextAction
		}
	}
	return ""
}
