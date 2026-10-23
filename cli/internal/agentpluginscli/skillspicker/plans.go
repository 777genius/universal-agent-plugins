package skillspicker

import (
	"context"
	"errors"
	"net/http"
	"time"
)

// ReviewRefusal contains only application-selected public guidance. Provider
// errors must never be copied into its message by the composition boundary.
type ReviewRefusal APIError

func (e *ReviewRefusal) Error() string { return e.Message }
func writeReviewError(w http.ResponseWriter, err error) {
	var refusal *ReviewRefusal
	if errors.As(err, &refusal) {
		writeJSON(w, http.StatusUnprocessableEntity, APIError(*refusal))
		return
	}
	writeError(w, http.StatusUnprocessableEntity, "PLAN_REJECTED", "Review one Skill and target at a time to identify the refused selection, or inspect agentplugins deps status for recovery guidance", false)
}

type requestProblem struct {
	status        int
	code, message string
	retryable     bool
}

func (p *requestProblem) write(w http.ResponseWriter) {
	writeError(w, p.status, p.code, p.message, p.retryable)
}
func problem(status int, code, message string) *requestProblem {
	return &requestProblem{status: status, code: code, message: message}
}
func (s *Server) preparationRefusalLocked(input PlanRequest) *requestProblem {
	if s.expired {
		return problem(410, "SESSION_EXPIRED", "This picker session ended")
	}
	if s.busy || s.preparing {
		return &requestProblem{409, "OPERATION_IN_PROGRESS", "Wait for the current operation to finish", true}
	}
	if s.catalog == nil || input.CatalogId != s.catalog.CatalogId || input.CatalogDigest != s.catalog.CatalogDigest {
		return problem(409, "CATALOG_MISMATCH", "The catalog changed; choose Skills again")
	}
	allowed := map[string]bool{}
	for _, candidate := range s.catalog.Candidates {
		allowed[candidate.Id] = candidate.Status == "available"
	}
	for _, id := range input.SelectedIds {
		if !allowed[id] {
			return problem(400, "INVALID_SELECTION", "Only available current catalog candidates may be selected")
		}
	}
	if s.unacceptedPlanCountLocked() >= 16 {
		return problem(409, "REVIEW_LIMIT", "Too many reviews; wait for old reviews to expire")
	}
	return nil
}
func (s *Server) requestWorkContext(r *http.Request) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(r.Context())
	stop := context.AfterFunc(s.ctx, cancel)
	return ctx, func() { stop(); cancel() }
}
func (s *Server) prepareApplication(ctx context.Context, input PlanRequest) (Prepared, error) {
	targets, err := s.opts.Callbacks.Targets(ctx)
	if err != nil {
		return nil, err
	}
	allowed := map[string]bool{}
	for _, target := range targets {
		allowed[target.Id] = target.Available
	}
	for _, id := range input.TargetIds {
		if !allowed[id] {
			return nil, errors.New("invalid target")
		}
	}
	return s.opts.Callbacks.Prepare(ctx, input)
}
func (s *Server) prepare(w http.ResponseWriter, r *http.Request) {
	var input PlanRequest
	if !decode(w, r, &input) {
		return
	}
	if !validIDs(input.SelectedIds, 2000) || !validIDs(input.TargetIds, 128) || len(input.CatalogId) > 256 || len(input.CatalogDigest) > 256 {
		writeError(w, 400, "INVALID_SELECTION", "Choose unique candidates and targets within the selection limit", false)
		return
	}
	s.mu.Lock()
	refusal := s.preparationRefusalLocked(input)
	if refusal != nil {
		s.mu.Unlock()
		refusal.write(w)
		return
	}
	s.preparing = true
	s.workers.Add(1)
	s.mu.Unlock()
	defer s.workers.Done()
	ctx, cancel := s.requestWorkContext(r)
	defer cancel()
	prepared, err := s.prepareApplication(ctx, input)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.preparing = false
	s.touched = time.Now()
	if err != nil {
		if prepared != nil {
			prepared.Close()
		}
		writeReviewError(w, err)
		return
	}
	if s.expired || s.ctx.Err() != nil || ctx.Err() != nil {
		prepared.Close()
		writeError(w, 410, "SESSION_EXPIRED", "This picker session ended", false)
		return
	}
	review := prepared.Review()
	review.PlanId = randomID()
	review.ExpiresAt = time.Now().Add(s.opts.PlanLifetime).UTC()
	normalizePlan(&review)
	s.expirePlansLocked(time.Now())
	s.plans[review.PlanId] = &planEntry{review: review, prepared: prepared}
	s.state = "ready"
	writeJSON(w, 200, review)
}
func validIDs(ids []string, limit int) bool {
	if len(ids) == 0 || len(ids) > limit {
		return false
	}
	seen := map[string]bool{}
	for _, id := range ids {
		if id == "" || len(id) > 256 || seen[id] {
			return false
		}
		seen[id] = true
	}
	return true
}

