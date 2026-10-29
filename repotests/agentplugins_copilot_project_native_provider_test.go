//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"time"
)

const copilotProjectBodyLimit = projectNativeRuntimeBodyLimit

func copilotProjectMessage(v any) error {
	m, ok := v.(map[string]any)
	if !ok || projectNativeRuntimeKeys(m, "role", "content name tool_calls tool_call_id refusal") != nil {
		return fmt.Errorf("message object")
	}
	role, ok := m["role"].(string)
	if !ok {
		return fmt.Errorf("message role")
	}
	switch role {
	case "system", "developer", "user", "assistant", "tool":
	default:
		return fmt.Errorf("message role")
	}
	content, has := m["content"]
	_, hasCalls := m["tool_calls"]
	if (!has || content == nil) && (role != "assistant" || !hasCalls) {
		return fmt.Errorf("missing message content")
	}
	if role == "tool" {
		if id, ok := m["tool_call_id"]; !ok || !projectNativeRuntimeText(id, 256) {
			return fmt.Errorf("missing tool response identity")
		}
	}
	if has && content != nil && !projectNativeRuntimeText(content, copilotProjectBodyLimit) {
		parts, ok := content.([]any)
		if !ok || len(parts) > 128 {
			return fmt.Errorf("message content")
		}
		for _, part := range parts {
			p, ok := part.(map[string]any)
			if !ok || projectNativeRuntimeKeys(p, "type text", "") != nil || p["type"] != "text" || !projectNativeRuntimeText(p["text"], copilotProjectBodyLimit) {
				return fmt.Errorf("message text part")
			}
		}
	}
	if name, exists := m["name"]; exists && !projectNativeRuntimeText(name, 256) {
		return fmt.Errorf("message name")
	}
	if refusal, exists := m["refusal"]; exists && refusal != nil && !projectNativeRuntimeText(refusal, 4096) {
		return fmt.Errorf("message refusal")
	}
	if id, exists := m["tool_call_id"]; exists && (role != "tool" || !projectNativeRuntimeText(id, 256)) {
		return fmt.Errorf("tool message id")
	}
	if calls, exists := m["tool_calls"]; exists {
		rows, ok := calls.([]any)
		if role != "assistant" || !ok || len(rows) != 1 {
			return fmt.Errorf("assistant calls")
		}
		for _, row := range rows {
			c, ok := row.(map[string]any)
			if !ok || projectNativeRuntimeKeys(c, "id type function", "") != nil || c["type"] != "function" || !projectNativeRuntimeText(c["id"], 256) {
				return fmt.Errorf("assistant call")
			}
			f, ok := c["function"].(map[string]any)
			if !ok || projectNativeRuntimeKeys(f, "name arguments", "") != nil || !projectNativeRuntimeText(f["name"], 64) || !projectNativeRuntimeText(f["arguments"], 4096) {
				return fmt.Errorf("assistant function")
			}
			if _, err := projectNativeRuntimeJSON([]byte(f["arguments"].(string))); err != nil {
				return err
			}
		}
	}
	return nil
}

