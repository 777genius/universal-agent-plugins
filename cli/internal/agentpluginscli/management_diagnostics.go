package agentpluginscli

import (
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
		return "Binding operation was cancelled; inspect the observed result"
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
