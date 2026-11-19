package agentpluginscli

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/clientdetect"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/hostprep"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/usecase"
)

// groupIsolation keeps one agent's failure from stopping the others in a plugin
// group. When a group refuses before changing anything because one selected
// agent could not be checked or prepared, that agent is left out and the rest
// run again as a new group, exactly as if only they had been selected. Causes
// that concern the whole operation still stop every agent, and a selection in
// which every agent fails on its own is reported as it always was.
type groupIsolation struct {
	selected    []domain.DetectedClient
	inputs      []usecase.AddInput
	active      []int // indexes into selected, in selection order
	excluded    map[int]isolatedTarget
	operationID string
}

// isolatedTarget is a selected agent left out of its group, with the result and
// the typed cause of the attempt that failed for it.
type isolatedTarget struct {
	result usecase.AddResult
	cause  error
}

func newGroupIsolation(selected []domain.DetectedClient, inputs []usecase.AddInput, operationID string) *groupIsolation {
	active := make([]int, len(selected))
	for index := range active {
		active[index] = index
	}
	return &groupIsolation{selected: selected, inputs: inputs, active: active, excluded: map[int]isolatedTarget{}, operationID: operationID}
}

func (isolation *groupIsolation) activeInputs() []usecase.AddInput {
	inputs := make([]usecase.AddInput, len(isolation.active))
	for position, index := range isolation.active {
		inputs[position] = isolation.inputs[index]
	}
	return inputs
}

func (isolation *groupIsolation) activeClients() []domain.DetectedClient {
	clients := make([]domain.DetectedClient, len(isolation.active))
	for position, index := range isolation.active {
		clients[position] = isolation.selected[index]
	}
	return clients
}

// addGroup runs the group for the active agents. Each retry without a failed
// agent is a new operation with its own id, so nothing staged or recorded for
// the failed attempt is reused.
func (isolation *groupIsolation) addGroup(ctx context.Context, service usecase.Service, input usecase.GroupInput) (usecase.GroupResult, error) {
	startActive, startID := append([]int(nil), isolation.active...), isolation.operationID
	var first usecase.GroupResult
	var firstErr error
	left := map[int]isolatedTarget{}
	for attempt := 0; ; attempt++ {
		input.Targets, input.OperationGroupID = isolation.activeInputs(), isolation.operationID
		result, err := service.AddGroup(ctx, input)
		if attempt == 0 {
			first, firstErr = result, err
		}
		failed := targetLocalGroupFailure(ctx, result, err)
		keep := make([]int, 0, len(isolation.active))
		for position, index := range isolation.active {
			if !failed[isolation.selected[index].ClientID] {
				keep = append(keep, index)
				continue
			}
			row := usecase.AddResult{}
			if position < len(result.Targets) {
				row = result.Targets[position]
			}
			left[index] = isolatedTarget{result: row, cause: err}
		}
		if len(keep) == 0 {
			// Every agent failed on its own: nothing changed, report as before.
			isolation.active, isolation.operationID = startActive, startID
			return first, firstErr
		}
		if len(keep) == len(isolation.active) {
			for index, target := range left {
				isolation.excluded[index] = target
			}
			return result, err
		}
		operationID, idErr := newOperationGroupID()
		if idErr != nil {
			isolation.active, isolation.operationID = startActive, startID
			return result, errors.Join(err, idErr)
		}
		isolation.active, isolation.operationID = keep, operationID
	}
}

// targetLocalGroupFailure names the agents a refused group can run without. It
// is empty unless the group changed nothing and the usecase attributed the
// failure to those agents' own checks or preparation.
func targetLocalGroupFailure(ctx context.Context, result usecase.GroupResult, err error) map[domain.ClientID]bool {
	var attributed *usecase.GroupTargetError
	if err == nil || ctx.Err() != nil || result.Mutated || !errors.As(err, &attributed) {
		return nil
	}
	if result.Phase != usecase.GroupPhasePlanned && result.Phase != usecase.GroupPhasePreparationFailed {
		return nil
	}
	if !failureStaysWithTarget(err) {
		return nil
	}
	failed := make(map[domain.ClientID]bool, len(attributed.Clients))
	for _, client := range attributed.Clients {
		failed[client] = true
	}
	return failed
}

// failureStaysWithTarget reports whether a cause concerns only the agent it was
// attributed to. These causes stop every agent instead: unconfirmed process
// cleanup and pending recovery need attention first, a security block concerns
// the package, edited or differently versioned installed copies need repair or
// update first, changed state invalidates the reviewed plan, and a timeout or
// cancellation ends the operation.
func failureStaysWithTarget(err error) bool {
	switch failureKindOf(err) {
	case failureProcessCleanup, failureRecovery, failureSecurityBlocked, failureManagedChanged,
		failureUpdateRequired, failureEnvironmentChange, failureTimeout, failureCanceled:
		return false
	}
	return true
}

