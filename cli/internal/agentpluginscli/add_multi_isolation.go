package agentpluginscli

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
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
	input.Targets, input.OperationGroupID = isolation.activeInputs(), isolation.operationID
	first, firstErr := service.AddGroup(ctx, input)
	if !isolation.enabled() {
		return first, firstErr
	}
	// restore reports the whole selection with the given cause and phase. Only
	// refusals before any change reach it, so the selection is unchanged.
	restore := func(phase usecase.GroupPhase, err error) (usecase.GroupResult, error) {
		isolation.active, isolation.operationID = startActive, startID
		whole := first
		whole.Phase = phase
		return whole, err
	}
	result, err := first, firstErr
	left := map[int]isolatedTarget{}
	for {
		failed := targetLocalGroupFailure(ctx, result, err)
		if len(failed) == 0 {
			if len(left) > 0 && ctx.Err() == nil && refusedBeforeChange(result, err) {
				// The others were refused for a reason that concerns them all.
				phase := result.Phase
				if phase == "" {
					phase = usecase.GroupPhasePlanned // Refused before planning any target.
				}
				return restore(phase, err)
			}
			for index, target := range left {
				isolation.excluded[index] = target
			}
			return result, err
		}
		keep := isolation.leaveOut(failed, result, err, left)
		if len(keep) == 0 || len(keep) == len(isolation.active) {
			// Every agent failed on its own, or the failure named none of them:
			// nothing changed, so report the selection as before.
			return restore(first.Phase, firstErr)
		}
		operationID, idErr := newOperationGroupID()
		if idErr != nil {
			return restore(first.Phase, firstErr)
		}
		isolation.active, isolation.operationID = keep, operationID
		input.Targets, input.OperationGroupID = isolation.activeInputs(), isolation.operationID
		result, err = service.AddGroup(ctx, input)
	}
}

// leaveOut records the failed active agents in left and returns the others,
// in selection order.
func (isolation *groupIsolation) leaveOut(failed map[domain.ClientID]bool, result usecase.GroupResult, err error, left map[int]isolatedTarget) []int {
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
	return keep
}

// enabled reports whether agents may be left out. VS Code shares its physical
// delivery with Copilot and is frozen, so a selection with it keeps the
// whole-selection behavior.
func (isolation *groupIsolation) enabled() bool {
	for _, client := range isolation.selected {
		if client.ClientID == domain.ClientVSCode {
			return false
		}
	}
	return true
}

// refusedBeforeChange reports a group that failed without changing anything,
// including one refused before it planned any target (no phase yet).
func refusedBeforeChange(result usecase.GroupResult, err error) bool {
	switch result.Phase {
	case "", usecase.GroupPhasePlanned, usecase.GroupPhasePreparationFailed:
		return err != nil && !result.Mutated
	}
	return false
}

