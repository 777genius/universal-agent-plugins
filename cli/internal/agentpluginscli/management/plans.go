package management

import (
	"context"
	"errors"
	"time"
)

func (a *Application) Prepare(ctx context.Context, input ChangeRequest) (Plan, error) {
	ctx, leave, err := a.admit(ctx)
	if err != nil {
		return Plan{}, err
	}
	defer leave()
	if !validChange(input) {
		return Plan{}, refusal(400, "INVALID_REQUEST", "Invalid action or bounded selection", false)
	}
	if err := a.reservePreparation(true); err != nil {
		return Plan{}, err
	}
	defer a.releasePreparation()
	ctx, cancel := operationContext(ctx)
	defer cancel()
	input = cloneChange(input)
	if input.Action == Recover && input.OperationId != "" {
		if err := a.validateRecovery(input); err != nil {
			return Plan{}, err
		}
	}
	operationID := randomID()
	prepared, err := a.opts.Backend.Prepare(ctx, cloneChange(input), operationID)
	if err != nil {
		if prepared != nil {
			prepared.Close()
		}
		return Plan{}, err
	}
	return a.retainPrepared(ctx, input, operationID, prepared, nil, nil)
}

// retainPrepared is the sole ownership transfer from preparation to retained
// review. Public snapshots contain no mutable references to retained authority.
func (a *Application) retainPrepared(ctx context.Context, input ChangeRequest, operationID string, prepared Prepared, localSkill *LocalSkillIdentity, restorePoint *LocalSkillBackup) (Plan, error) {
	if prepared == nil {
		return Plan{}, refusal(500, "INVALID_PREPARATION", "No prepared authority was retained", false)
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed || ctx.Err() != nil {
		prepared.Close()
		return Plan{}, refusal(410, "SESSION_EXPIRED", "Preparation was interrupted", false)
	}
	review := clonePlan(prepared.Review())
	review.PlanId = randomID()
	review.ContextId = a.opts.ContextID
	review.ExpiresAt = time.Now().Add(a.opts.PlanLifetime).UTC()
	review.Action = input.Action
	review.RestorePoint = cloneBackup(restorePoint)
	normalizePlan(&review)
	a.plans[review.PlanId] = &planEntry{localSkill: localSkill, review: review, prepared: prepared, operationID: operationID, request: cloneChange(input)}
	return clonePlan(review), nil
}

func (a *Application) reservePreparation(needsReviewSlot bool) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed || a.preparing || len(a.running) > 0 {
		return refusal(409, "OPERATION_IN_PROGRESS", "Wait for the current operation or preparation", true)
	}
	count := 0
	for _, p := range a.plans {
		if p.acceptedRequest == "" {
			count++
		}
	}
	if needsReviewSlot && count >= 16 {
		return refusal(409, "REVIEW_LIMIT", "Wait for old reviews to expire", true)
	}
	a.preparing = true
	return nil
}
func (a *Application) releasePreparation() { a.mu.Lock(); a.preparing = false; a.mu.Unlock() }

func validChange(input ChangeRequest) bool {
	if !input.Action.Valid() || input.Action == LocalSkill || !validIDs(input.TargetIds, 128) || !validIDs(input.Aliases, 2000) || !validIDs(input.CandidateIds, 2000) {
		return false
	}
	if len(input.Source) > 2048 || len(input.Ref) > 256 || len(input.Alias) > 256 || len(input.CatalogId) > 256 {
		return false
	}
	if input.Action == Recover {
		return (input.OperationId == "" || ValidID(input.OperationId)) && len(input.Aliases) == 0 && len(input.TargetIds) == 0
	}
	return input.OperationId == ""
}
func (a *Application) validateRecovery(input ChangeRequest) error {
	record, err := a.opts.Store.ByOperation(input.OperationId)
	if err != nil {
		return refusal(503, "OBSERVATION_UNAVAILABLE", "Cannot inspect the accepted operation", true)
	}
	if record == nil {
		return refusal(404, "OPERATION_NOT_FOUND", "The operation does not belong to this environment", false)
	}
	if record.Operation.LocalSkill != nil {
		return refusal(409, "LOCAL_RECOVERY_REQUIRED", "Prepare a local Skill restore; native recovery cannot settle local file effects", false)
	}
	if record.IntentRoot != a.opts.IntentRoot {
		return refusal(409, "RECOVERY_CONTEXT_CHANGED", "Reopen the original dependency root to reconcile this accepted operation", false)
	}
	return nil
}
func normalizePlan(p *Plan) {
	if p.Actions == nil {
		p.Actions = []PlanAction{}
	}
	if p.Warnings == nil {
		p.Warnings = []Diagnostic{}
	}
	if p.Conflicts == nil {
		p.Conflicts = []Diagnostic{}
	}
	if len(p.Conflicts) > 0 {
		p.Confirmable = false
	}
}

