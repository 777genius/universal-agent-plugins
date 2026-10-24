package management

import (
	"context"
	"errors"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/777genius/plugin-kit-ai/cli/internal/loopbackapi"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/processlock"
)

func (s *Server) runApply(plan *planEntry, record acceptance, unlock func() error) {
	defer s.workers.Done()
	defer func() { _ = unlock() }()
	defer plan.prepared.Close()
	s.mu.Lock()
	running := s.running[record.Operation.OperationId]
	running.record.Operation.State = Running
	running.record.Operation.Phase = "preflight"
	ctx := running.ctx
	s.mu.Unlock()
	outcome, panicked, err := executePrepared(ctx, plan.prepared, func(p Progress) { s.progress(record.Operation.OperationId, p) })
	s.mu.Lock()
	record = s.running[record.Operation.OperationId].record
	s.mu.Unlock()
	record.Operation = finishOperation(record.Operation, outcome, err, panicked)
	if err = s.store.save(record); err != nil {
		record.Operation.State = Interrupted
		record.Operation.Result = nil
		record.Operation.Diagnostics = append(record.Operation.Diagnostics, Diagnostic{
			Code: "OUTCOME_NOT_PERSISTED", Message: "The final result could not be saved; inspect before another mutation", Severity: DiagnosticSeverityError})
	}
	if err == nil && record.Operation.State == Succeeded {
		if linkErr := s.reconcileObservations(plan, record.Operation.OperationId, outcome); linkErr != nil {
			record.Operation.Diagnostics = append(record.Operation.Diagnostics, Diagnostic{Code: "RECOVERY_OBSERVATION_UNAVAILABLE", Message: "Files were reconciled but accepted history could not be settled; inspect and prepare recovery again", Severity: DiagnosticSeverityWarning})
			_ = s.store.save(record)
		}
	}
	s.mu.Lock()
	running.cancel()
	delete(s.running, record.Operation.OperationId)
	delete(s.plans, plan.review.PlanId)
	s.mu.Unlock()
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

func finishOperation(op Operation, outcome Outcome, err error, panicked bool) Operation {
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
		op.Diagnostics = append(op.Diagnostics, publicFailure(err))
		if op.State == Succeeded {
			op.State = Interrupted
			return op
		}
	}
	if outcome.Result.Outcome != "" {
		result := outcome.Result
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

func (s *Server) progress(id string, progress Progress) {
	s.mu.Lock()
	defer s.mu.Unlock()
	running := s.running[id]
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

func (s *Server) operation(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !validID(id) {
		writeError(w, 400, "INVALID_REQUEST", "Invalid operation identity", false)
		return
	}
	if op, ok := s.memoryOperation(id); ok {
		loopbackapi.WriteJSON(w, 200, op)
		return
	}
	record, err := s.store.byOperation(id)
	if err != nil {
		writeError(w, 503, "OBSERVATION_UNAVAILABLE", "Accepted-operation history cannot be read safely", true)
		return
	}
	if record == nil {
		writeError(w, 404, "OPERATION_NOT_FOUND", "Unknown or expired operation; inspect environment before retrying effects", false)
		return
	}
	op, err := s.observe(r.Context(), *record)
	if err != nil {
		writeError(w, 503, "OBSERVATION_UNAVAILABLE", "Accepted work cannot be observed safely", true)
		return
	}
	loopbackapi.WriteJSON(w, 200, op)
}

func (s *Server) memoryOperation(id string) (Operation, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if running := s.running[id]; running != nil {
		return cloneOperation(running.record.Operation), true
	}
	return Operation{}, false
}

func cloneOperation(op Operation) Operation {
	op.Items = append([]OperationItem{}, op.Items...)
	op.Diagnostics = append([]Diagnostic{}, op.Diagnostics...)
	if op.Result != nil {
		result := *op.Result
		result.Items = append([]OperationItem{}, result.Items...)
		result.NextActions = append([]NextAction{}, result.NextActions...)
		op.Result = &result
	}
	return op
}

// An accepted record without a live lock is interrupted, not a job to replay.
func (s *Server) observe(ctx context.Context, record acceptance) (Operation, error) {
	if op, ok := s.memoryOperation(record.Operation.OperationId); ok {
		return op, nil
	}
	if terminal(record.Operation.State) {
		return record.Operation, nil
	}
	unlock, err := s.mutationLock.Acquire(ctx)
	if errors.Is(err, processlock.ErrActive) {
		return record.Operation, nil
	}
	if err != nil {
		return Operation{}, err
	}
	defer func() { _ = unlock() }()
	current, err := s.store.byRequest(record.Operation.RequestId)
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
	if err = s.store.save(*current); err != nil {
		return Operation{}, err
	}
	return current.Operation, nil
}

func (s *Server) environmentOperations(ctx context.Context) ([]Operation, []LocalSkillBackup, error) {
	records, err := s.store.list()
	if err != nil {
		return nil, nil, err
	}
	sort.Slice(records, func(i, j int) bool { return records[i].Operation.UpdatedAt.After(records[j].Operation.UpdatedAt) })
	pending, backups := []Operation{}, []LocalSkillBackup{}
	for _, record := range records {
		op := record.Operation
		if pendingOperation(op) {
			op, err = s.observe(ctx, record)
			if err != nil {
				return nil, nil, err
			}
			if pendingOperation(op) {
				pending = append(pending, op)
			}
		}
		if len(backups) < 100 && op.ResolvedBy == "" && op.Result != nil && op.Result.LocalSkillBackup != nil && time.Since(op.UpdatedAt) < resultRetention {
			backups = append(backups, *op.Result.LocalSkillBackup)
		}
	}
	return pending, backups, nil
}

func pendingOperation(op Operation) bool {
	if op.ResolvedBy != "" {
		return false
	}
	if op.State == Queued || op.State == Running || op.State == Interrupted || op.State == Partial {
		return true
	}
	if op.State != Failed && op.State != Cancelled || op.Result == nil {
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

func (s *Server) cancelOperation(w http.ResponseWriter, r *http.Request) {
	if r.ContentLength != 0 {
		var input struct{}
		if !loopbackapi.Decode(w, r, &input, writeError) {
			return
		}
	}
	id := r.PathValue("id")
	if !validID(id) {
		writeError(w, 400, "INVALID_REQUEST", "Invalid operation identity", false)
		return
	}
	s.mu.Lock()
	running := s.running[id]
	if running != nil {
		running.record.Operation.Phase = "cancellation_requested"
		cancel := running.cancel
		op := cloneOperation(running.record.Operation)
		s.mu.Unlock()
		cancel()
		loopbackapi.WriteJSON(w, 202, op)
		return
	}
	s.mu.Unlock()
	record, err := s.store.byOperation(id)
	if err != nil {
		writeError(w, 503, "OBSERVATION_UNAVAILABLE", "Cannot determine operation state", true)
		return
	}
	if record == nil {
		writeError(w, 404, "OPERATION_NOT_FOUND", "Operation not found", false)
		return
	}
	op, err := s.observe(r.Context(), *record)
	if err != nil {
		writeError(w, 503, "OBSERVATION_UNAVAILABLE", "Cannot inspect operation state", true)
		return
	}
	if terminal(op.State) {
		loopbackapi.WriteJSON(w, 202, op)
		return
	}
	writeError(w, 409, "CANCEL_UNAVAILABLE", "This process does not own the running operation; reconnect to its host", false)
}
