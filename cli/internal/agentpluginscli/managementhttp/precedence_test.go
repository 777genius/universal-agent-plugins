package managementhttp

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/management"
)

// Embedding only the required interface deliberately hides optional capabilities.
type requiredBackend struct{ management.Backend }

func TestUnsupportedCapabilitiesPrecedeRequestDecodingAndSelection(t *testing.T) {
	server := newTestServer(t, "", "capture", requiredBackend{Backend: &testBackend{}})
	for _, tc := range []struct{ method, path string }{
		{"POST", "/api/v1/skills/local/plans"},
		{"DELETE", "/api/v1/skills/catalog/" + strings.Repeat("x", 257)},
		{"GET", "/api/v1/skills/catalog/" + strings.Repeat("x", 257) + "/candidates/invalid/preview"},
	} {
		t.Run(tc.method+tc.path, func(t *testing.T) {
			request, err := http.NewRequestWithContext(t.Context(), tc.method, server.URL()+tc.path, strings.NewReader("not JSON"))
			if err != nil {
				t.Fatal(err)
			}
			bootstrap := server.Bootstrap()
			request.Header.Set("Authorization", "Bearer "+bootstrap.Token)
			request.Header.Set("Origin", bootstrap.Origin)
			request.Header.Set("Content-Type", "text/plain")
			response, err := http.DefaultClient.Do(request)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = response.Body.Close() }()
			var failure management.APIError
			if err := json.NewDecoder(response.Body).Decode(&failure); err != nil {
				t.Fatal(err)
			}
			if response.StatusCode != 422 || failure.Code != "CAPABILITY_UNAVAILABLE" {
				t.Fatal("request shape masked the unavailable capability", response.StatusCode, failure)
			}
		})
	}
}