func copilotProjectEnvelope(body []byte, tool string) (map[string]any, error) {
	v, err := projectNativeRuntimeJSON(body)
	if err != nil {
		return nil, err
	}
	m, ok := v.(map[string]any)
	if !ok || projectNativeRuntimeKeys(m, "model messages stream", "tools tool_choice stream_options max_tokens max_completion_tokens temperature top_p parallel_tool_calls stop presence_penalty frequency_penalty seed user response_format reasoning_effort") != nil || m["model"] != "uap-fixture-model" || m["stream"] != true {
		return nil, fmt.Errorf("completion envelope")
	}
	messages, ok := m["messages"].([]any)
	if !ok || len(messages) == 0 || len(messages) > 256 {
		return nil, fmt.Errorf("completion messages")
	}
	for _, message := range messages {
		if err := copilotProjectMessage(message); err != nil {
			return nil, err
		}
	}
	for _, key := range []string{"max_tokens", "max_completion_tokens"} {
		if x, exists := m[key]; exists && !projectNativeRuntimeNumber(x, 1, 1<<20, true) {
			return nil, fmt.Errorf("token limit")
		}
	}
	for _, key := range []string{"temperature", "top_p", "presence_penalty", "frequency_penalty"} {
		low, high := -2.0, 2.0
		if key == "temperature" {
			low = 0
		}
		if key == "top_p" {
			low, high = 0, 1
		}
		if x, exists := m[key]; exists && !projectNativeRuntimeNumber(x, -2, 2, false) {
			return nil, fmt.Errorf("sampling number")
		}
		if x, exists := m[key]; exists && !projectNativeRuntimeNumber(x, low, high, false) {
			return nil, fmt.Errorf("sampling range")
		}
	}
	if x, exists := m["seed"]; exists && !projectNativeRuntimeNumber(x, -(1<<53), 1<<53, true) {
		return nil, fmt.Errorf("seed")
	}
	if x, exists := m["parallel_tool_calls"]; exists {
		if _, ok := x.(bool); !ok {
			return nil, fmt.Errorf("parallel tool calls")
		}
	}
	for _, key := range []string{"user", "reasoning_effort"} {
		if x, exists := m[key]; exists && !projectNativeRuntimeText(x, 256) {
			return nil, fmt.Errorf("completion text option")
		}
	}
	if x, exists := m["stream_options"]; exists {
		row, ok := x.(map[string]any)
		if !ok || projectNativeRuntimeKeys(row, "", "include_usage") != nil {
			return nil, fmt.Errorf("stream options")
		}
		for _, v := range row {
			if _, ok := v.(bool); !ok {
				return nil, fmt.Errorf("stream option type")
			}
		}
	}
	if x, exists := m["tool_choice"]; exists && x != "auto" && x != "required" && x != "none" {
		row, ok := x.(map[string]any)
		if !ok || projectNativeRuntimeKeys(row, "type function", "") != nil || row["type"] != "function" || !reflect.DeepEqual(row["function"], map[string]any{"name": tool}) {
			return nil, fmt.Errorf("tool choice")
		}
	}
	if x, exists := m["stop"]; exists && x != nil && !projectNativeRuntimeText(x, 256) {
		rows, ok := x.([]any)
		if !ok || len(rows) > 4 {
			return nil, fmt.Errorf("stop")
		}
		for _, v := range rows {
			if !projectNativeRuntimeText(v, 256) {
				return nil, fmt.Errorf("stop string")
			}
		}
	}
	if x, exists := m["response_format"]; exists {
		row, ok := x.(map[string]any)
		if !ok || projectNativeRuntimeKeys(row, "type", "") != nil || (row["type"] != "text" && row["type"] != "json_object") {
			return nil, fmt.Errorf("response format")
		}
	}
	return m, nil
}

func copilotProjectRequest(body []byte, tool string) (map[string]any, error) {
	m, err := copilotProjectEnvelope(body, tool)
	if err != nil {
		return nil, err
	}
	tools, ok := m["tools"].([]any)
	if !ok || len(tools) != 1 {
		return nil, fmt.Errorf("exact selected installed tool missing or foreign tool available")
	}
	row, ok := tools[0].(map[string]any)
	if !ok || projectNativeRuntimeKeys(row, "type function", "") != nil || row["type"] != "function" {
		return nil, fmt.Errorf("function tool")
	}
	function, ok := row["function"].(map[string]any)
	if !ok || projectNativeRuntimeKeys(function, "name description parameters", "strict") != nil || function["name"] != tool || !projectNativeRuntimeText(function["description"], 16384) {
		return nil, fmt.Errorf("selected tool declaration")
	}
	if x, exists := function["strict"]; exists {
		if _, ok := x.(bool); !ok {
			return nil, fmt.Errorf("strict")
		}
	}
	if err := projectNativeRuntimeSchema(function["parameters"]); err != nil {
		return nil, err
	}
	return m, nil
}

type copilotProjectProvider struct {
	mu                                sync.Mutex
	server                            *httptest.Server
	tool, id, nonce, marker, sentinel string
	requests                          [][]byte
	err                               error
	returned                          bool
	validate                          func(string) error
}

