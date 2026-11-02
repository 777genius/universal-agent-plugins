package agentpluginscli

import (
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/management"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

// Native Result.Reason may wrap provider stderr. Management publishes stable,
// actionable status messages rather than transporting that unbounded text.
func managementNativeMessage(result installer.Result) string {
	switch result.Outcome {
	case installer.OutcomeCompleted:
		return "Binding changes completed"
	case installer.OutcomeUnchanged:
		return "Binding already matches the reviewed intent"
	case installer.OutcomeIncomplete:
		return "Binding changes are incomplete; inspect readiness before continuing"
	case installer.OutcomeRecovery:
		return "Pending installer receipts require recovery"
	case installer.OutcomeConflict:
		return "Ownership or native target changed; prepare a new review"
	case installer.OutcomeCancelled:
		return "Binding operation was canceled; inspect the observed result"
	default:
		return "Native outcome is unknown; inspect current installer state"
	}
}

func managementNativeNextAction(kind, alias, target string) management.NextAction {
	messages := map[string]string{
		"recover":            "Inspect pending installer receipts and prepare fresh recovery",
		"update":             "Prepare a fresh update review for the desired release",
		"reprepare":          "Current native facts changed; prepare a new review",
		"activate":           "Complete the native client's activation workflow and inspect readiness",
		"external_uninstall": "Uninstall the owned package in the native client, then review removal with external uninstall confirmed",
		"data_compatibility": "Retained plugin data needs a compatibility review before further changes",
		"manual":             "Complete the native client's manual setup and inspect readiness",
	}
	reason, ok := messages[kind]
	if !ok {
		kind, reason = "inspect", "Inspect current native state before another change"
	}
	return management.NextAction{Kind: kind, Reason: reason, Alias: alias, TargetId: target}
}

const managementInstructionMaxCount = 32
const managementInstructionMaxLength = 4096 // NextAction.reason contract limit, in Unicode code points.

// ManualActions carries the adapters' public UserActions, including restart and
// cleanup advice. Preserve those instructions without forwarding private native
// reasons or LocalActions, and never turn a truncated command into guidance.
func managementManualNextActions(instructions []string, alias, target string) []management.NextAction {
	fallback := managementNativeNextAction("manual", alias, target)
	if len(instructions) > managementInstructionMaxCount {
		return []management.NextAction{fallback}
	}
	actions := make([]management.NextAction, 0, len(instructions))
	for _, instruction := range instructions {
		action := fallback
		if managementInstructionValid(instruction) {
			action.Reason = instruction
		}
		actions = append(actions, action)
	}
	return actions
}

func managementInstructionValid(instruction string) bool {
	if len(instruction) > managementInstructionMaxLength*utf8.UTFMax || !utf8.ValidString(instruction) || strings.TrimSpace(instruction) == "" || utf8.RuneCountInString(instruction) > managementInstructionMaxLength {
		return false
	}
	return strings.IndexFunc(instruction, func(r rune) bool {
		return unicode.IsControl(r) && r != '\n' && r != '\t'
	}) < 0
}

// Map only the typed native ownership refusal. Provider messages and paths are
// never parsed or forwarded, and readiness guidance for other bindings survives.
func managementOwnedSkillFailure(out *management.Outcome, applyErr error) bool {
	if !errors.Is(applyErr, installer.ErrManagedSkillChanged) {
		return false
	}
	alias, target := "", ""
	for _, item := range out.Result.Items {
		if item.Status == string(installer.OutcomeIncomplete) {
			alias, target = item.Alias, item.TargetId
		}
	}
	message := "UAP preserved an externally changed managed Skill directory. Preserve that directory outside the native Skills root, then prepare a fresh recovery review"
	out.Result.Message = "Managed Skill content changed; preserve it before continuing recovery"
	out.Diagnostics = append(out.Diagnostics, management.Diagnostic{Code: "OWNED_SKILL_CHANGED", Message: message, Severity: management.DiagnosticSeverityError, Alias: alias, TargetId: target})
	next := []management.NextAction{{Kind: "preserve_changes", Reason: message, Alias: alias, TargetId: target}}
	for _, action := range out.Result.NextActions {
		if action.Alias == alias && action.TargetId == target && (action.Kind == "activate" || action.Kind == "manual") {
			continue
		}
		next = append(next, action)
	}
	out.Result.NextActions = next
	return true
}