// hostChangedSinceReview reports that an agent's host no longer matches the
// snapshot reviewed before consent, so the reviewed plan no longer holds.
func hostChangedSinceReview(err error) bool {
	return errors.Is(err, clientdetect.ErrProbeTargetChanged) || errors.Is(err, hostprep.ErrPlanChanged)
}

// withExcluded returns the rows of every selected agent in selection order:
// rows[position] belongs to the active agent at that position, and each agent
// left out gets its failure row.
func (isolation *groupIsolation) withExcluded(rows []addTargetResult, combined *addMultiResult, dryRun bool, sourceArg string) []addTargetResult {
	if len(isolation.excluded) == 0 {
		return rows
	}
	type ordered struct {
		index int
		row   addTargetResult
	}
	all := make([]ordered, 0, len(rows)+len(isolation.excluded))
	for position, row := range rows {
		all = append(all, ordered{index: isolation.active[position], row: row})
	}
	for index := range isolation.excluded {
		row := isolation.excludedRow(index, dryRun, sourceArg)
		row.Output.OperationID = combined.OperationID
		combined.setTargetProof(isolation.selected[index].ClientID, "failed")
		all = append(all, ordered{index: index, row: row})
	}
	sort.SliceStable(all, func(left, right int) bool { return all[left].index < all[right].index })
	merged := make([]addTargetResult, len(all))
	for position, entry := range all {
		merged[position] = entry.row
	}
	return merged
}

// excludedRow reports one agent left out of its group. The cause is shown in
// plain words; its technical text goes only to the install log.
func (isolation *groupIsolation) excludedRow(index int, dryRun bool, sourceArg string) addTargetResult {
	excluded := isolation.excluded[index]
	result := excluded.result
	failure := describeSyncFailure(excluded.cause)
	if result.Plan.Status == domain.PlanUnsupported {
		failure.Kind = failureTargetUnsupported
		failure.Summary, failure.Next, failure.Retryable = applyFailureWording(failure.Kind)
	}
	message := failure.Summary + " " + failure.Next
	if failure.Kind == failureUnknown {
		// No plain explanation exists for this cause; show it precisely.
		message = failure.Detail
	}
	targetFailure := &usecase.GroupTargetFailure{Stage: "preparation", Message: message}
	result.GroupPhase, result.Failure = usecase.GroupTargetExternalFailed, targetFailure
	output := newAddResultData(isolation.inputs[index].Envelope, result, dryRun)
	output.NextAction = failure.Next
	row := addTargetResult{
		Target: string(isolation.selected[index].ClientID), Status: string(result.GroupPhase), Output: output,
		NextAction: failure.Next, Error: targetFailure, displayName: clientDisplayName(isolation.selected[index]),
		failure: failure,
	}
	if failure.Retryable && !dryRun {
		row.RetryCommand = batchRetryCommandFor(row, sourceArg)
	}
	return row
}

// excludedNotice tells a person reviewing the plan which agents were left out
// before they confirm installing the others.
func (isolation *groupIsolation) excludedNotice() []string {
	indexes := make([]int, 0, len(isolation.excluded))
	for index := range isolation.excluded {
		indexes = append(indexes, index)
	}
	sort.Ints(indexes)
	lines := make([]string, 0, len(indexes))
	for _, index := range indexes {
		row := isolation.excludedRow(index, true, "")
		lines = append(lines, fmt.Sprintf("%s will not be installed: %s", batchTargetLabel(row), row.Error.Message))
	}
	return lines
}

// excludedError is the result of a dry run or an apply in which some agents
// were left out. Causes without a plain explanation are named precisely.
func (isolation *groupIsolation) excludedError(combined addMultiResult) error {
	if len(isolation.excluded) == 0 {
		return nil
	}
	summary := batchActivationAggregateError(combined).Error()
	if combined.DryRun {
		summary = fmt.Sprintf("%d of %d selected clients cannot be installed; no target was changed (dry run); see results above", len(isolation.excluded), len(isolation.selected))
	}
	for _, row := range combined.Targets {
		if row.failure != nil && row.failure.Kind == failureUnknown {
			summary += "; " + row.Target + ": " + row.failure.Detail
		}
	}
	return errors.New(summary)
}

// boardResults aligns the final rows with the clients shown on the progress
// board, which lists the agents that were still selected when apply started.
func boardResults(clients []domain.DetectedClient, rows []addTargetResult) []usecase.AddResult {
	results := make([]usecase.AddResult, len(clients))
	for position, client := range clients {
		for _, row := range rows {
			if row.Target == string(client.ClientID) {
				results[position] = row.Output.Result
				break
			}
		}
	}
	return results
}
