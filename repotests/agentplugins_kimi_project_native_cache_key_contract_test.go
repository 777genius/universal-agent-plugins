//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const kimiCacheContractKey = "session_11111111-2222-4333-8444-555555555555"

func kimiCacheContractProvider() (*kimiAttemptProvider, *int) {
	validated := new(int)
	p := &kimiAttemptProvider{server: &httptest.Server{URL: "http://127.0.0.1:43219"},
		key: "inert-provider-key", id: "owned-call", nonce: "owned-nonce", marker: "owned-marker", sentinel: "inert-terminal"}
	p.validate = func(body string) error {
		if body != "owned facts" {
			return fmt.Errorf("wrong facts")
		}
		*validated++
		return nil
	}
	return p, validated
}

func kimiCacheContractBody(t *testing.T, value map[string]any) []byte {
	t.Helper()
	body, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func kimiCacheContractSend(p *kimiAttemptProvider, body []byte, mutate func(*http.Request)) *httptest.ResponseRecorder {
	r := httptest.NewRequestWithContext(context.Background(), http.MethodPost, p.server.URL+"/v1/chat/completions", bytes.NewReader(body))
	r.Header.Set("Authorization", "Bearer "+p.key)
	r.Header.Set("Content-Type", "application/json")
	if mutate != nil {
		mutate(r)
	}
	w := httptest.NewRecorder()
	p.serve(w, r)
	return w
}

func kimiCacheContractSecond(t *testing.T, p *kimiAttemptProvider, key string) map[string]any {
	t.Helper()
	value := kimiAttemptContractBody()
	if key != "" {
		value["prompt_cache_key"] = key
	}
	args := kimiCacheContractBody(t, map[string]any{"nonce": p.nonce, "operation": "write", "marker": p.marker})
	value["messages"] = append(value["messages"].([]any),
		map[string]any{"role": "assistant", "content": nil, "tool_calls": []any{map[string]any{
			"id": p.id, "type": "function", "function": map[string]any{"name": kimiAttemptTool, "arguments": string(args)}}}},
		map[string]any{"role": "tool", "tool_call_id": p.id, "content": "owned facts"})
	return value
}

func TestKimiSingleToolCanonicalCacheMetadataPreservesOwnedExchange(t *testing.T) {
	for _, key := range []string{"", kimiCacheContractKey} {
		p, validated := kimiCacheContractProvider()
		first := kimiAttemptContractBody()
		if key != "" {
			first["prompt_cache_key"] = key
		}
		body := kimiCacheContractBody(t, first)
		if _, err := kimiAttemptRequest(body); err != nil {
			t.Fatalf("canonical session metadata refused: %v", err)
		}
		w := kimiCacheContractSend(p, body, nil)
		if w.Code != 200 || !p.issued || p.returned || p.err != nil || p.requestCount != 1 || len(p.requests) != 1 || *validated != 0 || !bytes.Equal(p.requests[0], body) || !strings.Contains(w.Body.String(), kimiAttemptTool) || strings.Contains(w.Body.String(), p.sentinel) {
			t.Fatal("canonical metadata did not preserve the exact first owned response")
		}
		w = kimiCacheContractSend(p, kimiCacheContractBody(t, kimiCacheContractSecond(t, p, key)), nil)
		if w.Code != 200 || !p.returned || p.err != nil || *validated != 1 || p.requestCount != 2 || len(p.requests) != 2 || !strings.Contains(w.Body.String(), p.sentinel) {
			t.Fatal("same-session metadata did not preserve the exact owned result")
		}
	}
}

func TestKimiSingleToolCacheMetadataRefusesNoncanonicalValues(t *testing.T) {
	values := []any{nil, false, 1, []any{}, map[string]any{}, "", "session_owned", "session_" + strings.Repeat("a", 36),
		strings.Replace(kimiCacheContractKey, "session_", "Session_", 1), strings.Replace(kimiCacheContractKey, "1111", "GGGG", 1),
		strings.Replace(kimiCacheContractKey, "1111", "ABCD", 1), kimiCacheContractKey + "\n", " " + kimiCacheContractKey,
		kimiCacheContractKey[:43] + "\n", kimiCacheContractKey[:43] + "\x00", kimiCacheContractKey[:42] + "é",
		kimiCacheContractKey[:43], kimiCacheContractKey + "a", "session_" + strings.Repeat("a", 4096)}
	for index, value := range values {
		t.Run(fmt.Sprintf("shape-%d", index), func(t *testing.T) {
			request := kimiAttemptContractBody()
			request["prompt_cache_key"] = value
			assertKimiCacheContractRefusal(t, kimiCacheContractBody(t, request), nil)
		})
	}
	request := kimiAttemptContractBody()
	request["prompt_cache_key"] = kimiCacheContractKey
	body := kimiCacheContractBody(t, request)
	body = append([]byte(`{"prompt_cache_key":"`+kimiCacheContractKey+`",`), body[1:]...)
	assertKimiCacheContractRefusal(t, body, nil)
}

func assertKimiCacheContractRefusal(t *testing.T, body []byte, mutate func(*http.Request)) {
	t.Helper()
	p, validated := kimiCacheContractProvider()
	w := kimiCacheContractSend(p, body, mutate)
	if w.Code != 400 || p.issued || p.returned || p.err == nil || *validated != 0 || p.requestCount != 1 || strings.Contains(w.Body.String(), kimiCacheContractKey) {
		t.Fatal("invalid metadata or authority reached an owned response or leaked metadata")
	}
}

func TestKimiSingleToolCacheMetadataCannotBypassAuthority(t *testing.T) {
	for _, kind := range []string{"wrong-key", "wrong-route", "foreign-tool", "unknown-field"} {
		t.Run(kind, func(t *testing.T) {
			request := kimiAttemptContractBody()
			request["prompt_cache_key"] = kimiCacheContractKey
			switch kind {
			case "foreign-tool":
				request["tools"].([]any)[0].(map[string]any)["function"].(map[string]any)["name"] = "foreign-tool"
			case "unknown-field":
				request["unknown_cache_authority"] = true
			}
			assertKimiCacheContractRefusal(t, kimiCacheContractBody(t, request), func(r *http.Request) {
				switch kind {
				case "wrong-key":
					r.Header.Set("Authorization", "Bearer foreign-key")
				case "wrong-route":
					r.URL.Path = "/"
				}
			})
		})
	}
}

func TestKimiSingleToolCacheMetadataCannotChangeBetweenTurns(t *testing.T) {
	other := "session_aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee"
	for _, pair := range [][2]string{{"", kimiCacheContractKey}, {kimiCacheContractKey, ""}, {kimiCacheContractKey, other}} {
		p, validated := kimiCacheContractProvider()
		first := kimiAttemptContractBody()
		if pair[0] != "" {
			first["prompt_cache_key"] = pair[0]
		}
		if w := kimiCacheContractSend(p, kimiCacheContractBody(t, first), nil); w.Code != 200 || !p.issued {
			t.Fatal("canonical first request refused")
		}
		w := kimiCacheContractSend(p, kimiCacheContractBody(t, kimiCacheContractSecond(t, p, pair[1])), nil)
		if w.Code != 400 || p.returned || p.err == nil || *validated != 0 || p.requestCount != 2 || len(p.requests) != 2 {
			t.Fatal("changed cache metadata crossed the owned result boundary")
		}
		firstError := p.err
		w = kimiCacheContractSend(p, kimiCacheContractBody(t, kimiCacheContractSecond(t, p, pair[0])), nil)
		if w.Code != 400 || !errors.Is(p.err, firstError) || p.returned || *validated != 0 || p.requestCount != 3 || len(p.requests) != 2 {
			t.Fatal("later valid metadata cleared the provider refusal")
		}
	}
}
