package management

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestExplicitUpdateCheckRouteValidatesSelectionAndNormalizesArrays(t *testing.T) {
	server := newTestServer(t, "", "capture", nil)
	var out UpdateCheck
	requestJSON(t, server, "POST", "/api/v1/environment/check-updates", map[string]any{"all": true}, 200, &out)
	if out.ContextId != "capture" || out.Entries == nil || out.Diagnostics == nil {
		t.Fatal(out)
	}
	for _, body := range []any{map[string]any{}, map[string]any{"aliases": []string{}}, map[string]any{"aliases": []string{"a", "a"}}, map[string]any{"all": true, "aliases": []string{"a"}}, map[string]any{"all": true, "extra": true}} {
		requestJSON(t, server, "POST", "/api/v1/environment/check-updates", body, 400, nil)
	}
	request, err := http.NewRequestWithContext(t.Context(), http.MethodPost, server.URL()+"/api/v1/environment/check-updates", http.NoBody)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode < 400 {
		t.Fatal("update check bypassed private capability", response.StatusCode)
	}
}

type deadlineUpdateCheckBackend struct{ testBackend }

func (deadlineUpdateCheckBackend) CheckUpdates(ctx context.Context, _ UpdateCheckRequest) (UpdateCheck, error) {
	<-ctx.Done()
	return UpdateCheck{}, fmt.Errorf("source check: %w", ctx.Err())
}

func TestExplicitUpdateCheckDeadlineIsRetryable(t *testing.T) {
	ctx, cancel := context.WithDeadline(t.Context(), time.Now().Add(-time.Second))
	defer cancel()
	request := httptest.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/environment/check-updates", strings.NewReader(`{"all":true}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	server := Server{opts: Options{Backend: &deadlineUpdateCheckBackend{}}, ctx: t.Context()}
	server.checkUpdates(response, request)
	var out APIError
	if err := json.Unmarshal(response.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if response.Code != http.StatusGatewayTimeout || out.Code != "UPDATE_CHECK_TIMEOUT" || !out.Retryable || strings.Contains(out.Message, "source check") {
		t.Fatal(response.Code, out)
	}
}
