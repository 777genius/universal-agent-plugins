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

const kimiAttemptTool = "mcp__uap-kimi-native-a__inspect_runtime"
const kimiAttemptModel = "uap-kimi-native-model"

// Source b8dd3a134060c6e80ba4a1659335cd63b68483f2, OpenAI format
// d074e9c1fb310d78f3dd1f497f28bb4481e2ed75. This is a source-guided
// capture grammar; unfamiliar requests fail instead of expanding tool authority.
func kimiAttemptRequest(body []byte) (map[string]any, error) {
	v, err := projectNativeRuntimeJSON(body)
	if err != nil {
		return nil, err
	}
	m, ok := v.(map[string]any)
	if !ok || projectNativeRuntimeKeys(m, "model messages tools stream stream_options", "max_tokens max_completion_tokens temperature top_p parallel_tool_calls") != nil || m["model"] != kimiAttemptModel || m["stream"] != true || !reflect.DeepEqual(m["stream_options"], map[string]any{"include_usage": true}) {
		return nil, fmt.Errorf("Kimi completion envelope differs")
	}
	for _, key := range []string{"max_tokens", "max_completion_tokens"} {
		if value, exists := m[key]; exists && !projectNativeRuntimeNumber(value, 1, 1<<20, true) {
			return nil, fmt.Errorf("Kimi token bound differs")
		}
	}
	for _, key := range []string{"temperature", "top_p"} {
		high := 2.0
		if key == "top_p" {
			high = 1
		}
		if value, exists := m[key]; exists && !projectNativeRuntimeNumber(value, 0, high, false) {
			return nil, fmt.Errorf("Kimi sampling bound differs")
		}
	}
	if value, exists := m["parallel_tool_calls"]; exists {
		if _, ok := value.(bool); !ok {
			return nil, fmt.Errorf("Kimi parallel flag differs")
		}
	}
	rows, ok := m["tools"].([]any)
	if !ok || len(rows) != 1 {
		return nil, fmt.Errorf("Kimi exact installed tool missing or foreign tool offered")
	}
	row, ok := rows[0].(map[string]any)
	if !ok || projectNativeRuntimeKeys(row, "type function", "") != nil || row["type"] != "function" {
		return nil, fmt.Errorf("Kimi tool shape differs")
	}
	fn, ok := row["function"].(map[string]any)
	if !ok || projectNativeRuntimeKeys(fn, "name description parameters", "") != nil || fn["name"] != kimiAttemptTool || fn["description"] != "Inspect only disposable fixture runtime facts." {
		return nil, fmt.Errorf("Kimi installed declaration differs")
	}
	if err := projectNativeRuntimeSchema(fn["parameters"]); err != nil {
		return nil, err
	}
	messages, ok := m["messages"].([]any)
	if !ok || len(messages) == 0 || len(messages) > 256 {
		return nil, fmt.Errorf("Kimi message inventory differs")
	}
	for _, raw := range messages {
		message, ok := raw.(map[string]any)
		if !ok || projectNativeRuntimeKeys(message, "role content", "tool_call_id tool_calls") != nil {
			return nil, fmt.Errorf("Kimi message fields differ")
		}
		role := message["role"]
		if role != "system" && role != "user" && role != "assistant" && role != "tool" {
			return nil, fmt.Errorf("Kimi message role differs")
		}
		content := message["content"]
		if content == nil {
			if role != "assistant" || message["tool_calls"] == nil {
				return nil, fmt.Errorf("Kimi null message content differs")
			}
		} else if !projectNativeRuntimeText(content, projectNativeRuntimeBodyLimit) {
			parts, ok := content.([]any)
			if !ok || len(parts) == 0 || len(parts) > 128 {
				return nil, fmt.Errorf("Kimi content parts differ")
			}
			for _, part := range parts {
				p, ok := part.(map[string]any)
				if !ok || projectNativeRuntimeKeys(p, "type text", "") != nil || p["type"] != "text" || !projectNativeRuntimeText(p["text"], projectNativeRuntimeBodyLimit) {
					return nil, fmt.Errorf("Kimi content text differs")
				}
			}
		}
		if id, exists := message["tool_call_id"]; exists && (role != "tool" || !projectNativeRuntimeText(id, 256)) {
			return nil, fmt.Errorf("Kimi tool response identity differs")
		}
		if role == "tool" && message["tool_call_id"] == nil {
			return nil, fmt.Errorf("Kimi tool response identity absent")
		}
		if calls, exists := message["tool_calls"]; exists {
			calls, ok := calls.([]any)
			if role != "assistant" || !ok || len(calls) != 1 {
				return nil, fmt.Errorf("Kimi assistant tool call differs")
			}
			call, ok := calls[0].(map[string]any)
			if !ok || projectNativeRuntimeKeys(call, "id type function", "") != nil || call["type"] != "function" || !projectNativeRuntimeText(call["id"], 256) {
				return nil, fmt.Errorf("Kimi assistant call fields differ")
			}
			fn, ok := call["function"].(map[string]any)
			if !ok || projectNativeRuntimeKeys(fn, "name arguments", "") != nil || fn["name"] != kimiAttemptTool || !projectNativeRuntimeText(fn["arguments"], 4096) {
				return nil, fmt.Errorf("Kimi assistant function differs")
			}
			if _, err := projectNativeRuntimeJSON([]byte(fn["arguments"].(string))); err != nil {
				return nil, err
			}
		}
	}
	return m, nil
}

