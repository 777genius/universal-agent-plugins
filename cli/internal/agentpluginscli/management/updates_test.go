package management

import (
	"net/http"
	"testing"
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
