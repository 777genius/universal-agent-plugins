package agentpluginscli

import "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"

// normalizeAgentItem is the presentation boundary for reports, logs and binding
// JSON. Missing optional metadata cannot turn a non-success status into success.
// A provided failure retains its typed classification and namespace stop policy.
func normalizeAgentItem(item agentItem) agentItem {
	if item.failure != nil || item.Status == string(installer.OutcomeCompleted) || item.Status == string(installer.OutcomeUnchanged) {
		return item
	}
	kind := failureKindOfOutcome(installer.Outcome(item.Status))
	if item.Status == "not_attempted" {
		kind = failureSkipped
	}
	failure := installFailure{Kind: kind}
	failure.Summary, failure.Next, failure.Retryable = applyFailureWording(kind)
	failure.stopsNamespaceRetry = kind == failureRecovery || kind == failureProcessCleanup
	item.failure = &failure
	return item
}