func (a *Application) Apply(ctx context.Context, planID, requestID string) (Operation, error) {
	ctx, leave, err := a.admit(ctx)
	if err != nil {
		return Operation{}, err
	}
	defer leave()
	if !ValidID(planID) || !ValidRequestID(requestID) {
		return Operation{}, refusal(400, "INVALID_REQUEST", "Invalid plan or request identity", false)
	}
	// Accepted identities are replayed before plan lookup or expiry validation.
	if op, found, err := a.replay(ctx, planID, requestID); found || err != nil {
		return op, err
	}
	if err := a.reservePreparation(false); err != nil {
		return Operation{}, err
	}
	defer a.releasePreparation()
	unlock, err := a.opts.Lock.Acquire(ctx)
	if err != nil {
		return Operation{}, refusal(409, "OPERATION_IN_PROGRESS", "Another management process owns a mutation; inspect or retry the same request", true)
	}
	handedOff := false
	defer func() {
		if !handedOff {
			_ = unlock()
		}
	}()
	if op, found, err := a.replay(ctx, planID, requestID); found || err != nil {
		return op, err
	}
	plan, err := a.planForApply(planID, requestID)
	if err != nil {
		return Operation{}, err
	}
	if err := a.checkPrepared(ctx, plan.prepared); err != nil {
		return Operation{}, ErrPlanStale
	}
	record, err := a.accept(plan, requestID)
	if err != nil {
		return Operation{}, err
	}
	handedOff = true
	go a.runApply(plan, record, unlock)
	return cloneOperation(record.Operation), nil
}
func (a *Application) replay(ctx context.Context, planID, requestID string) (Operation, bool, error) {
	record, err := a.opts.Store.ByRequest(requestID)
	if err != nil {
		return Operation{}, false, refusal(503, "OBSERVATION_UNAVAILABLE", "Cannot determine whether the request was accepted; retain its identity", true)
	}
	if record == nil {
		return Operation{}, false, nil
	}
	if record.Fingerprint != digest(planID) {
		return Operation{}, true, refusal(409, "REQUEST_ID_CONFLICT", "This request identity was accepted for different input", false)
	}
	op, err := a.observe(ctx, *record)
	if err != nil {
		return Operation{}, true, refusal(503, "OBSERVATION_UNAVAILABLE", "Cannot observe accepted work; retain its identity", true)
	}
	return cloneOperation(op), true, nil
}
func (a *Application) planForApply(id, requestID string) (*planEntry, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	plan := a.plans[id]
	if plan == nil || !plan.review.Confirmable || !time.Now().Before(plan.review.ExpiresAt) {
		return nil, refusal(409, "PLAN_STALE", "Prepare a fresh confirmable review", false)
	}
	if plan.acceptedRequest != "" && plan.acceptedRequest != requestID {
		return nil, refusal(409, "PLAN_ALREADY_ACCEPTED", "This plan was already accepted; observe the original request", false)
	}
	return plan, nil
}
func (a *Application) accept(plan *planEntry, requestID string) (Observation, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed || a.ctx.Err() != nil || !time.Now().Before(plan.review.ExpiresAt) {
		return Observation{}, ErrPlanStale
	}
	now := time.Now().UTC()
	record := Observation{Version: 1, EnvironmentID: a.opts.EnvironmentID, IntentRoot: a.opts.IntentRoot, ContextID: a.opts.ContextID, Fingerprint: digest(plan.review.PlanId), InstallerIDs: append([]string(nil), plan.prepared.OperationIDs()...), Operation: Operation{
		EnvironmentId: a.opts.EnvironmentID, Action: plan.review.Action, LocalSkill: plan.localSkill, OperationId: plan.operationID, PlanId: plan.review.PlanId, RequestId: requestID, ContextId: a.opts.ContextID,
		State: Queued, Phase: "preflight", Items: []OperationItem{}, Diagnostics: []Diagnostic{}, Total: len(plan.review.Actions), CreatedAt: now, UpdatedAt: now,
	}}
	if err := a.admitMutation(plan); err != nil {
		return Observation{}, err
	}
	if err := a.reserveHistory(now); err != nil {
		if errors.Is(err, ErrHistoryFull) {
			return Observation{}, refusal(409, "HISTORY_FULL", "Retained operation history is full; unresolved identities cannot be discarded", false)
		}
		return Observation{}, refusal(503, "OBSERVATION_UNAVAILABLE", "Retained operation history cannot be read safely", true)
	}
	if err := a.opts.Store.Save(cloneObservation(record)); err != nil {
		// Atomic publication can be visible even after an error. Preserve the request
		// identity and any visible observation; no worker or effect starts here.
		return Observation{}, refusal(503, "ACCEPTANCE_UNAVAILABLE", "Acceptance could not be persisted; retain this request identity before retrying", true)
	}
	ctx, cancel := context.WithCancel(a.ctx)
	plan.acceptedRequest = requestID
	a.running[plan.operationID] = &runningOperation{record: cloneObservation(record), ctx: ctx, cancel: cancel}
	a.workers.Add(1)
	return record, nil
}

