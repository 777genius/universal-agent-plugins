package management

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/777genius/plugin-kit-ai/cli/internal/loopbackapi"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/processlock"
)

func (s *Server) prepare(w http.ResponseWriter, r *http.Request) {
	var input ChangeRequest
	if !loopbackapi.Decode(w, r, &input, writeError) {
		return
	}
	if !validChange(input) {
		writeError(w, 400, "INVALID_REQUEST", "Invalid action or bounded selection", false)
		return
	}
	if !s.reservePreparation(w, true) {
		return
	}
	defer func() { s.mu.Lock(); s.preparing = false; s.mu.Unlock() }()
	ctx, cancel := s.requestContext(r)
	defer cancel()
	if input.Action == Recover && input.OperationId != "" && !s.validateRecovery(w, input) {
		return
	}
	operationID := loopbackapi.RandomID()
	prepared, err := s.opts.Backend.Prepare(ctx, input, operationID)
	if err != nil {
		if prepared != nil {
			prepared.Close()
		}
		writeBackendError(w, err)
		return
	}
	s.retainPrepared(ctx, w, input, operationID, prepared, nil)
}

// retainPrepared is the sole place where a backend's retained authority receives
// a public review identity. Ordinary and local Skill changes share this lifecycle.
func (s *Server) retainPrepared(ctx context.Context, w http.ResponseWriter, input ChangeRequest, operationID string, prepared Prepared, localSkill *LocalSkillIdentity) {
	if prepared == nil {
		writeError(w, 500, "INVALID_PREPARATION", "No prepared authority was retained", false)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || ctx.Err() != nil {
		prepared.Close()
		writeError(w, 410, "SESSION_EXPIRED", "Preparation was interrupted", false)
		return
	}
	review := prepared.Review()
	review.PlanId = loopbackapi.RandomID()
	review.ContextId = s.opts.ContextID
	review.ExpiresAt = time.Now().Add(s.opts.PlanLifetime).UTC()
	review.Action = input.Action
	normalizePlan(&review)
	s.plans[review.PlanId] = &planEntry{localSkill: localSkill, review: review, prepared: prepared, operationID: operationID, request: input}
	loopbackapi.WriteJSON(w, 200, review)
}

func (s *Server) reservePreparation(w http.ResponseWriter, needsReviewSlot bool) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.preparing || len(s.running) > 0 {
		writeError(w, 409, "OPERATION_IN_PROGRESS", "Wait for the current operation or preparation", true)
		return false
	}
	count := 0
	for _, p := range s.plans {
		if p.acceptedRequest == "" {
			count++
		}
	}
	// Applying retained authority consumes no new review slot. Reaching the
	// preparation limit must not prevent an existing plan from being applied.
	if needsReviewSlot && count >= 16 {
		writeError(w, 409, "REVIEW_LIMIT", "Wait for old reviews to expire", true)
		return false
	}
	s.preparing = true
	return true
}

func validChange(input ChangeRequest) bool {
	if !input.Action.Valid() || input.Action == LocalSkill || !validIDs(input.TargetIds, 128) || !validIDs(input.Aliases, 2000) || !validIDs(input.CandidateIds, 2000) {
		return false
	}
	if len(input.Source) > 2048 || len(input.Ref) > 256 || len(input.Alias) > 256 || len(input.CatalogId) > 256 {
		return false
	}
	if input.Action == Recover {
		return (input.OperationId == "" || validID(input.OperationId)) && len(input.Aliases) == 0 && len(input.TargetIds) == 0
	}
	return input.OperationId == ""
}

