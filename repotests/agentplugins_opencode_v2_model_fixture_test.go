package pluginkitairepo_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// openCodeV2ModelFixture models only one deterministic local tool call. It is
// never a credentialed model or a direct invocation of the MCP helper. OpenCode
// must discover the MCP tool itself and send it to the model request.
type openCodeV2ModelFixture struct {
	server   *httptest.Server
	mu       sync.Mutex
	seenTool int
	seenCall int
	seenDone int
	invalid  int
}

type openCodeV2ModelRequest struct {
	Model    string           `json:"model"`
	Stream   bool             `json:"stream"`
	Tools    []map[string]any `json:"tools"`
	Messages []map[string]any `json:"messages"`
}

func newOpenCodeV2ModelFixture(t *testing.T) *openCodeV2ModelFixture {
	t.Helper()
	model := &openCodeV2ModelFixture{}
	model.server = httptest.NewServer(http.HandlerFunc(model.respond))
	t.Cleanup(model.server.Close)
	return model
}

func (m *openCodeV2ModelFixture) baseURL() string {
	return m.server.URL + "/v1"
}
func (m *openCodeV2ModelFixture) stats() (tool, call, done, invalid int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.seenTool, m.seenCall, m.seenDone, m.invalid
}

const openCodeV2CallID = "call_uap_fixture_1"
const openCodeV2ModelID = "uap-scripted"
const openCodeV2ToolName = "uap_project_fixture_uap_ping"

func (m *openCodeV2ModelFixture) respond(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet && r.URL.Path == "/v1/models" {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"object":"list","data":[{"object":"model","id":"uap-scripted"}]}`)
		return
	}
	if r.Method != http.MethodPost || r.URL.Path != "/v1/chat/completions" {
		m.mu.Lock()
		m.invalid++
		m.mu.Unlock()
		http.Error(w, "unsupported local scripted provider route", http.StatusNotFound)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, (1<<20)+1))
	if err != nil || len(body) > 1<<20 {
		http.Error(w, "unbounded model request", http.StatusRequestEntityTooLarge)
		return
	}
	var input openCodeV2ModelRequest
	if json.Unmarshal(body, &input) != nil || !strings.Contains(input.Model, openCodeV2ModelID) ||
		len(input.Messages) > 128 || len(input.Tools) > 128 {
		m.mu.Lock()
		m.invalid++
		m.mu.Unlock()
		http.Error(w, "invalid scripted model request", http.StatusBadRequest)
		return
	}
	found := false
	for _, tool := range input.Tools {
		function, _ := tool["function"].(map[string]any)
		if function["name"] == openCodeV2ToolName {
			found = true
			break
		}
	}
	hasResult := false
	for _, message := range input.Messages {
		if message["role"] == "tool" && message["tool_call_id"] == openCodeV2CallID &&
		strings.Contains(fmt.Sprint(message["content"]), "uap-pong") {
			hasResult = true
		}
	}
	m.mu.Lock()
	if found {
		m.seenTool++
	}
	call := found && !hasResult && m.seenCall == 0
	if call {
		m.seenCall++
	}
	if hasResult {
		m.seenDone++
	}
	m.mu.Unlock()
	choice := map[string]any{"index": 0, "delta": map[string]any{"role": "assistant"}, "finish_reason": nil}
	finish := "stop"
	if call {
		choice["delta"] = map[string]any{"role": "assistant", "tool_calls": []any{
			map[string]any{"index": 0, "id": openCodeV2CallID, "type": "function",
				"function": map[string]any{"name": openCodeV2ToolName, "arguments": "{}"}},
		}}
		finish = "tool_calls"
	} else {
		choice["delta"] = map[string]any{"role": "assistant", "content": "Local E2E response."}
	}
	if input.Stream {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		stream := func(choices []any) {
			event := map[string]any{
				"id": "chatcmpl-uap-local-fixture", "object": "chat.completion.chunk",
				"created": 1, "model": openCodeV2ModelID, "choices": choices,
			}
			data, _ := json.Marshal(event)
			_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
		}
		stream([]any{choice})
		stream([]any{map[string]any{"index": 0, "delta": map[string]any{}, "finish_reason": finish}})
		_, _ = io.WriteString(w, "data: [DONE]\n\n")
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		return
	}
	w.Header().Set("Content-Type", "application/json")
	message := map[string]any{"role": "assistant", "content": "Local E2E response."}
	if call {
		message["content"] = nil
		message["tool_calls"] = []any{map[string]any{
			"id": openCodeV2CallID, "type": "function",
			"function": map[string]string{"name": openCodeV2ToolName, "arguments": "{}"},
		}}
	}
	output, _ := json.Marshal(map[string]any{
		"id": "chatcmpl-uap-local-fixture", "object": "chat.completion",
		"created": 1, "model": openCodeV2ModelID,
		"choices": []any{map[string]any{"index": 0, "message": message, "finish_reason": finish}},
		"usage":   map[string]int{"prompt_tokens": 8, "completion_tokens": 4, "total_tokens": 12},
	})
	_, _ = w.Write(output)
}
