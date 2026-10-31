// Package skillspicker owns the short-lived loopback HTTP transport. Installer
// policy and source acquisition are supplied by the CLI composition boundary.
package skillspicker

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/777genius/plugin-kit-ai/cli/internal/loopbackapi"
)

// Prepared keeps the temporary installer workspace alive until apply or cleanup.
// Apply must repeat installer preflight and return ErrPlanStale before effects
// when the expected persistent facts changed.
type Prepared interface {
	Review() Plan
	Check(context.Context) error
	Apply(context.Context) (Outcome, error)
	Close()
}

type Outcome struct {
	State       OperationState
	Items       []OperationItem
	Diagnostics []Diagnostic
	Result      OperationResult
}

type Callbacks struct {
	Progress func() ScanProgress
	Scan     func(context.Context, string) (Catalog, error)
	Preview  func(context.Context, string) (Preview, error)
	Targets  func(context.Context) ([]Target, error)
	Prepare  func(context.Context, PlanRequest) (Prepared, error)
}

type Options struct {
	OnClose      func()
	Source       Source
	Callbacks    Callbacks
	IdleLifetime time.Duration
	PlanLifetime time.Duration
	ScanTimeout  time.Duration
}

var ErrPlanStale = errors.New("installer facts changed; prepare a new review")

type planEntry struct {
	review      Plan
	prepared    Prepared
	operationID string
}

type Server struct {
	mu                      sync.Mutex
	opts                    Options
	ctx                     context.Context
	cancel                  context.CancelFunc
	listener                net.Listener
	http                    *http.Server
	host, origin, token, id string
	state                   SessionState
	touched                 time.Time
	expired                 bool
	busy                    bool
	preparing               bool
	catalog                 *Catalog
	plans                   map[string]*planEntry
	operations              map[string]*Operation
	lastOperation           string
	done                    chan struct{}
	closeOnce               sync.Once
	doneOnce                sync.Once
	workers                 sync.WaitGroup
}

