package skillscatalog

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

type errorEnvelope struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
	RetryAfter int `json:"retryAfter,omitempty"`
}

// NewHandler permits only exact configured browser origins; CORS is not an
// authentication boundary. Upstream concurrency/rate controls also apply to
// clients that make anonymous requests without an Origin header.
func NewHandler(service *Service, origins []string) (http.Handler, error) {
	allowed := map[string]bool{}
	for _, origin := range origins {
		u, err := url.Parse(origin)
		if err != nil || (u.Scheme != "https" && !(u.Scheme == "http" && (u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1"))) || u.Host == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
			return nil, errors.New("invalid allowed origin")
		}
		allowed[origin] = true
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
		w.Header().Add("Vary", "Origin")
		origin := r.Header.Get("Origin")
		if origin != "" {
			if !allowed[origin] {
				writeError(w, 403, "origin_not_allowed", "Origin is not allowed", 0)
				return
			}
			w.Header().Set("Access-Control-Allow-Origin", origin)
		}
		if r.Method == http.MethodOptions {
			if origin == "" || r.Header.Get("Access-Control-Request-Method") != "GET" || r.Header.Get("Access-Control-Request-Headers") != "" {
				writeError(w, 403, "invalid_preflight", "Only anonymous GET is allowed", 0)
				return
			}
			w.Header().Set("Access-Control-Allow-Methods", "GET")
			w.WriteHeader(http.StatusNoContent)
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
		page, err := service.Get(r.Context(), req)
		if err != nil {
			retry := time.Second * 30
			var pe *ProviderError
			if errors.As(err, &pe) && pe.RetryAfter > 0 {
				retry = pe.RetryAfter
			}
			seconds := max(1, int((retry+time.Second-1)/time.Second))
			w.Header().Set("Retry-After", strconv.Itoa(seconds))
			writeError(w, 503, "provider_unavailable", "Skills provider is temporarily unavailable", seconds)
			return
		}
		writeJSON(w, 200, page)
	}), nil
}
func parseRequest(raw string) (Request, error) {
	bad := func(message string) (Request, error) { return Request{}, errors.New(message) }
	if len(raw) > 2048 {
		return bad("Query string is too long")
	}
	q, err := url.ParseQuery(raw)
	if err != nil {
		return bad("Invalid query string")
	}
	for k, v := range q {
		if len(v) != 1 {
			return bad("Duplicate query parameters are not allowed")
		}
		if k != "view" && k != "page" && k != "q" && k != "limit" {
			return bad("Unknown query parameter")
		}
	}
	r := Request{View: Popular}
	if q.Has("view") {
		r.View = View(q.Get("view"))
		if !r.View.Valid() {
			return bad("view must be popular, trending or hot")
		}
	}
	if q.Has("page") {
		r.Page, err = strconv.Atoi(q.Get("page"))
		if err != nil || r.Page < 0 || r.Page > 10000 {
			return bad("page must be between 0 and 10000")
		}
	}
	if q.Has("q") {
		if !safeText(q.Get("q"), 200) {
			return bad("q contains unsupported characters")
		}
		r.Query = strings.TrimSpace(q.Get("q"))
		if !utf8.ValidString(r.Query) || utf8.RuneCountInString(r.Query) < 2 || !safeText(r.Query, 200) {
			return bad("q must contain 2 or more characters, up to 200 bytes")
		}
		if r.Page != 0 {
			return bad("Search supports only page 0")
		}
		r.View = Popular
		r.Limit = 100
		if q.Has("limit") {
			r.Limit, err = strconv.Atoi(q.Get("limit"))
			if err != nil || r.Limit < 1 || r.Limit > 100 {
				return bad("limit must be between 1 and 100")
			}
		}
	} else if q.Has("limit") {
		return bad("limit is only supported with q")
	}
	return r, nil
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