func newCopilotProjectProvider(tool, nonce string, validate func(string) error) *copilotProjectProvider {
	p := &copilotProjectProvider{tool: tool, id: "uap_" + nonce + "_attempt", nonce: nonce, marker: "retained-" + nonce, sentinel: "UAP_DONE_" + nonce + "_attempt", validate: validate}
	return p
}
func (p *copilotProjectProvider) start(ctx context.Context) error {
	listener, err := (&net.ListenConfig{}).Listen(ctx, "tcp4", "127.0.0.1:0")
	if err != nil {
		return err
	}
	p.server = &httptest.Server{Listener: listener, Config: &http.Server{Handler: http.HandlerFunc(p.serve), ReadHeaderTimeout: time.Second, ReadTimeout: 5 * time.Second, WriteTimeout: 5 * time.Second, IdleTimeout: time.Second}}
	p.server.Start()
	return nil
}
func (p *copilotProjectProvider) serve(w http.ResponseWriter, r *http.Request) {
	p.mu.Lock()
	defer p.mu.Unlock()
	fail := func(err error) {
		if p.err == nil {
			p.err = err
		}
		http.Error(w, "scripted provider rejected request", http.StatusBadRequest)
	}
	if len(p.requests) >= 8 || r.Method != "POST" || r.URL.RequestURI() != "/v1/chat/completions" || r.Host != strings.TrimPrefix(p.server.URL, "http://") || r.Header.Get("Authorization") != "" || r.Header.Get("api-key") != "" || strings.Split(r.Header.Get("Content-Type"), ";")[0] != "application/json" {
		fail(fmt.Errorf("provider route/host/credential/request bound"))
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, copilotProjectBodyLimit+1))
	if err != nil || len(body) > copilotProjectBodyLimit {
		fail(fmt.Errorf("provider body bound"))
		return
	}
	p.requests = append(p.requests, bytes.Clone(body))
	m, err := copilotProjectRequest(body, p.tool)
	if err != nil {
		fail(err)
		return
	}
	args := map[string]any{"nonce": p.nonce, "operation": "write", "marker": p.marker}
	messages := m["messages"].([]any)
	if len(p.requests) == 1 {
		for _, v := range messages {
			if v.(map[string]any)["role"] == "tool" {
				fail(fmt.Errorf("tool output before call"))
				return
			}
		}
		b, _ := json.Marshal(args)
		p.chunk(w, map[string]any{"role": "assistant", "tool_calls": []any{map[string]any{"index": 0, "id": p.id, "type": "function", "function": map[string]any{"name": p.tool, "arguments": string(b)}}}}, nil)
		p.chunk(w, map[string]any{}, "tool_calls")
	} else if len(p.requests) == 2 {
		calls := 0
		for _, value := range messages {
			message := value.(map[string]any)
			if rows, ok := message["tool_calls"].([]any); ok {
				calls += len(rows)
				for _, value := range rows {
					row := value.(map[string]any)
					function := row["function"].(map[string]any)
					parsed, parseErr := projectNativeRuntimeJSON([]byte(function["arguments"].(string)))
					if parseErr != nil || row["id"] != p.id || function["name"] != p.tool || !reflect.DeepEqual(parsed, args) {
						fail(fmt.Errorf("native assistant call identity/arguments differ"))
						return
					}
				}
			}
		}
		if calls != 1 {
			fail(fmt.Errorf("missing/duplicate native assistant tool call"))
			return
		}
		last := messages[len(messages)-1].(map[string]any)
		if last["role"] != "tool" || last["tool_call_id"] != p.id || !projectNativeRuntimeText(last["content"], copilotProjectBodyLimit) {
			fail(fmt.Errorf("missing matching returned tool result"))
			return
		}
		if err := p.validate(last["content"].(string)); err != nil {
			fail(err)
			return
		}
		p.returned = true
		p.chunk(w, map[string]any{"role": "assistant", "content": p.sentinel}, nil)
		p.chunk(w, map[string]any{}, "stop")
	} else {
		fail(fmt.Errorf("unexpected completion turn"))
		return
	}
	_, _ = io.WriteString(w, "data: [DONE]\n\n")
}
func (p *copilotProjectProvider) chunk(w http.ResponseWriter, delta map[string]any, finish any) {
	w.Header().Set("Content-Type", "text/event-stream")
	body, _ := json.Marshal(map[string]any{"id": "chatcmpl-" + p.nonce, "object": "chat.completion.chunk", "created": 1, "model": "uap-fixture-model", "choices": []any{map[string]any{"index": 0, "delta": delta, "finish_reason": finish}}})
	_, _ = fmt.Fprintf(w, "data: %s\n\n", body)
}
