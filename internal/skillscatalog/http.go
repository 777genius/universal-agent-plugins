package skillscatalog

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

type errorEnvelope struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
	RetryAfter int `json:"retryAfter,omitempty"`
}
type catalogHandler struct {
	service *Service
	origins map[string]bool
}

// NewHandler permits only exact configured browser origins; CORS is not an
// authentication boundary. Upstream concurrency/rate controls also apply to
// clients that make anonymous requests without an Origin header.
func NewHandler(service *Service, origins []string) (http.Handler, error) {
	h := &catalogHandler{service: service, origins: map[string]bool{}}
	for _, origin := range origins {
		if !validOrigin(origin) {
			return nil, errors.New("invalid allowed origin")
		}
		h.origins[origin] = true
	}
	return h, nil
}
func validOrigin(origin string) bool {
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
		return false
	}
	local := u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1"
	return u.Scheme == "https" || (u.Scheme == "http" && local)
}
func (h *catalogHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "no-store")
	// Health remains available even during upstream failures or CORS rejection.
	if r.URL.Path == "/healthz" && r.Method == http.MethodGet {
		writeJSON(w, 200, map[string]string{"status": "ok"})
		return
	}
	if r.URL.Path != "/v1/skills" {
		writeError(w, 404, "not_found", "Endpoint not found", 0)
		return
	}
	if !h.allowOrigin(w, r) {
		return
	}
	if r.Method == http.MethodOptions {
		writePreflight(w, r)
		return
	}
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET, OPTIONS")
		writeError(w, 405, "method_not_allowed", "Only GET is allowed", 0)
		return
	}
	req, err := parseRequest(r.URL.RawQuery)
	if err != nil {
		writeError(w, 400, "invalid_request", err.Error(), 0)
		return
	}
	page, err := h.service.Get(r.Context(), req)
	if err != nil {
		writeUnavailable(w, err)
		return
	}
	writeJSON(w, 200, page)
}
func (h *catalogHandler) allowOrigin(w http.ResponseWriter, r *http.Request) bool {
	w.Header().Add("Vary", "Origin")
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	if !h.origins[origin] {
		writeError(w, 403, "origin_not_allowed", "Origin is not allowed", 0)
		return false
	}
	w.Header().Set("Access-Control-Allow-Origin", origin)
	return true
}
func writePreflight(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Origin") == "" || r.Header.Get("Access-Control-Request-Method") != "GET" || r.Header.Get("Access-Control-Request-Headers") != "" {
		writeError(w, 403, "invalid_preflight", "Only anonymous GET is allowed", 0)
		return
	}
	w.Header().Set("Access-Control-Allow-Methods", "GET")
	w.WriteHeader(http.StatusNoContent)
}
func writeUnavailable(w http.ResponseWriter, err error) {
	retry := 30 * time.Second
	var pe *ProviderError
	if errors.As(err, &pe) && pe.RetryAfter > 0 {
		retry = pe.RetryAfter
	}
	seconds := max(1, int((retry+time.Second-1)/time.Second))
	w.Header().Set("Retry-After", strconv.Itoa(seconds))
	writeError(w, 503, "provider_unavailable", "Skills provider is temporarily unavailable", seconds)
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func writeError(w http.ResponseWriter, status int, code, message string, retry int) {
	e := errorEnvelope{RetryAfter: retry}
	e.Error.Code = code
	e.Error.Message = message
	writeJSON(w, status, e)
}
