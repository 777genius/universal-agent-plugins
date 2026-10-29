package agentpluginscli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/management"
)

func TestManagementPreparationRefusalReturnsJSONWithoutTypedNilPanic(t *testing.T) {
	repo, _ := savedJourneyRepository(t)
	backend := managementJourneyBackend(t, newSavedJourney(t, repo))
	request := managementInstallRequest(t.TempDir())
	retained, err := backend.Prepare(t.Context(), request, "refused-local-source")
	if err == nil || retained != nil {
		t.Fatalf("error returned executable-looking typed nil: %T, %v", retained, err)
	}
	server, err := management.Start(t.Context(), management.Options{Backend: backend, ContextID: backend.ContextID(), EnvironmentID: backend.EnvironmentID(), IntentRoot: backend.config.DependenciesRoot, StateRoot: backend.config.StateRoot, EngineVersion: backend.app.Version})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(server.Close)
	bootstrap := server.Bootstrap()
	body, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, bootstrap.Url+"/api/v1/plans", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+bootstrap.Token)
	req.Header.Set("Origin", bootstrap.Origin)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Transport: &http.Transport{Proxy: nil}, Timeout: 10 * time.Second}
	defer client.CloseIdleConnections()
	response, err := client.Do(req)
	if err != nil {
		t.Fatal("preparation refusal broke the HTTP connection", err)
	}
	defer func() { _ = response.Body.Close() }()
	var failure management.Error
	if err := json.NewDecoder(response.Body).Decode(&failure); err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusUnprocessableEntity || failure.Code != "SOURCE_NOT_SUPPORTED" {
		t.Fatal(response.StatusCode, failure)
	}
}
