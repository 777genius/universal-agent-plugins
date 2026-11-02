package agentpluginscli

import (
	"fmt"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

func (session *removeManySession) apply() error {
	session.result.Status = "applying"
	session.result.Succeeded = 0
	session.result.Failed = 0
	session.result.Targets = session.result.Targets[:0]
	for index := range session.inputs {
		session.inputs[index].Confirmed = true
	}
	appliedGroup, groupErr := session.service.RemoveGroup(session.ctx, usecase.RemoveGroupInput{
		Selector: session.installationSelector, Targets: session.inputs, OperationGroupID: session.result.OperationID,
		Confirmed: true, PurgeData: session.opts.purgeData,
	})
	appliedResults := appliedGroup.Targets
	if len(appliedResults) > len(session.inputs) || len(appliedResults) != len(session.inputs) && groupErr == nil {
		return fmt.Errorf("install engine returned %d remove results for %d targets", len(appliedResults), len(session.inputs))
	}
	session.appendAppliedTargets(appliedResults)
	if groupErr != nil {
		session.result.Status, session.result.Failed = groupFailureStatus(appliedGroup.Phase), len(session.inputs)-session.result.Succeeded
		_ = renderRemoveMultiResult(session.cmd, session.opts, session.result)
		return groupErr
	}
	session.result.Status = string(appliedGroup.Phase)
	session.recordRetainedData()
	return renderRemoveMultiResult(session.cmd, session.opts, session.result)
}

func (session *removeManySession) appendAppliedTargets(appliedResults []usecase.RemoveResult) {
	for index, applied := range appliedResults {
		output := newRemoveResultData(session.installation, applied, false)
		output.OperationID = session.result.OperationID
		status := string(applied.GroupPhase)
		if status == "" {
			status = "apply_failed"
		}
		session.result.Targets = append(session.result.Targets, removeTargetResult{Target: string(session.selected[index].ClientID), Status: status, Output: output})
		if applied.GroupPhase == usecase.GroupTargetExternalCompleted {
			session.result.Succeeded++
		}
	}
}

func (session *removeManySession) recordRetainedData() {
	after, loadErr := session.app.StateStore.Load()
	if loadErr != nil {
		return
	}
	retained, ok := locallyMatchedInstallation(after, session.installation.InstallationID)
	if !ok || !retained.DataRetained {
		return
	}
	session.result.DataRetained = true
	session.result.Status = "data_retained"
	for _, receipt := range retained.DataReceipts {
		session.result.RetainedData = append(session.result.RetainedData, retainedDataResult{
			DataReceiptID: receipt.DataReceiptID, PhysicalBackend: receipt.PhysicalBackend,
			Scope: receipt.Scope, State: receipt.State,
		})
	}
	session.result.RetainedDataAction = fmt.Sprintf("run `%s` to delete retained plugin data", retainedDataPurgeCommand(session.app, session.installation.InstallationID))
}

func multiRemovalNeedsManualStep(result removeMultiResult) bool {
	for _, target := range result.Targets {
		if removalNeedsManualStep(target.Output.Result.Deactivation) || target.Status == "blocked" || target.Status == "not_applied" {
			return true
		}
	}
	return false
}