// Start binds a random IPv4 loopback port and starts the first bounded scan.
// Accepted work uses the session context, never a completed HTTP handler context.
func Start(ctx context.Context, opts Options) (*Server, error) {
	if opts.Callbacks.Scan == nil || opts.Callbacks.Preview == nil || opts.Callbacks.Targets == nil || opts.Callbacks.Prepare == nil {
		return nil, errors.New("incomplete picker callbacks")
	}
	if opts.IdleLifetime <= 0 {
		opts.IdleLifetime = 15 * time.Minute
	}
	if opts.PlanLifetime <= 0 {
		opts.PlanLifetime = 5 * time.Minute
	}
	if opts.ScanTimeout <= 0 {
		opts.ScanTimeout = 2 * time.Minute
	}
	listener, err := (&net.ListenConfig{}).Listen(ctx, "tcp4", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	sessionCtx, cancel := context.WithCancel(ctx)
	s := &Server{opts: opts, ctx: sessionCtx, cancel: cancel, listener: listener, host: listener.Addr().String(), token: randomID(), id: randomID(), state: "scanning", touched: time.Now(), plans: map[string]*planEntry{}, operations: map[string]*Operation{}, done: make(chan struct{})}
	s.origin = "http://" + s.host
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/session", s.session)
	mux.HandleFunc("GET /api/v1/catalog", s.getCatalog)
	mux.HandleFunc("POST /api/v1/catalog", s.rescan)
	mux.HandleFunc("GET /api/v1/candidates/{id}/preview", s.preview)
	mux.HandleFunc("GET /api/v1/targets", s.targets)
	mux.HandleFunc("POST /api/v1/plans", s.prepare)
	mux.HandleFunc("POST /api/v1/plans/{id}/apply", s.apply)
	mux.HandleFunc("GET /api/v1/operations/{id}", s.operation)
	mux.HandleFunc("/", serveAssets)
	s.http = &http.Server{Handler: s.guard(mux), ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 30 * time.Second}
	go func() { _ = s.http.Serve(listener) }()
	s.mu.Lock()
	s.startScanLocked(opts.Source.Scope)
	s.mu.Unlock()
	go s.expiryLoop()
	return s, nil
}

func randomID() string                  { return loopbackapi.RandomID() }
func (s *Server) URL() string           { return s.origin + "/#capability=" + s.token }
func (s *Server) Done() <-chan struct{} { return s.done }

// Close cancels scans, waits for accepted installation cleanup, then disposes
// unaccepted temporary reviews. CLI signal handling remains the context owner.
func (s *Server) Close() {
	s.closeOnce.Do(func() {
		s.mu.Lock()
		s.expired = true
		s.mu.Unlock()
		s.cancel()
		_ = s.http.Close()
		s.workers.Wait()
		s.mu.Lock()
		s.cleanupPlansLocked()
		s.expired = true
		s.mu.Unlock()
		if s.opts.OnClose != nil {
			s.opts.OnClose()
		}
		s.doneOnce.Do(func() { close(s.done) })
	})
}
func (s *Server) cleanupPlansLocked() {
	for id, p := range s.plans {
		if p.operationID == "" {
			p.prepared.Close()
			delete(s.plans, id)
		}
	}
}
func (s *Server) expirePlansLocked(now time.Time) {
	if s.preparing {
		return
	}
	for id, p := range s.plans {
		if p.operationID == "" && !now.Before(p.review.ExpiresAt) {
			p.prepared.Close()
			delete(s.plans, id)
		}
	}
}
func (s *Server) expiryLoop() {
	interval := time.Second
	if s.opts.IdleLifetime/4 < interval {
		interval = s.opts.IdleLifetime / 4
	}
	if interval < time.Millisecond {
		interval = time.Millisecond
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-s.ctx.Done():
			go s.Close()
			return
		case now := <-ticker.C:
			s.mu.Lock()
			s.expirePlansLocked(now)
			expired := !s.busy && !s.preparing && now.Sub(s.touched) >= s.opts.IdleLifetime
			if expired {
				s.expired = true
				s.cleanupPlansLocked()
			}
			s.mu.Unlock()
			// Keep the HTTP listener for the next request to observe 410. The owning CLI
			// sees Done and closes the server; no new work can be admitted after expiry.
			if expired {
				s.cancel()
				s.doneOnce.Do(func() { close(s.done) })
				return
			}
		}
	}
}
func (s *Server) guard(next http.Handler) http.Handler {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			s.mu.Lock()
			if s.expired {
				s.mu.Unlock()
				writeError(w, 410, "SESSION_EXPIRED", "This picker session expired; run the CLI again", false)
				return
			}
			s.touched = time.Now()
			s.expirePlansLocked(s.touched)
			s.workers.Add(1)
			s.mu.Unlock()
			defer s.workers.Done()
		}
		next.ServeHTTP(w, r)
	})
	return loopbackapi.Guard(s.host, s.origin, s.token, writeError, handler)
}

