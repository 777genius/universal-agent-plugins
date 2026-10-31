package skillscatalog

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/777genius/plugin-kit-ai/internal/skillsresolve"
)

type SourceResolver interface {
	Resolve(context.Context, skillsresolve.Request) (skillsresolve.Result, error)
}

type resolveHandler struct {
	resolver SourceResolver
	token    [32]byte
	enabled  bool
	mu       sync.Mutex
	active   bool
	tokens   float64
	tokenAt  time.Time
}

// NewResolveHandler is a service-only capability. A missing resolver or token
// disables it without affecting anonymous catalog browsing or health.
func NewResolveHandler(resolver SourceResolver, token string) http.Handler {
	return &resolveHandler{resolver: resolver, token: sha256.Sum256([]byte(token)),
		enabled: resolver != nil && len(token) >= 32 && safeText(token, 512) && token == strings.TrimSpace(token),
		tokens:  2, tokenAt: time.Now()}
}

func (h *resolveHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "no-store")
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	if !h.authorize(w, r) {
		return
	}
	request, err := readResolveRequest(w, r)
	if err != nil {
		writeError(w, 400, "invalid_request", "Invalid source request", 0)
		return
	}
	if !h.admit() {
		resolveUnavailable(w, "resolver_busy", 5)
		return
	}
	defer h.release()
	// The core runner may need five seconds to terminate its process tree. That
	// grace fits inside the overall deadline and the server's 35-second write.
	deadline, _ := ctx.Deadline()
	work, stop := context.WithDeadline(ctx, deadline.Add(-5*time.Second))
	defer stop()
	result, err := h.resolver.Resolve(work, request)
	if errors.Is(err, skillsresolve.ErrInvalidRequest) {
		writeError(w, 400, "invalid_request", "Invalid source request", 0)
		return
	}
	if err != nil || ctx.Err() != nil {
		resolveUnavailable(w, "source_unavailable", 30)
		return
	}
	writeResolveResult(w, result)
}

func (h *resolveHandler) authorize(w http.ResponseWriter, r *http.Request) bool {
	if !h.enabled {
		resolveUnavailable(w, "resolver_unavailable", 30)
		return false
	}
	if r.Header.Get("Origin") != "" {
		writeError(w, 403, "service_only", "Source resolution is available to authorized services", 0)
		return false
	}
	authorization := r.Header.Values("Authorization")
	provided := sha256.Sum256([]byte(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")))
	if len(authorization) != 1 || !strings.HasPrefix(authorization[0], "Bearer ") || subtle.ConstantTimeCompare(provided[:], h.token[:]) != 1 {
		w.Header().Set("WWW-Authenticate", "Bearer")
		writeError(w, 401, "unauthorized", "Service authorization required", 0)
		return false
	}
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		writeError(w, 405, "method_not_allowed", "Only POST is allowed", 0)
		return false
	}
	return true
}

func readResolveRequest(w http.ResponseWriter, r *http.Request) (skillsresolve.Request, error) {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" || r.URL.RawQuery != "" {
		return skillsresolve.Request{}, skillsresolve.ErrInvalidRequest
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 8<<10))
	if err != nil || !utf8.Valid(body) {
		return skillsresolve.Request{}, skillsresolve.ErrInvalidRequest
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	first, err := decoder.Token()
	if err != nil || first != json.Delim('{') {
		return skillsresolve.Request{}, skillsresolve.ErrInvalidRequest
	}
	request := skillsresolve.Request{}
	seen := map[string]bool{}
	for decoder.More() {
		if err = decodeResolveField(decoder, &request, seen); err != nil {
			return request, err
		}
	}
	last, err := decoder.Token()
	if err != nil || last != json.Delim('}') {
		return request, skillsresolve.ErrInvalidRequest
	}
	if _, err = decoder.Token(); !errors.Is(err, io.EOF) {
		return request, skillsresolve.ErrInvalidRequest
	}
	return request, skillsresolve.ValidateRequest(request)
}

func decodeResolveField(decoder *json.Decoder, request *skillsresolve.Request, seen map[string]bool) error {
	key, err := decoder.Token()
	name, ok := key.(string)
	if err != nil || !ok || seen[name] {
		return skillsresolve.ErrInvalidRequest
	}
	seen[name] = true
	var value *string
	if err = decoder.Decode(&value); err != nil || value == nil {
		return skillsresolve.ErrInvalidRequest
	}
	switch name {
	case "repository":
		request.Repository = *value
	case "skillNameHint":
		request.SkillNameHint = value
	case "commit":
		request.Commit = value
	case "path":
		request.Path = value
	default:
		return skillsresolve.ErrInvalidRequest
	}
	return nil
}

func (h *resolveHandler) admit() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	now := time.Now()
	h.tokens = min(2, h.tokens+now.Sub(h.tokenAt).Seconds()/5)
	h.tokenAt = now
	if h.active || h.tokens < 1 {
		return false
	}
	h.active, h.tokens = true, h.tokens-1
	return true
}

func (h *resolveHandler) release() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.active = false
}

func resolveUnavailable(w http.ResponseWriter, code string, retry int) {
	w.Header().Set("Retry-After", "30")
	if retry == 5 {
		w.Header().Set("Retry-After", "5")
	}
	writeError(w, 503, code, "Skill preview is temporarily unavailable", retry)
}

func writeResolveResult(w http.ResponseWriter, result skillsresolve.Result) {
	body, err := json.Marshal(result)
	if err != nil || len(body) > skillsresolve.MaxResponseBytes {
		result.Status, result.Skill, result.Candidates = "incomplete", nil, nil
		result.Reasons = []string{"response_bytes_limit"}
		writeJSON(w, http.StatusOK, result)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}
