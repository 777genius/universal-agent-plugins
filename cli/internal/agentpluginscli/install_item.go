package agentpluginscli

import (
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

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

// Before the coordinator runs, a refused environment has no binding outcomes.
// Missing reviewed actions are unattempted; observed outcomes keep their exact
// facts even when an unrelated binding or a later boundary failed.
func appliedAgentItems(actions []dependencyAction, outcomes []environment.BindingOutcome, cause error) []agentItem {
	observed := outcomeAgentItems(outcomes)
	if cause == nil {
		return observed
	}
	key := func(binding dependencyAction) [3]string {
		return [3]string{binding.Alias, binding.ClientID, string(binding.Action)}
	}
	byBinding := make(map[[3]string]agentItem, len(outcomes))
	for index, outcome := range outcomes {
		byBinding[key(outcome.Binding)] = observed[index]
	}
	items := make([]agentItem, 0, len(actions))
	for _, action := range actions {
		identity := key(action)
		if item, found := byBinding[identity]; found {
			items = append(items, item)
			delete(byBinding, identity)
			continue
		}
		items = append(items, agentItem{Alias: action.Alias, ClientID: action.ClientID, Status: "not_attempted", failure: describeSyncFailure(cause)})
	}
	// Preserve any additional coordinator fact rather than silently discarding
	// it if a caller supplied an incomplete presentation list.
	for index, outcome := range outcomes {
		identity := key(outcome.Binding)
		if _, remaining := byBinding[identity]; remaining {
			items = append(items, observed[index])
			delete(byBinding, identity)
		}
	}
	return items
}
