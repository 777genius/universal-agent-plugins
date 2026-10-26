//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"testing"
)

func kimiAttemptContractBody() map[string]any {
	return map[string]any{"model": kimiAttemptModel, "stream": true, "stream_options": map[string]any{"include_usage": true},
		"messages": []any{map[string]any{"role": "user", "content": "inert owned single tool request"}},
		"tools": []any{map[string]any{"type": "function", "function": map[string]any{"name": kimiAttemptTool, "description": "Inspect only disposable fixture runtime facts.",
			"parameters": map[string]any{"type": "object", "properties": map[string]any{"nonce": map[string]any{"type": "string"}, "operation": map[string]any{"type": "string"}, "marker": map[string]any{"type": "string"}}, "required": []any{"nonce"}, "additionalProperties": false}}}}}
}

func TestKimiSingleToolRequestRefusesForeignToolsAndUnknownGrammar(t *testing.T) {
	for _, kind := range []string{"valid", "foreign-tool", "extra-tool", "unknown-schema", "unknown-option", "nonfinite", "duplicate"} {
		value := kimiAttemptContractBody()
		function := value["tools"].([]any)[0].(map[string]any)["function"].(map[string]any)
		switch kind {
		case "foreign-tool":
			function["name"] = "Bash"
		case "extra-tool":
			value["tools"] = append(value["tools"].([]any), value["tools"].([]any)[0])
		case "unknown-schema":
			function["parameters"].(map[string]any)["extra"] = true
		case "unknown-option":
			value["unreviewed_authority"] = true
		}
		body, _ := json.Marshal(value)
		if kind == "nonfinite" {
			body = bytes.Replace(body, []byte(`"stream":true`), []byte(`"temperature":1e999,"stream":true`), 1)
		}
		if kind == "duplicate" {
			body = append([]byte(`{"model":"foreign",`), body[1:]...)
		}
		_, err := kimiAttemptRequest(body)
		if (err == nil) != (kind == "valid") {
			t.Fatalf("%s request: %v", kind, err)
		}
	}
}

func TestKimiSingleToolProviderOwnsOnlyOneTwoTurnExchange(t *testing.T) {
	for _, kind := range []string{"valid", "foreign-call-id", "wrong-arguments", "wrong-key", "wrong-host", "extra-turn"} {
		p := &kimiAttemptProvider{server: &httptest.Server{URL: "http://127.0.0.1:43219"}, key: "inert-provider-key", id: "owned-call", nonce: "owned-nonce", marker: "owned-marker", sentinel: "done"}
		validated := 0
		p.validate = func(body string) error {
			if body != "owned facts" {
				return fmt.Errorf("wrong facts")
			}
			validated++
			return nil
		}
		send := func(value map[string]any, wrongKey, wrongHost bool) *httptest.ResponseRecorder {
			body, _ := json.Marshal(value)
			r := httptest.NewRequest("POST", p.server.URL+"/v1/chat/completions", bytes.NewReader(body))
			r.Header.Set("Authorization", "Bearer "+p.key)
			r.Header.Set("Content-Type", "application/json")
			if wrongKey {
				r.Header.Set("Authorization", "Bearer foreign-key")
			}
			if wrongHost {
				r.Host = "foreign.invalid"
			}
			w := httptest.NewRecorder()
			p.serve(w, r)
			return w
		}
		first := send(kimiAttemptContractBody(), kind == "wrong-key", kind == "wrong-host")
		if kind == "wrong-key" || kind == "wrong-host" {
			if p.issued || p.returned || first.Code != 400 {
				t.Fatal("foreign provider authority admitted", kind)
			}
			continue
		}
		if !p.issued || p.returned || first.Code != 200 || !bytes.Contains(first.Body.Bytes(), []byte(kimiAttemptTool)) {
			t.Fatal("exact first call not owned")
		}
		args := map[string]any{"nonce": p.nonce, "operation": "write", "marker": p.marker}
		if kind == "wrong-arguments" {
			args["operation"] = "read"
		}
		encoded, _ := json.Marshal(args)
		id := p.id
		if kind == "foreign-call-id" {
			id = "foreign-call"
		}
		second := kimiAttemptContractBody()
		second["messages"] = append(second["messages"].([]any), map[string]any{"role": "assistant", "content": nil, "tool_calls": []any{map[string]any{"id": id, "type": "function", "function": map[string]any{"name": kimiAttemptTool, "arguments": string(encoded)}}}}, map[string]any{"role": "tool", "tool_call_id": id, "content": "owned facts"})
		send(second, false, false)
		if kind == "foreign-call-id" || kind == "wrong-arguments" {
			if p.returned || validated != 0 || p.err == nil {
				t.Fatal("foreign call/result joined", kind)
			}
			continue
		}
		if !p.returned || validated != 1 || p.err != nil || p.requestCount != 2 {
			t.Fatal("exact two-turn returned facts not joined")
		}
		if kind == "extra-turn" {
			send(second, false, false)
			if p.err == nil || p.requestCount != 3 {
				t.Fatal("third provider request accepted")
			}
		}
	}
}

func TestKimiSingleToolArtifactAdmissionCannotPromoteOrLeakPrivateKey(t *testing.T) {
	for _, kind := range []string{"valid", "qualified", "successful", "key-leak", "unknown-file", "partial-command", "provider-gap"} {
		result := map[string]any{"client": "kimi", "runtime_attempt_complete": false, "vendor_authentication_qualified": false, "real_model_qualified": false, "test_failed": true,
			"qualification": map[string]bool{"project-lifecycle": false, "native-restart-removal": false, "mcp-initialize": false, "mcp-tools-list": false, "mcp-tools-call": false}}
		if kind == "qualified" {
			result["qualification"].(map[string]bool)["mcp-tools-call"] = true
		}
		if kind == "successful" {
			result["test_failed"] = false
		}
		body, _ := json.Marshal(result)
		files := map[string][]byte{"runtime-attempt.json": body}
		switch kind {
		case "key-leak":
			files["python-api.json"] = []byte("uap-kimi-key-01234567890123456789012345678901")
		case "unknown-file":
			files["profile.json"] = []byte("{}")
		case "partial-command":
			files["native-attempt.stdout.log"] = []byte("incomplete")
		case "provider-gap":
			files["provider-request-2.json"] = []byte("{}")
		}
		if err := kimiAttemptAdmit(files); (err == nil) != (kind == "valid") {
			t.Fatalf("%s artifact: %v", kind, err)
		}
	}
}
