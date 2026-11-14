// Package managementhttp adapts the Management application to private loopback HTTP.
package managementhttp

import (
	"context"
	"errors"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/management"
	"github.com/777genius/plugin-kit-ai/cli/internal/loopbackapi"
)

// Server owns its listener and, after successful Start, application shutdown.
// Requests are tracked only to drain transport handlers; the application owns
// all retained authority, admitted application calls and operation workers.
type Server struct {
	application         *management.Application
	http                *http.Server
	origin, host, token string
	mu                  sync.Mutex
	closed              bool
	requests            sync.WaitGroup
	closeOnce           sync.Once
	done                chan struct{}
}

// Start leaves application ownership with the caller if the listener cannot
// start. A successfully returned server closes the application on shutdown.
func Start(ctx context.Context, application *management.Application) (*Server, error) {
	if application == nil {
		return nil, errors.New("management HTTP requires an application")
	}
	listener, err := (&net.ListenConfig{}).Listen(ctx, "tcp4", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	s := &Server{application: application, host: listener.Addr().String(), token: loopbackapi.RandomID(), done: make(chan struct{})}
	s.origin = "http://" + s.host
	s.http = &http.Server{Handler: s.guard(s.routes()), ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 30 * time.Second}
	go func() { _ = s.http.Serve(listener) }()
	go func() {
		select {
		case <-ctx.Done():
			s.Close()
		case <-application.Stopping():
			s.Close()
		case <-s.done:
		}
	}()
	return s, nil
}

func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/session", s.session)
	mux.HandleFunc("GET /api/v1/catalog", s.catalog)
	mux.HandleFunc("GET /api/v1/catalog/{id}", s.catalogDetail)
	mux.HandleFunc("POST /api/v1/skills/catalog", s.skillsCatalog)
	mux.HandleFunc("DELETE /api/v1/skills/catalog/{catalogId}", s.discardSkillsCatalog)
	mux.HandleFunc("GET /api/v1/skills/catalog/{catalogId}/candidates/{candidateId}/preview", s.skillsPreview)
	mux.HandleFunc("POST /api/v1/skills/local/plans", s.prepareLocalSkill)
	mux.HandleFunc("GET /api/v1/environment", s.environment)
	mux.HandleFunc("POST /api/v1/environment/check-updates", s.checkUpdates)
	mux.HandleFunc("POST /api/v1/plans", s.prepare)
	mux.HandleFunc("POST /api/v1/plans/{id}/apply", s.apply)
	mux.HandleFunc("DELETE /api/v1/plans/{id}", s.discardPlan)
	mux.HandleFunc("GET /api/v1/operations/{id}", s.operation)
	mux.HandleFunc("POST /api/v1/operations/{id}/cancel", s.cancelOperation)
	return mux
}

func (s *Server) Bootstrap() management.Bootstrap {
	value := s.application.Bootstrap()
	value.Origin, value.Url, value.Token = s.origin, s.origin, s.token
	return value
}

func (s *Server) URL() string           { return s.origin }
func (s *Server) Done() <-chan struct{} { return s.done }

// Close stops admission and body decoding before waiting for application work.
// Done closes only after both application cleanup and HTTP handlers have ended.
func (s *Server) Close() {
	s.closeOnce.Do(func() {
		s.mu.Lock()
		s.closed = true
		s.mu.Unlock()
		_ = s.http.Close()
		s.application.Close()
		s.requests.Wait()
		close(s.done)
	})
}

func (s *Server) guard(next http.Handler) http.Handler {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		select {
		case <-s.application.Stopping():
			s.mu.Unlock()
			writeError(w, 410, "SESSION_EXPIRED", "This management process is shutting down", false)
			return
		default:
		}
		if s.closed {
			s.mu.Unlock()
			writeError(w, 410, "SESSION_EXPIRED", "This management process is shutting down", false)
			return
		}
		s.requests.Add(1)
		s.mu.Unlock()
		defer s.requests.Done()
		next.ServeHTTP(w, r)
	})
	return loopbackapi.Guard(s.host, s.origin, s.token, writeError, handler)
}

func writeError(w http.ResponseWriter, status int, code, message string, retryable bool) {
	loopbackapi.WriteJSON(w, status, management.APIError{Code: code, Message: message, Retryable: retryable})
}

func writeBackendError(w http.ResponseWriter, err error) {
	var refusal *management.Refusal
	if errors.As(err, &refusal) {
		status := refusal.Status
		if status < 400 || status > 599 {
			status = http.StatusUnprocessableEntity
		}
		writeError(w, status, refusal.Code, refusal.Message, refusal.Retryable)
		return
	}
	if errors.Is(err, management.ErrPlanStale) {
		writeError(w, 409, "PLAN_STALE", "Installer facts changed; prepare a new review", false)
		return
	}
	writeError(w, 422, "OPERATION_REFUSED", "The requested operation could not be prepared; inspect the environment and source", false)
}