func (s *Server) applyPlanLocked(id string) (*planEntry, *requestProblem) {
	if s.expired {
		return nil, problem(410, "SESSION_EXPIRED", "This picker session ended")
	}
	plan := s.plans[id]
	if plan != nil && plan.operationID != "" {
		return plan, nil
	}
	if s.busy || s.preparing {
		return nil, &requestProblem{409, "OPERATION_IN_PROGRESS", "Wait for the accepted operation to finish", true}
	}
	if plan == nil || !time.Now().Before(plan.review.ExpiresAt) || !plan.review.Confirmable {
		return nil, problem(409, "PLAN_STALE", "Prepare a fresh confirmable review before installing")
	}
	return plan, nil
}
func (s *Server) createOperationLocked(plan *planEntry, id string) *Operation {
	opID := randomID()
	plan.operationID = opID
	operation := &Operation{OperationId: opID, PlanId: id, State: "queued", Phase: "preflight", Total: len(plan.review.Actions), Items: []OperationItem{}, Diagnostics: []Diagnostic{}}
	for _, action := range plan.review.Actions {
		alias, path := action.Alias, action.DestinationPath
		operation.Items = append(operation.Items, OperationItem{CandidateId: action.CandidateId, TargetId: action.TargetId, Alias: &alias, DestinationPath: &path, Status: "queued"})
	}
	s.operations[opID] = operation
	s.lastOperation = opID
	s.busy = true
	s.state = "installing"
	s.cleanupPlansLocked()
	return operation
}
func (s *Server) apply(w http.ResponseWriter, r *http.Request) {
	if r.ContentLength != 0 {
		writeError(w, 400, "INVALID_REQUEST", "Apply does not accept a request body", false)
		return
	}
	id := r.PathValue("id")
	s.mu.Lock()
	plan, refusal := s.applyPlanLocked(id)
	if refusal != nil {
		s.mu.Unlock()
		refusal.write(w)
		return
	}
	if plan.operationID != "" {
		opID := plan.operationID
		s.mu.Unlock()
		writeJSON(w, 202, ApplyResponse{OperationId: opID})
		return
	}
	// Check is read-only and request-owned. Reserve the session gate throughout.
	s.preparing = true
	s.workers.Add(1)
	s.mu.Unlock()
	ctx, cancel := s.requestWorkContext(r)
	err := plan.prepared.Check(ctx)
	cancel()
	s.mu.Lock()
	s.preparing = false
	if err != nil || s.expired || s.ctx.Err() != nil || !time.Now().Before(plan.review.ExpiresAt) {
		s.mu.Unlock()
		s.workers.Done()
		writeError(w, 409, "PLAN_STALE", "Installer facts or review lifetime changed; prepare a new review", false)
		return
	}
	operation := s.createOperationLocked(plan, id)
	s.mu.Unlock()
	// Accepted work owns the worker reservation before writing HTTP 202.
	go s.runApply(plan, operation)
	writeJSON(w, 202, ApplyResponse{OperationId: operation.OperationId})
}
func (s *Server) runApply(plan *planEntry, operation *Operation) {
	defer s.workers.Done()
	defer plan.prepared.Close()
	s.mu.Lock()
	operation.State = "running"
	operation.Phase = "installing"
	s.mu.Unlock()
	outcome, err := plan.prepared.Apply(s.ctx)
	s.mu.Lock()
	defer s.mu.Unlock()
	operation.State = outcome.State
	if operation.State != "succeeded" && operation.State != "partial" && operation.State != "failed" {
		operation.State = "failed"
	}
	if len(outcome.Items) > 0 {
		operation.Items = outcome.Items
	} else if err != nil {
		for i := range operation.Items {
			operation.Items[i].Status = "skipped"
		}
	}
	for i := range operation.Items {
		if !operation.Items[i].Status.Valid() {
			operation.Items[i].Status = "failed"
		}
	}
	if outcome.Diagnostics != nil {
		operation.Diagnostics = outcome.Diagnostics
	}
	if err != nil {
		if operation.State != "partial" {
			operation.State = "failed"
		}
		code := "INSTALL_FAILED"
		message := "Installation failed; inspect recovery guidance before retrying"
		if errors.Is(err, ErrPlanStale) {
			code = "PLAN_STALE"
			message = "Installer facts changed; prepare a new review"
		}
		operation.Diagnostics = append(operation.Diagnostics, Diagnostic{Code: code, Message: message, Severity: "error"})
	}
	if outcome.Result.RepairInstructions == nil {
		outcome.Result.RepairInstructions = []string{}
	}
	operation.Result = &outcome.Result
	operation.Completed = len(operation.Items)
	operation.Phase = "complete"
	s.busy = false
	s.touched = time.Now()
	if operation.State == "succeeded" {
		s.state = "done"
	} else {
		s.state = "failed"
	}
}
func (s *Server) operation(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	operation := s.operations[r.PathValue("id")]
	if operation == nil {
		writeError(w, 404, "OPERATION_NOT_FOUND", "The operation does not belong to this session", false)
		return
	}
	writeJSON(w, 200, operation)
}

func normalizePlan(p *Plan) {
	if p.Actions == nil {
		p.Actions = []Action{}
	}
	if p.Conflicts == nil {
		p.Conflicts = []Diagnostic{}
	}
	if p.Warnings == nil {
		p.Warnings = []Diagnostic{}
	}
	if len(p.Conflicts) > 0 {
		p.Confirmable = false
	}
	for i := range p.Actions {
		if p.Actions[i].ReasonCodes == nil {
			p.Actions[i].ReasonCodes = []string{}
		}
		switch p.Actions[i].Action {
		case "install", "update", "unchanged":
		default:
			p.Confirmable = false
		}
	}
}

func (s *Server) unacceptedPlanCountLocked() int {
	count := 0
	for _, plan := range s.plans {
		if plan.operationID == "" {
			count++
		}
	}
	return count
}