func (s *Server) validateRecovery(w http.ResponseWriter, input ChangeRequest) bool {
	record, err := s.store.byOperation(input.OperationId)
	if err != nil {
		writeError(w, 503, "OBSERVATION_UNAVAILABLE", "Cannot inspect the accepted operation", true)
		return false
	}
	if record == nil {
		writeError(w, 404, "OPERATION_NOT_FOUND", "The operation does not belong to this environment", false)
		return false
	}
	if record.Operation.LocalSkill != nil {
		writeError(w, 409, "LOCAL_RECOVERY_REQUIRED", "Prepare a local Skill restore; native recovery cannot settle local file effects", false)
		return false
	}
	if record.IntentRoot != s.opts.IntentRoot {
		writeError(w, 409, "RECOVERY_CONTEXT_CHANGED", "Reopen the original dependency root to reconcile this accepted operation", false)
		return false
	}
	return true
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

func (s *Server) apply(w http.ResponseWriter, r *http.Request) {
	var input ApplyRequest
	if !loopbackapi.Decode(w, r, &input, writeError) {
		return
	}
	id := r.PathValue("id")
	if !validID(id) || !validRequestID(input.RequestId) {
		writeError(w, 400, "INVALID_REQUEST", "Invalid plan or request identity", false)
		return
	}
	// This lookup intentionally precedes plan lookup and expiry validation.
	if s.replay(r.Context(), w, id, input.RequestId) {
		return
	}
	if !s.reservePreparation(w, false) {
		return
	}
	defer func() { s.mu.Lock(); s.preparing = false; s.mu.Unlock() }()
	unlock, err := s.mutationLock.Acquire(r.Context())
	if err != nil {
		writeError(w, 409, "OPERATION_IN_PROGRESS", "Another management process owns a mutation; inspect or retry the same request", true)
		return
	}
	handedOff := false
	defer func() {
		if !handedOff {
			_ = unlock()
		}
	}()
	if s.replay(r.Context(), w, id, input.RequestId) {
		return
	}
	plan := s.planForApply(w, id, input.RequestId)
	if plan == nil {
		return
	}
	if err = s.checkPrepared(r, plan.prepared); err != nil {
		writeBackendError(w, ErrPlanStale)
		return
	}
	record, err := s.accept(plan, input.RequestId)
	if err != nil {
		writeBackendError(w, err)
		return
	}
	handedOff = true
	go s.runApply(plan, record, unlock)
	loopbackapi.WriteJSON(w, http.StatusAccepted, record.Operation)
}

func (s *Server) replay(ctx context.Context, w http.ResponseWriter, planID, requestID string) bool {
	record, err := s.store.byRequest(requestID)
	if err != nil {
		writeError(w, 503, "OBSERVATION_UNAVAILABLE", "Cannot determine whether the request was accepted; retain its identity", true)
		return true
	}
	if record == nil {
		return false
	}
	if record.Fingerprint != digest(planID) {
		writeError(w, 409, "REQUEST_ID_CONFLICT", "This request identity was accepted for different input", false)
		return true
	}
	op, err := s.observe(ctx, *record)
	if err != nil {
		writeError(w, 503, "OBSERVATION_UNAVAILABLE", "Cannot observe accepted work; retain its identity", true)
		return true
	}
	loopbackapi.WriteJSON(w, 202, op)
	return true
}

func (s *Server) planForApply(w http.ResponseWriter, id, requestID string) *planEntry {
	s.mu.Lock()
	defer s.mu.Unlock()
	plan := s.plans[id]
	if plan == nil || !plan.review.Confirmable || !time.Now().Before(plan.review.ExpiresAt) {
		writeError(w, 409, "PLAN_STALE", "Prepare a fresh confirmable review", false)
		return nil
	}
	if plan.acceptedRequest != "" && plan.acceptedRequest != requestID {
		writeError(w, 409, "PLAN_ALREADY_ACCEPTED", "This plan was already accepted; observe the original request", false)
		return nil
	}
	return plan
}

func (s *Server) accept(plan *planEntry, requestID string) (acceptance, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.ctx.Err() != nil || !time.Now().Before(plan.review.ExpiresAt) {
		return acceptance{}, ErrPlanStale
	}
	now := time.Now().UTC()
	record := acceptance{Version: 1, EnvironmentID: s.opts.EnvironmentID, IntentRoot: s.opts.IntentRoot, ContextID: s.opts.ContextID, Fingerprint: digest(plan.review.PlanId),
		InstallerIDs: plan.prepared.OperationIDs(), Operation: Operation{
			EnvironmentId: s.opts.EnvironmentID, Action: plan.review.Action, LocalSkill: plan.localSkill, OperationId: plan.operationID, PlanId: plan.review.PlanId, RequestId: requestID, ContextId: s.opts.ContextID,
			State: Queued, Phase: "preflight", Items: []OperationItem{}, Diagnostics: []Diagnostic{},
			Total: len(plan.review.Actions), CreatedAt: now, UpdatedAt: now,
		}}
	if err := s.admitMutation(plan); err != nil {
		return acceptance{}, err
	}
	if err := s.store.reserve(now, s.forgetObservation); err != nil {
		if errors.Is(err, errHistoryFull) {
			return acceptance{}, &Refusal{Code: "HISTORY_FULL", Message: "Retained operation history is full; unresolved identities cannot be discarded", Status: 409}
		}
		return acceptance{}, &Refusal{Code: "OBSERVATION_UNAVAILABLE", Retryable: true, Message: "Retained operation history cannot be read safely", Status: 503}
	}
	if err := s.store.save(record); err != nil {
		return acceptance{}, &Refusal{Code: "ACCEPTANCE_UNAVAILABLE", Retryable: true, Message: "Acceptance could not be persisted; retain this request identity before retrying", Status: 503}
	}
	ctx, cancel := context.WithCancel(s.ctx)
	plan.acceptedRequest = requestID
	s.running[plan.operationID] = &runningOperation{record: record, ctx: ctx, cancel: cancel}
	s.workers.Add(1)
	// Store the accepted context cancellation in the running record, not HTTP request.
	return record, nil
}

func publicFailure(err error) Diagnostic {
	code, message := "OPERATION_FAILED", "Operation did not complete; inspect the result before retrying"
	if errors.Is(err, ErrPlanStale) {
		code, message = "PLAN_STALE", "Installer facts changed; prepare a fresh review"
	}
	if errors.Is(err, processlock.ErrActive) {
		code, message = "OPERATION_IN_PROGRESS", "Another installer owns this environment"
	}
	return Diagnostic{Code: code, Message: message, Severity: DiagnosticSeverityError}
}

func (s *Server) discardPlan(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !validID(id) {
		writeError(w, 400, "INVALID_REQUEST", "Invalid plan identity", false)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	p := s.plans[id]
	if p == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if p.acceptedRequest != "" || s.preparing {
		writeError(w, 409, "PLAN_IN_USE", "An accepted or currently checked plan cannot be discarded", false)
		return
	}
	p.prepared.Close()
	delete(s.plans, id)
	w.WriteHeader(http.StatusNoContent)
}

// admitMutation runs while both the session mutex and cross-process management
// lock are held. A new request may not bypass durable uncertainty after restart.
// Fresh recovery reviews are permitted to settle each category independently.
func (s *Server) admitMutation(plan *planEntry) error {
	if plan.request.Action == Recover || plan.localSkill != nil && plan.request.OperationId != "" {
		return nil
	}
	records, err := s.store.list()
	if err != nil {
		return &Refusal{Code: "OBSERVATION_UNAVAILABLE", Retryable: true, Message: "Cannot prove that accepted work is settled", Status: 503}
	}
	for _, record := range records {
		if pendingOperation(record.Operation) {
			return &Refusal{Code: "RECOVERY_REQUIRED", Message: "Inspect and prepare recovery for accepted work before a new mutation", Status: 409}
		}
	}
	return nil
}

func (s *Server) checkPrepared(r *http.Request, prepared Prepared) error {
	ctx, cancel := s.requestContext(r)
	defer cancel()
	return prepared.Check(ctx)
}

// Expiry removes only verified private undo data; accepted uncertainty is never
// eligible. Implementations without local-authoring support cannot prune it.
func (s *Server) forgetObservation(record acceptance) error {
	if record.Operation.LocalSkill == nil {
		return nil
	}
	backend, ok := s.opts.Backend.(interface {
		ForgetObservation(context.Context, Operation) error
	})
	if !ok {
		return errors.New("local observation cleanup is unavailable")
	}
	return backend.ForgetObservation(s.ctx, record.Operation)
}
