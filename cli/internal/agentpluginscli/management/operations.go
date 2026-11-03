package management

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"
)

func (a *Application) runApply(plan *planEntry, record Observation, unlock func() error) {
	defer a.workers.Done()
	defer func() { _ = unlock() }()
	defer plan.prepared.Close()
	a.mu.Lock()
	running := a.running[record.Operation.OperationId]
	running.record.Operation.State = Running
	running.record.Operation.Phase = "preflight"
	ctx := running.ctx
	a.mu.Unlock()
	outcome, panicked, err := executePrepared(ctx, plan.prepared, func(p Progress) { a.progress(record.Operation.OperationId, p) })
	a.mu.Lock()
	record = cloneObservation(a.running[record.Operation.OperationId].record)
	a.mu.Unlock()
	record.Operation = finishOperation(record.Operation, outcome, err, panicked, a.opts.Lock.IsActive(err))
	if err = a.opts.Store.Save(cloneObservation(record)); err != nil {
		record.Operation.State = Interrupted
		record.Operation.Result = nil
		record.Operation.Diagnostics = append(record.Operation.Diagnostics, Diagnostic{
			Code: "OUTCOME_NOT_PERSISTED", Message: "The final result could not be saved; inspect before another mutation", Severity: DiagnosticSeverityError})
	}
	if err == nil && record.Operation.State == Succeeded {
		if linkErr := a.reconcileObservations(plan, record.Operation.OperationId, outcome); linkErr != nil {
			record.Operation.Diagnostics = append(record.Operation.Diagnostics, Diagnostic{Code: "RECOVERY_OBSERVATION_UNAVAILABLE", Message: "Files were reconciled but accepted history could not be settled; inspect and prepare recovery again", Severity: DiagnosticSeverityWarning})
			_ = a.opts.Store.Save(cloneObservation(record))
		}
	}
	a.mu.Lock()
	running.cancel()
	delete(a.running, record.Operation.OperationId)
	delete(a.plans, plan.review.PlanId)
	a.mu.Unlock()
}

func executePrepared(ctx context.Context, p Prepared, progress func(Progress)) (outcome Outcome, panicked bool, err error) {
	defer func() {
		if recover() != nil {
			panicked = true
		}
	}()
	outcome, err = p.Apply(ctx, progress)
	return
}

func finishOperation(op Operation, outcome Outcome, err error, panicked, active bool) Operation {
	op.UpdatedAt = time.Now().UTC()
	op.Phase = "complete"
	op.State = outcome.State
	op.Diagnostics = append(op.Diagnostics, outcome.Diagnostics...)
	if panicked || !op.State.Valid() || !terminal(op.State) || (outcome.Result.Outcome == "" && op.State != Interrupted) {
		op.State = Interrupted
		op.Result = nil
		op.Diagnostics = append(op.Diagnostics, Diagnostic{Code: "EFFECTS_UNKNOWN", Message: "Inspect accepted work before preparing recovery", Severity: DiagnosticSeverityError})
		return op
	}
	if err != nil {
		op.Diagnostics = append(op.Diagnostics, publicFailure(err, active))
		if op.State == Succeeded {
			op.State = Interrupted
			return op
		}
	}
	if outcome.Result.Outcome != "" {
		result := *cloneOperation(Operation{Result: &outcome.Result}).Result
		if result.LocalSkillBackup != nil {
			backup := *result.LocalSkillBackup
			observedAt := op.UpdatedAt
			backup.RecordedAt = &observedAt
			result.LocalSkillBackup = &backup
		}
		if result.Items == nil {
			result.Items = []OperationItem{}
		}
		if result.NextActions == nil {
			result.NextActions = []NextAction{}
		}
		op.Result = &result
		op.Items = append([]OperationItem{}, result.Items...)
		op.Completed = len(op.Items)
		if op.Completed > op.Total {
			op.Total = op.Completed
		}
	}
	return op
}

func (a *Application) progress(id string, progress Progress) {
	a.mu.Lock()
	defer a.mu.Unlock()
	running := a.running[id]
	if running == nil {
		return
	}
	op := &running.record.Operation
	if progress.Phase != "" && len(progress.Phase) <= 128 {
		op.Phase = progress.Phase
	}
	if progress.Completed >= 0 && progress.Total >= progress.Completed {
		op.Completed, op.Total = progress.Completed, progress.Total
	}
	if progress.Items != nil {
		op.Items = append([]OperationItem{}, progress.Items...)
	}
	op.UpdatedAt = time.Now().UTC()
}

func (a *Application) memoryOperation(id string) (Operation, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if running := a.running[id]; running != nil {
		return cloneOperation(running.record.Operation), true
	}
	return Operation{}, false
}

// An accepted record without a live lock is interrupted, not a job to replay.
func (a *Application) observe(ctx context.Context, record Observation) (Operation, error) {
	if op, ok := a.memoryOperation(record.Operation.OperationId); ok {
		return op, nil
	}
	if terminal(record.Operation.State) {
		return record.Operation, nil
	}
	unlock, err := a.opts.Lock.Acquire(ctx)
	if a.opts.Lock.IsActive(err) {
		return record.Operation, nil
	}
	if err != nil {
		return Operation{}, err
	}
	defer func() { _ = unlock() }()
	current, err := a.opts.Store.ByRequest(record.Operation.RequestId)
	if err != nil || current == nil {
		return Operation{}, errors.New("accepted observation disappeared")
	}
	if terminal(current.Operation.State) {
		return current.Operation, nil
	}
	current.Operation.State = Interrupted
	current.Operation.Phase = "interrupted"
	current.Operation.UpdatedAt = time.Now().UTC()
	current.Operation.Result = nil
	current.Operation.Diagnostics = append(current.Operation.Diagnostics, Diagnostic{
		Code: "PROCESS_INTERRUPTED", Message: "The accepting process ended; inspect and prepare recovery without repeating the request", Severity: DiagnosticSeverityWarning})
	if err = a.opts.Store.Save(cloneObservation(*current)); err != nil {
		return Operation{}, err
	}
	return current.Operation, nil
}