func publicFailure(err error, active bool) Diagnostic {
	code, message := "OPERATION_FAILED", "Operation did not complete; inspect the result before retrying"
	if errors.Is(err, ErrPlanStale) {
		code, message = "PLAN_STALE", "Installer facts changed; prepare a fresh review"
	}
	if active {
		code, message = "OPERATION_IN_PROGRESS", "Another installer owns this environment"
	}
	return Diagnostic{Code: code, Message: message, Severity: DiagnosticSeverityError}
}
func (a *Application) DiscardPlan(ctx context.Context, id string) error {
	_, leave, err := a.admit(ctx)
	if err != nil {
		return err
	}
	defer leave()
	if !ValidID(id) {
		return refusal(400, "INVALID_REQUEST", "Invalid plan identity", false)
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	p := a.plans[id]
	if p == nil {
		return nil
	}
	if p.acceptedRequest != "" || a.preparing {
		return refusal(409, "PLAN_IN_USE", "An accepted or currently checked plan cannot be discarded", false)
	}
	p.prepared.Close()
	delete(a.plans, id)
	return nil
}

// admitMutation runs with both the application mutex and mutation lock held.
func (a *Application) admitMutation(plan *planEntry) error {
	if plan.request.Action == Recover || plan.localSkill != nil && plan.request.OperationId != "" {
		return nil
	}
	records, err := a.opts.Store.List()
	if err != nil {
		return refusal(503, "OBSERVATION_UNAVAILABLE", "Cannot prove that accepted work is settled", true)
	}
	for _, record := range records {
		if pendingOperation(record.Operation) {
			return refusal(409, "RECOVERY_REQUIRED", "Inspect and prepare recovery for accepted work before a new mutation", false)
		}
	}
	return nil
}
func (a *Application) checkPrepared(ctx context.Context, prepared Prepared) error {
	ctx, cancel := operationContext(ctx)
	defer cancel()
	return prepared.Check(ctx)
}
func (a *Application) forgetObservation(record Observation) error {
	if record.Operation.LocalSkill == nil {
		return nil
	}
	backend, ok := a.opts.Backend.(interface {
		ForgetObservation(context.Context, Operation) error
	})
	if !ok {
		return errors.New("local observation cleanup is unavailable")
	}
	return backend.ForgetObservation(a.ctx, cloneOperation(record.Operation))
}
