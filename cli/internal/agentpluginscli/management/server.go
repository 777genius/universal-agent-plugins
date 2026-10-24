package management

import (
	"context"
	"errors"
	"net"
	"net/http"
	"path/filepath"
	"sync"
	"time"

	"github.com/777genius/plugin-kit-ai/cli/internal/loopbackapi"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/processlock"
)

type Options struct {
	EnvironmentID string
	IntentRoot    string
	Backend       Backend
	ContextID     string
	StateRoot     string
	EngineVersion string
	PlanLifetime  time.Duration
}

type planEntry struct {
	localSkill      *LocalSkillIdentity
	review          Plan
	prepared        Prepared
	operationID     string
	request         ChangeRequest
	acceptedRequest string
}

type runningOperation struct {
	record acceptance
	ctx    context.Context
	cancel context.CancelFunc
}

// Server hosts one immutable captured environment. The durable store observes
// existing installer work; it never replays a job after a process restart.
type Server struct {
	opts                Options
	ctx                 context.Context
	cancel              context.CancelFunc
	listener            net.Listener
	http                *http.Server
	origin, host, token string
	store               observationStore
	mutationLock        processlock.Lock
	mu                  sync.Mutex
	closed, preparing   bool
	plans               map[string]*planEntry
	running             map[string]*runningOperation
	workers             sync.WaitGroup
	closeOnce           sync.Once
	done                chan struct{}
}

func Start(ctx context.Context, opts Options) (*Server, error) {
	if opts.Backend == nil || opts.ContextID == "" || opts.EnvironmentID == "" || !filepath.IsAbs(opts.StateRoot) || !filepath.IsAbs(opts.IntentRoot) {
		return nil, errors.New("management requires a backend, captured context and absolute state root")
	}
	if opts.PlanLifetime <= 0 {
		opts.PlanLifetime = 5 * time.Minute
	}
	listener, err := (&net.ListenConfig{}).Listen(ctx, "tcp4", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	sessionCtx, cancel := context.WithCancel(ctx)
	s := &Server{opts: opts, ctx: sessionCtx, cancel: cancel, listener: listener,
		host: listener.Addr().String(), token: loopbackapi.RandomID(),
		store:        newObservationStore(opts.StateRoot, opts.EnvironmentID),
		mutationLock: processlock.Lock{Path: filepath.Join(opts.StateRoot, "management.lock")},
		plans:        map[string]*planEntry{}, running: map[string]*runningOperation{}, done: make(chan struct{})}
	s.origin = "http://" + s.host
	s.http = &http.Server{Handler: s.guard(s.routes()), ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 30 * time.Second}
	go func() { _ = s.http.Serve(listener) }()
	go s.expiryLoop()
	return s, nil
}

func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()
	s.registerLocalSkillsRoutes(mux)
	mux.HandleFunc("GET /api/v1/session", s.session)
	mux.HandleFunc("GET /api/v1/catalog", s.catalog)
	mux.HandleFunc("GET /api/v1/catalog/{id}", s.catalogDetail)
	mux.HandleFunc("POST /api/v1/skills/catalog", s.skillsCatalog)
	mux.HandleFunc("DELETE /api/v1/skills/catalog/{catalogId}", s.discardSkillsCatalog)
	mux.HandleFunc("GET /api/v1/skills/catalog/{catalogId}/candidates/{candidateId}/preview", s.skillsPreview)
	mux.HandleFunc("GET /api/v1/environment", s.environment)
	mux.HandleFunc("POST /api/v1/plans", s.prepare)
	mux.HandleFunc("POST /api/v1/plans/{id}/apply", s.apply)
	mux.HandleFunc("DELETE /api/v1/plans/{id}", s.discardPlan)
	mux.HandleFunc("GET /api/v1/operations/{id}", s.operation)
	mux.HandleFunc("POST /api/v1/operations/{id}/cancel", s.cancelOperation)
	return mux
}

func (s *Server) Bootstrap() Bootstrap {
	return Bootstrap{ApiVersion: N1, ContextId: s.opts.ContextID, EngineVersion: s.opts.EngineVersion,
		Origin: s.origin, Url: s.origin, Token: s.token}
}

func (s *Server) URL() string           { return s.origin }
func (s *Server) Done() <-chan struct{} { return s.done }

// Close is idempotent. The host bounds its wait and may terminate only this
// process; durable accepted identities survive abrupt termination.
func (s *Server) Close() {
	s.closeOnce.Do(func() {
		s.mu.Lock()
		s.closed = true
		s.mu.Unlock()
		s.cancel()
		_ = s.http.Close()
		s.workers.Wait()
		s.mu.Lock()
		for _, p := range s.plans {
			if p.acceptedRequest == "" {
				p.prepared.Close()
			}
		}
		s.plans = map[string]*planEntry{}
		s.mu.Unlock()
		if closer, ok := s.opts.Backend.(interface{ Close() }); ok {
			closer.Close()
		}
		close(s.done)
	})
}

func (s *Server) guard(next http.Handler) http.Handler {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		if s.closed {
			s.mu.Unlock()
			writeError(w, 410, "SESSION_EXPIRED", "This management process is shutting down", false)
			return
		}
		s.workers.Add(1)
		s.mu.Unlock()
		defer s.workers.Done()
		next.ServeHTTP(w, r)
	})
	return loopbackapi.Guard(s.host, s.origin, s.token, writeError, handler)
}

func (s *Server) expiryLoop() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-s.ctx.Done():
			go s.Close()
			return
		case now := <-ticker.C:
			s.mu.Lock()
			if !s.preparing {
				for id, p := range s.plans {
					if p.acceptedRequest == "" && !now.Before(p.review.ExpiresAt) {
						p.prepared.Close()
						delete(s.plans, id)
					}
				}
			}
			s.mu.Unlock()
		}
	}
}

func (s *Server) requestContext(r *http.Request) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	stop := context.AfterFunc(s.ctx, cancel)
	return ctx, func() { stop(); cancel() }
}

func writeError(w http.ResponseWriter, status int, code, message string, retryable bool) {
	loopbackapi.WriteJSON(w, status, APIError{Code: code, Message: message, Retryable: retryable})
}