func (a *Application) environmentOperations(ctx context.Context) ([]Operation, []LocalSkillBackup, error) {
	records, err := a.opts.Store.List()
	if err != nil {
		return nil, nil, err
	}
	sort.Slice(records, func(i, j int) bool { return records[i].Operation.UpdatedAt.After(records[j].Operation.UpdatedAt) })
	pending, backups := []Operation{}, []LocalSkillBackup{}
	for _, record := range records {
		op := record.Operation
		if pendingOperation(op) {
			op, err = a.observe(ctx, record)
			if err != nil {
				return nil, nil, err
			}
			if pendingOperation(op) {
				pending = append(pending, op)
			}
		}
		if len(backups) < 100 && op.ResolvedBy == "" && time.Since(op.UpdatedAt) < resultRetention {
			if backup := localSkillRestorePoint(op); backup != nil {
				backups = append(backups, *backup)
			}
		}
	}
	return pending, backups, nil
}

// Restore labels come only from the retained operation, never a client-provided
// timestamp/action or list position. Legacy observations retain an unknown action.
func localSkillRestorePoint(op Operation) *LocalSkillBackup {
	if op.Action != LocalSkill || op.LocalSkill == nil || op.Result == nil || op.Result.LocalSkillBackup == nil {
		return nil
	}
	backup := *cloneBackup(op.Result.LocalSkillBackup)
	if backup.OperationId != op.OperationId || backup.RootId != op.LocalSkill.RootId || backup.SkillName != op.LocalSkill.SkillName {
		return nil
	}
	if backup.RecordedAt == nil && !op.UpdatedAt.IsZero() {
		observedAt := op.UpdatedAt
		backup.RecordedAt = &observedAt
	}
	return &backup
}

func pendingOperation(op Operation) bool {
	if op.ResolvedBy != "" {
		return false
	}
	if op.State == Queued || op.State == Running || op.State == Interrupted || op.State == Partial {
		return true
	}
	if op.State != Failed && op.State != Canceled || op.Result == nil {
		return false
	}
	if op.Result.Mutated {
		return true
	}
	for _, action := range op.Result.NextActions {
		kind := strings.ToLower(action.Kind)
		if strings.Contains(kind, "recover") || strings.Contains(kind, "repair") || strings.Contains(kind, "inspect") {
			return true
		}
	}
	return false
}

func (a *Application) Operation(ctx context.Context, id string) (Operation, error) {
	ctx, leave, err := a.admit(ctx)
	if err != nil {
		return Operation{}, err
	}
	defer leave()
	if !ValidID(id) {
		return Operation{}, refusal(400, "INVALID_REQUEST", "Invalid operation identity", false)
	}
	if op, ok := a.memoryOperation(id); ok {
		return op, nil
	}
	record, err := a.opts.Store.ByOperation(id)
	if err != nil {
		return Operation{}, refusal(503, "OBSERVATION_UNAVAILABLE", "Accepted-operation history cannot be read safely", true)
	}
	if record == nil {
		return Operation{}, refusal(404, "OPERATION_NOT_FOUND", "Unknown or expired operation; inspect environment before retrying effects", false)
	}
	op, err := a.observe(ctx, *record)
	if err != nil {
		return Operation{}, refusal(503, "OBSERVATION_UNAVAILABLE", "Accepted work cannot be observed safely", true)
	}
	return cloneOperation(op), nil
}

func (a *Application) CancelOperation(ctx context.Context, id string) (Operation, error) {
	ctx, leave, err := a.admit(ctx)
	if err != nil {
		return Operation{}, err
	}
	defer leave()
	if !ValidID(id) {
		return Operation{}, refusal(400, "INVALID_REQUEST", "Invalid operation identity", false)
	}
	a.mu.Lock()
	running := a.running[id]
	if running != nil {
		running.record.Operation.Phase = "cancellation_requested"
		cancel := running.cancel
		op := cloneOperation(running.record.Operation)
		a.mu.Unlock()
		cancel()
		return op, nil
	}
	a.mu.Unlock()
	record, err := a.opts.Store.ByOperation(id)
	if err != nil {
		return Operation{}, refusal(503, "OBSERVATION_UNAVAILABLE", "Cannot determine operation state", true)
	}
	if record == nil {
		return Operation{}, refusal(404, "OPERATION_NOT_FOUND", "Operation not found", false)
	}
	op, err := a.observe(ctx, *record)
	if err != nil {
		return Operation{}, refusal(503, "OBSERVATION_UNAVAILABLE", "Cannot inspect operation state", true)
	}
	if terminal(op.State) {
		return cloneOperation(op), nil
	}
	return Operation{}, refusal(409, "CANCEL_UNAVAILABLE", "This process does not own the running operation; reconnect to its host", false)
}
