package loopbackapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGuardProtectsAllAPIRequests(t *testing.T) {
	for _, tc := range []struct {
		name, host, method, origin, token string
		status                            int
	}{
		{"valid mutation", "127.0.0.1:1234", "POST", "http://127.0.0.1:1234", "Bearer secret", 204},
		{"valid read", "127.0.0.1:1234", "GET", "", "Bearer secret", 204},
		{"wrong host", "attacker.example", "GET", "", "Bearer secret", 403},
		{"missing token", "127.0.0.1:1234", "GET", "", "", 401},
		{"cross origin read", "127.0.0.1:1234", "GET", "http://localhost:5678", "Bearer secret", 403},
		{"missing mutation origin", "127.0.0.1:1234", "POST", "", "Bearer secret", 403},
		{"null origin", "127.0.0.1:1234", "POST", "null", "Bearer secret", 403},
	} {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			handler := Guard("127.0.0.1:1234", "http://127.0.0.1:1234", "secret", testError, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				called = true
				w.WriteHeader(http.StatusNoContent)
			}))
			req := httptest.NewRequest(tc.method, "/api/v1/session", nil)
			req.Host = tc.host
			req.Header.Set("Origin", tc.origin)
			req.Header.Set("Authorization", tc.token)
			out := httptest.NewRecorder()
			handler.ServeHTTP(out, req)
			if out.Code != tc.status || called != (tc.status == 204) {
				t.Fatalf("status=%d called=%v, want %d", out.Code, called, tc.status)
			}
			if out.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("private response can be cached")
			}
		})
	}
}

func TestDecodeRejectsAmbiguousOrOversizedInput(t *testing.T) {
	for _, body := range []string{
		`{"known":"yes","unknown":true}`,
		`{"known":"one"} {"known":"two"}`,
		`{"known":"` + strings.Repeat("x", 1<<20) + `"}`,
	} {
		req := httptest.NewRequest("POST", "/api/v1/plans", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		out := httptest.NewRecorder()
		var value struct {
			Known string `json:"known"`
		}
		if Decode(out, req, &value, testError) || out.Code != 400 {
			t.Fatalf("ambiguous input accepted: %d", out.Code)
		}
	}
}

func testError(w http.ResponseWriter, status int, _, _ string, _ bool) {
	w.WriteHeader(status)
}
