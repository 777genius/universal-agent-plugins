//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"reflect"
	"testing"
)

func TestClineCycleHeldEndpointPreservesPriorActualHTTPPhases(t *testing.T) {
	c := clineCycleInert(t)
	c.startProvider(t)
	defer c.owner.close(t, c)
	transport := &http.Transport{DisableKeepAlives: true}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport}
	var previous *clineProjectProvider
	var captured []byte
	for _, scope := range []string{"a", "b"} {
		stage := "empty-" + scope
		p, err := c.owner.arm(stage, c.recipes[scope])
		if err != nil {
			t.Fatal(err)
		}
		body := clineContractRequest(p)
		body["tools"].([]any)[0].(map[string]any)["function"].(map[string]any)["name"] = "read_file"
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		r, err := http.NewRequestWithContext(c.f.ctx, http.MethodPost, "http://"+c.owner.listener.Addr().String()+"/v1/chat/completions", bytes.NewReader(raw))
		if err != nil {
			t.Fatal(err)
		}
		r.Header.Set("Authorization", "Bearer "+c.f.key)
		r.Header.Set("Content-Type", "application/json")
		response, err := client.Do(r)
		if err != nil {
			t.Fatal(err)
		}
		result, readErr := io.ReadAll(io.LimitReader(response.Body, (1<<20)+1))
		closeErr := response.Body.Close()
		if readErr != nil || closeErr != nil || response.StatusCode != 200 || len(result) > 1<<20 || !bytes.Contains(result, []byte(p.doneText())) || bytes.Contains(result, []byte("tool_calls")) {
			t.Fatal("empty phase actual bounded HTTP reply differs")
		}
		c.owner.complete(t, c, stage)
		if previous != nil {
			if previous.callID == p.callID || previous.prompt == p.prompt || previous.doneText() == p.doneText() || len(previous.requests) != 1 || !bytes.HasPrefix(c.aggregates["cycle-provider-requests.jsonl"], captured) {
				t.Fatal("next phase reused identity or discarded previous evidence")
			}
		}
		previous = p
		captured = bytes.Clone(c.aggregates["cycle-provider-requests.jsonl"])
		c.f.check(t)
	}
	if !reflect.DeepEqual(c.owner.completed, []string{"empty-a", "empty-b"}) {
		t.Fatal("actual phase history differs")
	}
}

func TestClineCycleDataDeltaKeepsUnselectedCallerAndMarkerBytes(t *testing.T) {
	c := clineCycleInert(t)
	clineCycleStageActual(t, c, "a")
	r := c.recipes["a"]
	q := &clineProjectProvider{operation: "write", marker: "retained-inert"}
	before := c.nativeDataBefore(t, r, q)
	projectNativeRuntimeWrite(t, r.Data+"/native-marker.txt", []byte(q.marker), 0600)
	c.nativeDataAfter(t, r, q, before)
	c.f.check(t)
	q.operation = "read"
	before = c.nativeDataBefore(t, r, q)
	c.nativeDataAfter(t, r, q, before)
	c.f.check(t)
}