// targetLocalGroupFailure names the agents a refused group can run without. It
// is empty unless the group changed nothing and the usecase attributed the
// failure to those agents' own checks or preparation.
func targetLocalGroupFailure(ctx context.Context, result usecase.GroupResult, err error) map[domain.ClientID]bool {
	var attributed *usecase.GroupTargetError
	if ctx.Err() != nil || !refusedBeforeChange(result, err) || !errors.As(err, &attributed) {
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

// targetLocalKinds are the only causes that leave the other agents to install:
// problems with one agent's own folder, program, facts, support or install
// location. PERMISSION_DENIED, READ_ONLY_LOCATION and AGENT_PROGRAM_MISSING are
// produced by failureKindOf's plain machine-problem classification. Disk full
// and network problems concern the whole machine and stop every agent, as do
// all other and unclassified causes.
var targetLocalKinds = map[failureKind]bool{
	failureForeignTarget:     true,
	failureTargetUnavailable: true,
	failureTargetUnsupported: true,
	"PERMISSION_DENIED":      true,
	"READ_ONLY_LOCATION":     true,
	"AGENT_PROGRAM_MISSING":  true,
}

// failureStaysWithTarget reports whether a cause concerns only the agent it was
// attributed to. It fails closed: the cause and every cause in its chain,
// joined causes included, must be target-local; a single group-wide cause
// anywhere, such as recovery, unconfirmed cleanup, a security block, an edited
// installed copy, a changed plan, source or host, a timeout or cancellation,
// stops every agent.
func failureStaysWithTarget(err error) bool {
	var verification *ports.VerificationError
	if err == nil || errors.As(err, &verification) || !targetLocalKinds[failureKindOf(err)] {
		return false
	}
	return everyCauseStaysWithTarget(err)
}

// everyCauseStaysWithTarget walks the cause tree. A cause the classifier
// recognizes must be target-local, and every innermost cause must be one.
func everyCauseStaysWithTarget(err error) bool {
	kind := failureKindOf(err)
	var causes []error
	switch wrapped := err.(type) { //nolint:errorlint // Walking the cause tree needs each node's own Unwrap.
	case interface{ Unwrap() []error }:
		causes = wrapped.Unwrap()
	case interface{ Unwrap() error }:
		if inner := wrapped.Unwrap(); inner != nil {
			causes = []error{inner}
		}
	}
	if len(causes) == 0 {
		return targetLocalKinds[kind]
	}
	if kind != failureUnknown && !targetLocalKinds[kind] {
		return false
	}
	for _, cause := range causes {
		if cause != nil && !everyCauseStaysWithTarget(cause) {
			return false
		}
	}
	return true
}

// withExcluded returns the rows of every selected agent in selection order:
// rows[position] belongs to the active agent at that position, and each agent
// left out gets its failure row.
func (isolation *groupIsolation) withExcluded(rows []addTargetResult, combined *addMultiResult, dryRun bool) []addTargetResult {
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
		row := isolation.excludedRow(index, dryRun)
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

// excludedRow reports one agent left out of its group: in plain words when the
// cause has a plain explanation, otherwise the cause itself. The full chain is
// kept for the install log. The retry step is the person's own command with
// --target, which keeps scope, project root and saving; a reconstructed
// command could not.
func (isolation *groupIsolation) excludedRow(index int, dryRun bool) addTargetResult {
	excluded := isolation.excluded[index]
	result := excluded.result
	result.InstallationID = "" // The failed attempt's installation was never recorded.
	failure := describeSyncFailure(excluded.cause)
	if result.Plan.Status == domain.PlanUnsupported {
		failure.Kind = failureTargetUnsupported
		failure.Summary, failure.Next, failure.Retryable = applyFailureWording(failure.Kind)
	}
	targetFailure := &usecase.GroupTargetFailure{Stage: "preparation", Message: failure.Summary + " " + failure.Next}
	result.GroupPhase, result.Failure = usecase.GroupTargetExternalFailed, targetFailure
	output := newAddResultData(isolation.inputs[index].Envelope, result, dryRun)
	output.NextAction = failure.Next
	row := addTargetResult{
		Target: string(isolation.selected[index].ClientID), Status: string(result.GroupPhase), Output: output,
		NextAction: failure.Next, Error: targetFailure, displayName: clientDisplayName(isolation.selected[index]),
		failure: failure,
	}
	return row
}

// excludedRetryOffered reports whether a left-out agent's row ends with the
// retry step. Its next step already says what to fix; a retry line is added
// only when a rerun can then succeed. A destination UAP does not own needs the
// person to move it first, an unsupported agent needs another choice, and
// unconfirmed process cleanup stops every retry.
func excludedRetryOffered(target addTargetResult, dryRun bool) bool {
	if dryRun || target.failure == nil || !target.failure.Retryable || target.failure.Kind == failureForeignTarget {
		return false
	}
	return target.Error == nil || target.Error.Stage != "cleanup"
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
		row := isolation.excludedRow(index, true)
		lines = append(lines, fmt.Sprintf("%s will not be installed: %s", batchTargetLabel(row), row.Error.Message))
	}
	return lines
}

// excludedError is the result of a dry run or an apply in which some agents
// were left out.
func (isolation *groupIsolation) excludedError(combined addMultiResult) error {
	if len(isolation.excluded) == 0 {
		return nil
	}
	if combined.DryRun {
		return fmt.Errorf("%d of %d selected clients cannot be installed; no target was changed (dry run); see results above", len(isolation.excluded), len(isolation.selected))
	}
	return batchActivationAggregateError(combined)
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
