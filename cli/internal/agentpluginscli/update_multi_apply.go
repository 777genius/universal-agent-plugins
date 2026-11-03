package agentpluginscli

import (
	"context"
	"errors"
	"fmt"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

func (prepared *preparedUpdateMany) planUpdate(ctx context.Context) (*preparedUpdateMany, error) {
	loaded := prepared.loaded
	result := updateMultiResult{
		Batch: true, Status: "planned", Plugin: loaded.envelope.Manifest.Name,
		Version: loaded.envelope.Manifest.Version, Source: publicPackageSource(loaded.envelope.Source),
		Revision: loaded.envelope.Source.ResolvedRevision, TreeDigest: loaded.envelope.TreeDigest,
		DryRun: prepared.dryRun, Targets: make([]updateTargetResult, 0, len(prepared.inputs)),
	}
	operationID, err := newOperationGroupID()
	if err != nil {
		prepared.cleanup()
		return nil, err
	}
	result.OperationID = operationID
	planned, err := prepared.service.UpdateGroup(ctx, usecase.GroupInput{
		Targets: prepared.inputs, CompatibilityChecks: prepared.compatibility, OperationGroupID: operationID, DryRun: true, OnlinePreview: !prepared.dryRun,
	})
	if countErr := validateUpdateResultCount(len(planned.Targets), len(prepared.inputs), err); countErr != nil {
		prepared.cleanup()
		return nil, countErr
	}
	retainPreparedClients(prepared.inputs, planned.PreparedClients)
	retainPreparedClients(prepared.compatibility, planned.PreparedClients)
	prepared.recordPlannedTargets(&result, planned, operationID)
	prepared.result = result
	if err != nil {
		result.Status, result.Failed, result.Succeeded = "preflight_failed", len(prepared.inputs), 0
		prepared.result = result
		return prepared, err
	}
	return prepared, nil
}

func (prepared *preparedUpdateMany) recordPlannedTargets(result *updateMultiResult, planned usecase.GroupResult, operationID string) {
	for index, targetResult := range planned.Targets {
		output := newAddResultData(prepared.inputs[index].Envelope, targetResult, true)
		output.OperationID = operationID
		result.Targets = append(result.Targets, updateTargetResult{
			Target: string(prepared.selected[index]), Selected: true, Status: groupTargetStatus(targetResult),
			Output: output, NextAction: nextLifecycleAction(targetResult),
		})
	}
	result.Succeeded = len(planned.Targets)
	prepared.noChange = len(planned.Targets) > 0
	for _, targetResult := range planned.Targets {
		if !targetResult.NoChange {
			prepared.noChange = false
			break
		}
	}
}

func applyPreparedUpdate(ctx context.Context, prepared *preparedUpdateMany) (updateMultiResult, error) {
	result := prepared.result
	applied, groupErr := prepared.service.UpdateGroup(ctx, usecase.GroupInput{
		Targets: prepared.inputs, CompatibilityChecks: prepared.compatibility, OperationGroupID: result.OperationID, Confirmed: true,
	})
	if countErr := validateUpdateResultCount(len(applied.Targets), len(prepared.inputs), groupErr); countErr != nil {
		result.Status, result.Succeeded, result.Failed = "apply_failed", 0, len(prepared.inputs)
		result.Targets = result.Targets[:0]
		return result, countErr
	}
	result.Status, result.Targets, result.Succeeded = string(applied.Phase), result.Targets[:0], 0
	for index, targetResult := range applied.Targets {
		output := newAddResultData(prepared.inputs[index].Envelope, targetResult, false)
		output.OperationID = result.OperationID
		result.Targets = append(result.Targets, updateTargetResult{
			Target: string(prepared.selected[index]), Selected: true, Status: groupTargetStatus(targetResult),
			Output: output, NextAction: nextLifecycleAction(targetResult),
		})
		if targetResult.GroupPhase == usecase.GroupTargetExternalCompleted {
			result.Succeeded++
		}
	}
	if groupErr != nil {
		result.Status, result.Failed = groupFailureStatus(applied.Phase), len(prepared.inputs)-result.Succeeded
		return result, groupErr
	}
	return result, nil
}

// A group engine may return partial results when it also returns an error.
// A result with no error must cover every selected target; extra results are
// always invalid because their indices have no selected-client authority.
func validateUpdateResultCount(got, selected int, groupErr error) error {
	if got > selected || (groupErr == nil && got != selected) {
		return errors.Join(fmt.Errorf("update engine returned %d results for %d selected targets", got, selected), groupErr)
	}
	return nil
}

func (prepared *preparedUpdateMany) cleanup() {
	if prepared == nil || prepared.loaded.cleanup == nil {
		return
	}
	_ = prepared.loaded.cleanup()
	prepared.loaded.cleanup = nil
}