type kimiAttemptProvider struct {
	mu                               sync.Mutex
	server                           *httptest.Server
	key, id, nonce, marker, sentinel string
	requests                         [][]byte
	requestCount                     int
	err                              error
	issued, returned                 bool
	validate                         func(string) error
}

func (p *kimiAttemptProvider) start() error {
	listener, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp4", "127.0.0.1:0")
	if err != nil {
		return err
	}
	p.server = &httptest.Server{Listener: listener, Config: &http.Server{Handler: http.HandlerFunc(p.serve), ReadHeaderTimeout: time.Second, ReadTimeout: 5 * time.Second, WriteTimeout: 5 * time.Second, IdleTimeout: time.Second}}
	p.server.Start()
	return nil
}

func (p *kimiAttemptProvider) serve(w http.ResponseWriter, r *http.Request) {
	p.mu.Lock()
	defer p.mu.Unlock()
	fail := func(err error) {
		if p.err == nil {
			p.err = err
		}
		http.Error(w, "scripted Kimi provider refused request", http.StatusBadRequest)
	}
	p.requestCount++
	if p.err != nil || p.requestCount > 8 || r.Method != "POST" || r.URL.RequestURI() != "/v1/chat/completions" || r.Host != strings.TrimPrefix(p.server.URL, "http://") || len(r.Header.Values("Authorization")) != 1 || r.Header.Get("Authorization") != "Bearer "+p.key || r.Header.Get("api-key") != "" || strings.Split(r.Header.Get("Content-Type"), ";")[0] != "application/json" {
		fail(fmt.Errorf("Kimi provider route/host/credential/request bound differs"))
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, projectNativeRuntimeBodyLimit+1))
	if err != nil || len(body) > projectNativeRuntimeBodyLimit {
		fail(fmt.Errorf("Kimi provider body bound differs"))
		return
	}
	p.requests = append(p.requests, bytes.Clone(body))
	m, err := kimiAttemptRequest(body)
	if err != nil {
		fail(err)
		return
	}
	messages := m["messages"].([]any)
	args := map[string]any{"nonce": p.nonce, "operation": "write", "marker": p.marker}
	if len(p.requests) == 1 {
		for _, raw := range messages {
			message := raw.(map[string]any)
			if message["role"] == "tool" || message["tool_calls"] != nil {
				fail(fmt.Errorf("Kimi tool result/call precedes issued call"))
				return
			}
		}
		encoded, _ := json.Marshal(args)
		p.issued = true // Establish ownership before a byte can reach the native client.
		p.chunk(w, map[string]any{"role": "assistant", "tool_calls": []any{map[string]any{"index": 0, "id": p.id, "type": "function", "function": map[string]any{"name": kimiAttemptTool, "arguments": string(encoded)}}}}, nil)
		p.chunk(w, map[string]any{}, "tool_calls")
	} else if len(p.requests) == 2 {
		calls, results := 0, 0
		for _, raw := range messages {
			message := raw.(map[string]any)
			if rows, ok := message["tool_calls"].([]any); ok {
				calls += len(rows)
				call := rows[0].(map[string]any)
				fn := call["function"].(map[string]any)
				actual, err := projectNativeRuntimeJSON([]byte(fn["arguments"].(string)))
				if err != nil || call["id"] != p.id || fn["name"] != kimiAttemptTool || !reflect.DeepEqual(actual, args) {
					fail(fmt.Errorf("Kimi returned assistant call differs"))
					return
				}
			}
			if message["role"] == "tool" {
				results++
				if message["tool_call_id"] != p.id || !projectNativeRuntimeText(message["content"], projectNativeRuntimeBodyLimit) {
					fail(fmt.Errorf("Kimi returned tool identity differs"))
					return
				}
			}
		}
		last := messages[len(messages)-1].(map[string]any)
		if !p.issued || calls != 1 || results != 1 || last["role"] != "tool" {
			fail(fmt.Errorf("Kimi exact second turn absent"))
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
		fail(fmt.Errorf("Kimi unexpected completion turn"))
		return
	}
	_, _ = io.WriteString(w, "data: [DONE]\n\n")
}

func (p *kimiAttemptProvider) chunk(w http.ResponseWriter, delta map[string]any, finish any) {
	w.Header().Set("Content-Type", "text/event-stream")
	frame := map[string]any{"id": p.id, "object": "chat.completion.chunk", "created": time.Now().Unix(), "model": kimiAttemptModel, "choices": []any{map[string]any{"index": 0, "delta": delta, "finish_reason": finish}}}
	body, _ := json.Marshal(frame)
	_, _ = fmt.Fprintf(w, "data: %s\n\n", body)
	if flush, ok := w.(http.Flusher); ok {
		flush.Flush()
	}
}
