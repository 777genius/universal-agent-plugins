//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCopilotCycleActualInertLoopbackProviderClosesOrdinarily(t *testing.T) {
	c := copilotCycleInert(t)
	_, old, _ := copilotReadinessActual(t)
	value, err := projectNativeRuntimeJSON(old.requests[0])
	if err != nil {
		t.Fatal(err)
	}
	row := value.(map[string]any)
	delete(row, "tools")
	raw, _ := json.Marshal(row)
	p := newCopilotCycleProvider(old.nonce, "empty-a", "", "", nil)
	if err = p.start(c.f.ctx); err != nil {
		t.Fatal(err)
	}
	defer p.shutdown(t, c, "empty-a")
	request, err := http.NewRequestWithContext(c.f.ctx, "POST", p.url+"/v1/chat/completions", bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 3 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if closeErr := response.Body.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}
	if err != nil || response.StatusCode != 200 || !bytes.Contains(body, []byte(p.p.sentinel)) {
		t.Fatal("inert loopback response", err)
	}
	p.shutdown(t, c, "empty-a")
	if p.server == nil || !p.closed || !p.connectionsQuiet(c.f.ctx) || len(c.aggregates["cycle-provider-receipts.jsonl"]) == 0 {
		t.Fatal("ordinary provider closure not captured")
	}
	receipt, err := projectNativeRuntimeJSON(bytes.TrimSuffix(c.aggregates["cycle-provider-receipts.jsonl"], []byte{'\n'}))
	if err != nil {
		t.Fatal(err)
	}
	fields := receipt.(map[string]any)["value"].(map[string]any)
	if fields["shutdown"] != true || fields["serve_closed"] != true || fields["connections_closed"] != true || fields["forced"] != false {
		t.Fatal("ordinary closure receipt differs")
	}
}

func TestCopilotCycleProviderRejectsCredentialsAndForeignRoutesBeforeCapture(t *testing.T) {
	_, old, _ := copilotReadinessActual(t)
	for _, kind := range []string{"authorization", "api-key", "path", "host", "method", "private"} {
		p := newCopilotCycleProvider(old.nonce, "inert", old.tool, "write", nil)
		p.url = "http://127.0.0.1:39999"
		raw := old.requests[0]
		if kind == "private" {
			raw = []byte(`{"canary":"uap-copilot-\u0070rivate-test"}`)
		}
		request := copilotCycleInertRequest(t, p.url, raw)
		request.Header.Set("Content-Type", "application/json")
		switch kind {
		case "authorization":
			request.Header.Set("Authorization", "inert canary")
		case "api-key":
			request.Header.Set("api-key", "inert canary")
		case "path":
			request.URL.Path = "/foreign"
		case "host":
			request.Host = "foreign.invalid"
		case "method":
			request.Method = "GET"
		}
		recorder := httptest.NewRecorder()
		p.serve(recorder, request)
		if recorder.Code != 400 || p.p.err == nil || len(p.p.requests) != 0 || len(p.responses) != 0 {
			t.Fatal("rejected credential/route/private body was captured", kind)
		}
	}
}
