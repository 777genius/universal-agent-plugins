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
	response, err := http.Post(server.URL()+"/api/v1/environment/check-updates", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode < 400 {
		t.Fatal("update check bypassed private capability", response.StatusCode)
	}
}