func writeJSON(w http.ResponseWriter, status int, v any) { loopbackapi.WriteJSON(w, status, v) }
func writeError(w http.ResponseWriter, status int, code, message string, retry bool) {
	writeJSON(w, status, APIError{Code: code, Message: message, Retryable: retry})
}
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	return loopbackapi.Decode(w, r, v, writeError)
}
func (s *Server) session(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	writeJSON(w, 200, s.sessionLocked())
}
func (s *Server) sessionLocked() Session {
	response := Session{ApiVersion: 1, SessionId: s.id, State: s.state, Source: s.opts.Source, Scan: ScanProgress{}, ExpiresAt: s.touched.Add(s.opts.IdleLifetime).UTC()}
	if s.catalog != nil {
		id := s.catalog.CatalogId
		response.CatalogId = &id
		response.Scan = ScanProgress{Discovered: len(s.catalog.Candidates), Validated: s.catalog.Counts.Available + s.catalog.Counts.Invalid}
		if s.opts.Callbacks.Progress != nil {
			response.Scan = s.opts.Callbacks.Progress()
		}
	}
	if s.lastOperation != "" {
		id := s.lastOperation
		response.OperationId = &id
	}
	return response
}
func (s *Server) getCatalog(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.catalog == nil {
		writeError(w, 409, "CATALOG_NOT_READY", "The catalog scan has not completed", true)
		return
	}
	writeJSON(w, 200, s.catalog)
}
func (s *Server) startScanLocked(scope string) {
	s.busy = true
	s.state = "scanning"
	s.catalog = nil
	s.cleanupPlansLocked()
	s.workers.Add(1)
	go func() {
		defer s.workers.Done()
		ctx, cancel := context.WithTimeout(s.ctx, s.opts.ScanTimeout)
		defer cancel()
		catalog, err := s.opts.Callbacks.Scan(ctx, scope)
		s.mu.Lock()
		defer s.mu.Unlock()
		s.busy = false
		s.touched = time.Now()
		if err != nil {
			s.state = "failed"
			failed := Catalog{CatalogId: randomID(), Repository: s.opts.Source.Repository, Scope: scope, Diagnostics: []Diagnostic{{Code: "SCAN_FAILED", Severity: "error", Message: "Catalog scan failed; narrow the scope or retry"}}}
			if s.opts.Source.SourceRevision != nil {
				failed.SourceRevision = *s.opts.Source.SourceRevision
			}
			normalizeCatalog(&failed)
			s.catalog = &failed
			return
		}
		if catalog.CatalogId == "" {
			catalog.CatalogId = randomID()
		}
		normalizeCatalog(&catalog)
		s.catalog = &catalog
		s.opts.Source.Scope = scope
		s.state = "ready"
	}()
}
func (s *Server) rescan(w http.ResponseWriter, r *http.Request) {
	var input struct {
		ScopePath *string `json:"scopePath"`
	}
	if !decode(w, r, &input) {
		return
	}
	if input.ScopePath == nil || !validScope(*input.ScopePath) {
		writeError(w, 400, "INVALID_REQUEST", "Scope must be a repository-relative path", false)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.expired {
		writeError(w, 410, "SESSION_EXPIRED", "This picker session ended", false)
		return
	}
	if s.busy || s.preparing {
		writeError(w, 409, "OPERATION_IN_PROGRESS", "Wait for the current operation to finish", true)
		return
	}
	s.startScanLocked(*input.ScopePath)
	writeJSON(w, 202, s.sessionLocked())
}
func validScope(p string) bool {
	if len(p) > 4096 || p != strings.TrimSpace(p) || strings.HasPrefix(p, "/") || strings.HasSuffix(p, "/") || strings.Contains(p, "\\") || strings.ContainsRune(p, 0) {
		return false
	}
	for _, part := range strings.Split(p, "/") {
		if part == ".." || part == "." || (part == "" && p != "") {
			return false
		}
	}
	return true
}
func (s *Server) preview(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	s.mu.Lock()
	found := false
	if s.catalog != nil {
		for _, c := range s.catalog.Candidates {
			if c.Id == id {
				found = true
				break
			}
		}
	}
	s.mu.Unlock()
	if !found {
		writeError(w, 404, "CANDIDATE_NOT_FOUND", "Choose a candidate from the current catalog", false)
		return
	}
	preview, err := s.opts.Callbacks.Preview(r.Context(), id)
	if err != nil {
		writeError(w, 422, "PREVIEW_UNAVAILABLE", "The bounded Skill preview could not be read", false)
		return
	}
	normalizePreview(&preview)
	writeJSON(w, 200, preview)
}
func (s *Server) targets(w http.ResponseWriter, r *http.Request) {
	targets, err := s.opts.Callbacks.Targets(r.Context())
	if err != nil {
		writeError(w, 422, "TARGETS_UNAVAILABLE", "Target configuration could not be inspected safely", false)
		return
	}
	if targets == nil {
		targets = []Target{}
	}
	writeJSON(w, 200, Targets{Targets: targets})
}

func normalizeCatalog(c *Catalog) {
	if c.Candidates == nil {
		c.Candidates = []Candidate{}
	}
	if c.Diagnostics == nil {
		c.Diagnostics = []Diagnostic{}
	}
	for i := range c.Candidates {
		if c.Candidates[i].DiagnosticCodes == nil {
			c.Candidates[i].DiagnosticCodes = []string{}
		}
	}
}
func normalizePreview(p *Preview) {
	if p.Resources == nil {
		p.Resources = []Resource{}
	}
	if p.Diagnostics == nil {
		p.Diagnostics = []Diagnostic{}
	}
}
