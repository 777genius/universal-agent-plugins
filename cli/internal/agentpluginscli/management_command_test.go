package agentpluginscli

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/management"
)

type failedManagementBootstrap struct {
	bytes.Buffer
	err error
}

func (w *failedManagementBootstrap) Write(body []byte) (int, error) {
	_, _ = w.Buffer.Write(body)
	return 0, w.err
}

func TestManagementBootstrapWriteFailureClosesStartedHost(t *testing.T) {
	backend := managementJourneyBackend(t, newSavedJourney(t, t.TempDir()))
	app := backend.app
	app.ManagementFactory = func(management.Startup) (App, error) { return backend.app, nil }
	startup, err := json.Marshal(backend.config)
	if err != nil {
		t.Fatal(err)
	}
	failed := &failedManagementBootstrap{err: errors.New("bootstrap output unavailable")}
	err = runManagementServer(t.Context(), app, bytes.NewReader(append(startup, '\n')), failed)
	if !errors.Is(err, failed.err) {
		t.Fatal("startup did not return the output failure", err)
	}
	var bootstrap management.Bootstrap
	if err := json.Unmarshal(failed.Bytes(), &bootstrap); err != nil {
		t.Fatal("test did not reach a real listener bootstrap", err)
	}
	request, err := http.NewRequestWithContext(t.Context(), "GET", bootstrap.Url+"/api/v1/session", http.NoBody)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer "+bootstrap.Token)
	client := &http.Client{Transport: &http.Transport{Proxy: nil}, Timeout: time.Second}
	defer client.CloseIdleConnections()
	response, err := client.Do(request)
	if response != nil {
		_ = response.Body.Close()
	}
	if err == nil {
		t.Fatal("bootstrap output failure left the management listener running")
	}
}
