package agentpluginscli

import (
	"errors"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

// failImperativeUpdate keeps the original result for JSON and native evidence.
// Human rendering uses the shared classified binding reports and never the raw
// apply/check error. A failed stream cannot hide the original typed cause.
func (session *updateAllSession) failImperativeUpdate(index int, applied *updateMultiResult, cause error) error {
	item := session.preparedItems[index]
	view := &session.result.Installations[item.resultIndex]
	view.Status, view.Reason = "apply_failed", cause.Error()
	session.result.Failed++
	session.result.Status = "partial_failure"
	markUnattemptedUpdateAll(&session.result, session.preparedItems, index+1)
	var items []agentItem
	if previous := session.opts.installResult; previous != nil {
		items = append(items, previous.Items...)
	}
	for _, completed := range session.preparedItems[:index] {
		plan := session.result.Installations[completed.resultIndex].Plan
		if plan != nil {
			items = append(items, imperativeUpdateAgentItems(completed.prepared, plan, nil)...)
		}
	}
	items = append(items, imperativeUpdateAgentItems(item.prepared, applied, cause)...)
	for _, remaining := range session.preparedItems[index+1:] {
		for _, client := range remaining.prepared.selected {
			failure := describeBindingFailure(environment.BindingOutcome{Status: environment.BindingNotAttempted, Err: cause})
			items = append(items, agentItem{Alias: remaining.installation.DeclaredName, ClientID: string(client), Status: "not_attempted", installationID: remaining.installation.InstallationID, failure: failure})
		}
	}
	session.app.finishInstall(session.opts, "update --all", items)
	renderErr := renderUpdateAll(session.cmd, session.opts, session.result)
	return errors.Join(humanInstallApplyError(session.opts.format, session.opts.installResult, cause), installOutputError(renderErr))
}

func imperativeUpdateAgentItems(prepared *preparedUpdateMany, applied *updateMultiResult, cause error) []agentItem {
	rows := map[string]updateTargetResult{}
	if applied != nil {
		for _, row := range applied.Targets {
			if row.Selected {
				rows[row.Target] = row
			}
		}
	}
	items := make([]agentItem, 0, len(prepared.selected))
	for _, client := range prepared.selected {
		item := agentItem{Alias: prepared.result.Plugin, ClientID: string(client), Status: "not_attempted", installationID: prepared.installationID, failure: describeSyncFailure(cause)}
		if row, found := rows[string(client)]; found {
			group := groupAgentItems(item.Alias, []addTargetResult{{Target: row.Target, Status: row.Status, Output: row.Output, NextAction: row.NextAction, Error: row.Output.Result.Failure}})
			item = group[0]
			item.installationID = prepared.installationID
			if item.failure != nil {
				item.failure = imperativeUpdateFailure(item, cause)
			}
		}
		items = append(items, item)
	}
	return items
}

func imperativeUpdateFailure(item agentItem, cause error) *installFailure {
	if domain.IsUnclosedClientProbe(cause) {
		status := environment.BindingFailed
		if item.Status == "not_attempted" {
			status = environment.BindingNotAttempted
		}
		return describeBindingFailure(environment.BindingOutcome{Status: status, Err: cause})
	}
	if item.failure.Kind == failureSkipped || errors.Is(cause, installer.ErrRecoveryRequired) {
		failure := describeSyncFailure(cause)
		if item.failure.Detail != "" {
			failure.Detail = item.failure.Detail + "\n" + failure.Detail
		}
		return failure
	}
	failure := *item.failure
	if failure.Detail == "" {
		failure.Detail = errorText(cause)
	}
	return &failure
}

// Update-all rows aggregate agents. Keep their native/JSON reason untouched;
// a copy of the human view links failed rows to the detailed per-agent report.
func updateAllHumanFailureResult(result updateAllResult, outcome *installOutcome) updateAllResult {
	if outcome == nil || len(outcome.failedAgents()) == 0 {
		return result
	}
	display := result
	display.Installations = append([]updateAllInstallation(nil), result.Installations...)
	for index := range display.Installations {
		row := &display.Installations[index]
		for _, item := range outcome.Items {
			matches := item.installationID != "" && item.installationID == row.InstallationID || item.installationID == "" && item.Alias == row.Name
			if !matches || normalizeAgentItem(item).failure == nil {
				continue
			}
			row.Reason = "One or more agents need attention; follow the per-agent results below."
			break
		}
	}
	return display
}
