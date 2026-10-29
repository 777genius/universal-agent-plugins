package skillscatalog

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/internal/skillsresolve"
)

const resolverToken = "fixture-capability-token-at-least-32-bytes"

type resolverFunc func(context.Context, skillsresolve.Request) (skillsresolve.Result, error)

func (f resolverFunc) Resolve(ctx context.Context, r skillsresolve.Request) (skillsresolve.Result, error) {
	return f(ctx, r)
}

func resolveRequest(body string) *http.Request {
	r := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/skills/resolve", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Authorization", "Bearer "+resolverToken)
	return r
}

func TestResolveHTTPRejectsBeforeSourceWork(t *testing.T) {
	var calls atomic.Int32
	f := resolverFunc(func(context.Context, skillsresolve.Request) (skillsresolve.Result, error) {
		calls.Add(1)
		return skillsresolve.Result{Version: 1, Status: "not_found", Repository: "owner/repo"}, nil
	})
	h := NewResolveHandler(f, resolverToken)
	for _, body := range []string{
		`{}`, `null`, `[]`, `{"repository":"owner/repo","unknown":"x"}`,
		`{"repository":"owner/repo","repository":"other/repo"}`,
		`{"repository":"owner/repo","path":""}`, `{"repository":"owner/repo","commit":null}`,
		`{"repository":"owner/repo","skillNameHint":42}`, `{"repository":"owner/repo"} {}`,
		`{"repository":"owner/repo","skillNameHint":"` + strings.Repeat("x", 8192) + `"}`,
		"{\"repository\":\"owner/repo\",\"skillNameHint\":\"bad\xff\"}",
	} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, resolveRequest(body))
		if w.Code != 400 {
			t.Fatalf("invalid request status %d: %q", w.Code, body[:min(len(body), 100)])
		}
	}
	for _, tc := range []struct {
		header, value string
		status        int
	}{
		{"Authorization", "", 401}, {"Authorization", "Bearer wrong", 401},
		{"Origin", "https://reviewrouter.org", 403}, {"Content-Type", "text/plain", 400},
	} {
		r := resolveRequest(`{"repository":"owner/repo"}`)
		r.Header.Set(tc.header, tc.value)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != tc.status || w.Header().Get("Access-Control-Allow-Origin") != "" {
			t.Fatal("unauthorized or browser request accepted", tc, w.Code)
		}
	}
	if calls.Load() != 0 {
		t.Fatal("source called for invalid request")
	}
	w := httptest.NewRecorder()
	NewResolveHandler(nil, "").ServeHTTP(w, resolveRequest(`{"repository":"owner/repo"}`))
	if w.Code != 503 || w.Header().Get("Retry-After") == "" {
		t.Fatal("disabled resolver did not fail unavailable")
	}
}

func TestResolveHTTPBoundsParallelDemandAndResponse(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	h := NewResolveHandler(resolverFunc(func(ctx context.Context, _ skillsresolve.Request) (skillsresolve.Result, error) {
		deadline, ok := ctx.Deadline()
		if !ok || time.Until(deadline) > 25*time.Second {
			t.Error("missing overall work deadline")
		}
		close(entered)
		<-release
		return skillsresolve.Result{Version: 1, Status: "resolved", Repository: "owner/repo", Skill: &skillsresolve.Skill{RawContent: strings.Repeat("<", 64<<10)}}, nil
	}), resolverToken)
	first, done := httptest.NewRecorder(), make(chan struct{})
	go func() { h.ServeHTTP(first, resolveRequest(`{"repository":"owner/repo"}`)); close(done) }()
	<-entered
	second := httptest.NewRecorder()
	h.ServeHTTP(second, resolveRequest(`{"repository":"owner/repo"}`))
	if second.Code != 503 || second.Header().Get("Retry-After") != "5" {
		t.Fatal("parallel resolver was not bounded", second.Code)
	}
	close(release)
	<-done
	var result skillsresolve.Result
	if json.Unmarshal(first.Body.Bytes(), &result) != nil || result.Status != "incomplete" || result.Skill != nil || len(first.Body.Bytes()) > skillsresolve.MaxResponseBytes {
		t.Fatal("oversize result became a truncated or eligible success", first.Body.String())
	}
}
