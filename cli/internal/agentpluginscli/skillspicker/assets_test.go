package skillspicker

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// A bundled SPA must serve its index internally: passing /index.html through
// FileServer would redirect to ./ and create a loop in the outer SPA handler.
func TestBundledRootAndSPAServeIndexWithoutRedirect(t *testing.T) {
	if !Bundled() {
		t.Skip("production assets are checked after bundle:go")
	}
	for _, route := range []string{"/", "/browse/engineering"} {
		response := httptest.NewRecorder()
		serveAssets(response, httptest.NewRequestWithContext(context.Background(), http.MethodGet, route, http.NoBody))
		if response.Code != http.StatusOK || response.Header().Get("Location") != "" || !strings.Contains(response.Body.String(), "<html") {
			t.Fatalf("bundled %s returned %d location=%q", route, response.Code, response.Header().Get("Location"))
		}
	}
}
