//go:build linux

package pluginkitairepo_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"
)

const clineACPRefusalOutcome = "cancelled" //nolint:misspell // Exact native ACP wire value.

// Source grammar: cline c7185735 ACP/permissions/tool-utils and the shipping
// lock's @ai-sdk/openai-compatible 3.0.58 (chat source 17d09ac4). Unknown wire
// extensions fail; initial genuine captures cannot turn qualification true.
type clineProjectProvider struct {
	mu                                      sync.Mutex
	server                                  *http.Server
	listener                                net.Listener
	url, key, nonce, callID, marker, prompt string
	requests, responses                     [][]byte
	issued, approved                        bool
	facts                                   map[string]any
	failure                                 error
	cancel                                  context.CancelFunc
}

func newClineProjectProvider(t *testing.T, f *clineProjectAttempt) *clineProjectProvider {
	t.Helper()
	listener, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	p := &clineProjectProvider{listener: listener, url: "http://" + listener.Addr().String() + "/v1", key: f.key, nonce: f.base.Nonce, callID: "uap-cline-call-" + f.base.Nonce, marker: "retained-" + f.base.Nonce, prompt: "UAP_CLINE_PROMPT_" + f.base.Nonce, cancel: f.cancel}
	p.server = &http.Server{Handler: http.HandlerFunc(p.serve), ReadHeaderTimeout: 3 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 3 * time.Second, MaxHeaderBytes: 16 << 10}
	go func() {
		err := p.server.Serve(listener)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			p.fail(err)
		}
	}()
	return p
}
func (p *clineProjectProvider) fail(err error) {
	p.mu.Lock()
	if p.failure == nil {
		p.failure = err
	}
	p.mu.Unlock()
	p.cancel()
}
func (p *clineProjectProvider) configure(t *testing.T, f *clineProjectAttempt) {
	t.Helper()
	settings := map[string]any{"version": 1, "modes": map[string]any{}, "lastUsedProvider": clineAttemptProviderID, "providers": map[string]any{clineAttemptProviderID: map[string]any{"settings": map[string]any{"provider": clineAttemptProviderID, "apiKey": p.key, "model": clineAttemptModel, "protocol": "openai-chat", "client": "openai-compatible", "baseUrl": p.url, "timeout": 30000, "maxTokens": 512, "contextWindow": 32768, "capabilities": []string{"streaming", "tools"}, "modelCatalog": map[string]any{"loadLatestOnInit": false, "includeClineCloudModels": false, "loadPrivateOnAuth": false, "failOnError": true}}, "updatedAt": "2026-10-25T00:00:00.000Z", "tokenSource": "manual"}}}
	models := map[string]any{"version": 1, "providers": map[string]any{clineAttemptProviderID: map[string]any{"provider": map[string]any{"name": "UAP scripted loopback", "baseUrl": p.url, "defaultModelId": clineAttemptModel, "protocol": "openai-chat", "client": "openai-compatible", "capabilities": []string{"streaming", "tools"}}, "models": map[string]any{clineAttemptModel: map[string]any{"id": clineAttemptModel, "name": "UAP scripted model", "maxTokens": 512, "contextWindow": 32768, "capabilities": []string{"streaming", "tools"}}}}}}
	for name, value := range map[string]any{"providers.json": settings, "models.json": models} {
		raw, _ := json.Marshal(value)
		path := filepath.Join(f.base.Home, ".cline", "data", "settings", name)
		projectNativeRuntimeWrite(t, path, raw, 0600)
		f.protected[path] = clineAttemptSHA(raw)
	}
	f.evidence["provider_mode"] = "supported original-HOME user custom-provider catalog; loopback only; external catalog refresh disabled; fake key omitted"
}
func (p *clineProjectProvider) close(t *testing.T) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := p.server.Shutdown(ctx); err != nil {
		_ = p.server.Close()
		t.Error("Cline loopback provider did not stop within cleanup budget")
	}
}
func (p *clineProjectProvider) capture(t *testing.T, f *clineProjectAttempt) {
	t.Helper()
	p.mu.Lock()
	defer p.mu.Unlock()
	for i, raw := range p.requests {
		f.keep(t, fmt.Sprintf("provider-%d.request.json", i+1), raw)
	}
	for i, raw := range p.responses {
		f.keep(t, fmt.Sprintf("provider-%d.response.log", i+1), raw)
	}
	meta := map[string]any{"requests": len(p.requests), "responses": len(p.responses), "exact_call_issued": p.issued, "single_call_approved": p.approved, "error": p.failure != nil, "transport": "loopback scripted OpenAI chat-completions SSE; no real model"}
	if p.failure != nil {
		meta["error_text"] = p.failure.Error()
	}
	raw, _ := json.Marshal(meta)
	f.keep(t, "provider.json", raw)
}
func (p *clineProjectProvider) arguments() map[string]any {
	return map[string]any{"nonce": p.nonce, "operation": "write", "marker": p.marker}
}
func clineAttemptProviderRequest(r *http.Request, host, key string) bool {
	return r.Method == "POST" && r.Host == host && !r.URL.IsAbs() && r.URL.Host == "" && r.URL.Path == "/v1/chat/completions" && r.URL.RawPath == "" && r.URL.RawQuery == "" && r.Header.Get("Authorization") == "Bearer "+key && len(r.Header.Values("Authorization")) == 1 && r.Header.Get("Content-Type") == "application/json" && len(r.Header.Values("Content-Type")) == 1
}
func (p *clineProjectProvider) serve(w http.ResponseWriter, r *http.Request) {
	if !clineAttemptProviderRequest(r, p.listener.Addr().String(), p.key) {
		http.Error(w, "closed provider request refused", 400)
		p.fail(fmt.Errorf("unknown loopback provider route/credentials/content type"))
		return
	}
	raw, err := io.ReadAll(io.LimitReader(r.Body, (1<<20)+1))
	if err != nil || len(raw) > 1<<20 {
		http.Error(w, "request bound", 400)
		p.fail(fmt.Errorf("provider request capture bound"))
		return
	}
	p.mu.Lock()
	index := len(p.requests)
	if index < 2 {
		p.requests = append(p.requests, bytes.Clone(raw))
	}
	p.mu.Unlock()
	if index >= 2 {
		http.Error(w, "request count", 400)
		p.fail(fmt.Errorf("third provider request forbidden"))
		return
	}
	facts, err := p.validateRequest(raw, index)
	if err != nil {
		http.Error(w, "closed provider grammar refused", 400)
		p.fail(err)
		return
	}
	p.mu.Lock()
	if index == 0 {
		p.issued = true
	} else {
		p.facts = facts
	}
	p.mu.Unlock()
	var chunks []map[string]any
	base := map[string]any{"id": "uap-cline-completion-" + p.nonce, "object": "chat.completion.chunk", "created": 1, "model": clineAttemptModel}
	chunk := func(delta map[string]any, reason any) map[string]any {
		m := map[string]any{}
		for k, v := range base {
			m[k] = v
		}
		m["choices"] = []any{map[string]any{"index": 0, "delta": delta, "finish_reason": reason}}
		return m
	}
	if index == 0 {
		args, _ := json.Marshal(p.arguments())
		chunks = []map[string]any{chunk(map[string]any{"role": "assistant", "tool_calls": []any{map[string]any{"index": 0, "id": p.callID, "type": "function", "function": map[string]any{"name": clineAttemptTool, "arguments": string(args)}}}}, nil), chunk(map[string]any{}, "tool_calls")}
	} else {
		chunks = []map[string]any{chunk(map[string]any{"role": "assistant", "content": "UAP_CLINE_DONE_" + p.nonce}, nil), chunk(map[string]any{}, "stop")}
	}
	var response bytes.Buffer
	for _, c := range chunks {
		body, _ := json.Marshal(c)
		fmt.Fprintf(&response, "data: %s\n\n", body)
	}
	response.WriteString("data: [DONE]\n\n")
	p.mu.Lock()
	p.responses = append(p.responses, bytes.Clone(response.Bytes()))
	p.mu.Unlock()
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(200)
	if _, err := w.Write(response.Bytes()); err != nil {
		p.fail(fmt.Errorf("provider response write: %w", err))
		return
	}
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}
}
func (p *clineProjectProvider) validateRequest(raw []byte, index int) (map[string]any, error) {
	value, err := projectNativeRuntimeJSON(raw)
	if err != nil {
		return nil, err
	}
	m, ok := value.(map[string]any)
	if !ok || projectNativeRuntimeKeys(m, "model messages tools stream", "max_tokens temperature top_p frequency_penalty presence_penalty seed tool_choice stream_options") != nil || m["model"] != clineAttemptModel || m["stream"] != true {
		return nil, fmt.Errorf("unknown provider request grammar/model/stream")
	}
	for _, key := range []string{"max_tokens", "temperature", "top_p", "frequency_penalty", "presence_penalty", "seed"} {
		if v, exists := m[key]; exists && !projectNativeRuntimeNumber(v, -65536, 65536, false) {
			return nil, fmt.Errorf("invalid provider numeric option %s", key)
		}
	}
	if v, ok := m["tool_choice"]; ok && v != "auto" {
		return nil, fmt.Errorf("unexpected provider tool choice")
	}
	if v, ok := m["stream_options"]; ok && !reflect.DeepEqual(v, map[string]any{"include_usage": true}) {
		return nil, fmt.Errorf("unknown stream options")
	}
	tools, ok := m["tools"].([]any)
	if !ok || len(tools) == 0 || len(tools) > 128 {
		return nil, fmt.Errorf("bounded native tool catalog required")
	}
	seen := map[string]bool{}
	selected := false
	for _, v := range tools {
		tool, ok := v.(map[string]any)
		if !ok || projectNativeRuntimeKeys(tool, "type function", "") != nil || tool["type"] != "function" {
			return nil, fmt.Errorf("unknown native tool definition")
		}
		function, ok := tool["function"].(map[string]any)
		if !ok || projectNativeRuntimeKeys(function, "name parameters", "description strict") != nil {
			return nil, fmt.Errorf("unknown native function definition")
		}
		name, ok := function["name"].(string)
		if !ok || !regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`).MatchString(name) || seen[name] {
			return nil, fmt.Errorf("invalid duplicate native tool")
		}
		seen[name] = true
		if d, ok := function["description"]; ok && !projectNativeRuntimeText(d, 32768) {
			return nil, fmt.Errorf("tool description bound")
		}
		if _, ok := function["parameters"].(map[string]any); !ok {
			return nil, fmt.Errorf("tool schema object required")
		}
		if strict, ok := function["strict"]; ok && strict != false {
			return nil, fmt.Errorf("unobserved strict-tool conversion")
		}
		if name == clineAttemptTool {
			if projectNativeRuntimeSchema(function["parameters"]) != nil || function["description"] != "Inspect only disposable fixture runtime facts." {
				return nil, fmt.Errorf("installed MCP probe schema differs")
			}
			selected = true
		} else if strings.Contains(name, "__") {
			return nil, fmt.Errorf("foreign native MCP tool visible")
		}
	}
	if !selected {
		return nil, fmt.Errorf("installed A MCP tool absent from actual provider catalog")
	}
	messages, ok := m["messages"].([]any)
	if !ok || len(messages) < 2 || len(messages) > 96 {
		return nil, fmt.Errorf("bounded provider history required")
	}
	user := false
	assistantCalls, results := 0, 0
	var facts map[string]any
	for _, v := range messages {
		msg, ok := v.(map[string]any)
		if !ok || projectNativeRuntimeKeys(msg, "role content", "tool_calls tool_call_id name") != nil {
			return nil, fmt.Errorf("unknown message grammar")
		}
		role, _ := msg["role"].(string)
		switch role {
		case "system", "user":
			text, err := clineAttemptMessageText(msg["content"])
			if err != nil {
				return nil, err
			}
			if role == "user" && strings.Contains(text, p.prompt) {
				user = true
			}
			if msg["tool_calls"] != nil || msg["tool_call_id"] != nil {
				return nil, fmt.Errorf("tool metadata on user/system message")
			}
		case "assistant":
			if c := msg["content"]; c != nil {
				if _, err := clineAttemptMessageText(c); err != nil {
					return nil, err
				}
			}
			if calls, exists := msg["tool_calls"]; exists {
				list, ok := calls.([]any)
				if !ok || len(list) != 1 {
					return nil, fmt.Errorf("single assistant tool call required")
				}
				call, ok := list[0].(map[string]any)
				if !ok || projectNativeRuntimeKeys(call, "id type function", "") != nil || call["id"] != p.callID || call["type"] != "function" {
					return nil, fmt.Errorf("assistant tool ID/type differs")
				}
				fn, ok := call["function"].(map[string]any)
				if !ok || projectNativeRuntimeKeys(fn, "name arguments", "") != nil || fn["name"] != clineAttemptTool {
					return nil, fmt.Errorf("assistant tool name differs")
				}
				arguments, ok := fn["arguments"].(string)
				if !ok {
					return nil, fmt.Errorf("assistant arguments not text")
				}
				args, err := projectNativeRuntimeJSON([]byte(arguments))
				if err != nil || !reflect.DeepEqual(args, p.arguments()) {
					return nil, fmt.Errorf("assistant arguments differ")
				}
				assistantCalls++
			}
		case "tool":
			if msg["tool_call_id"] != p.callID || msg["tool_calls"] != nil {
				return nil, fmt.Errorf("tool result ID differs")
			}
			text, err := clineAttemptMessageText(msg["content"])
			if err != nil {
				return nil, err
			}
			facts, err = clineAttemptMCPResult([]byte(text))
			if err != nil {
				return nil, err
			}
			results++
		default:
			return nil, fmt.Errorf("unknown provider message role")
		}
	}
	if !user || index == 0 && (assistantCalls != 0 || results != 0) || index == 1 && (assistantCalls != 1 || results != 1) {
		return nil, fmt.Errorf("provider prompt/history/call count differs")
	}
	return facts, nil
}
func clineAttemptMessageText(v any) (string, error) {
	if text, ok := v.(string); ok && projectNativeRuntimeText(text, 512<<10) {
		return text, nil
	}
	blocks, ok := v.([]any)
	if !ok || len(blocks) > 64 {
		return "", fmt.Errorf("unknown text content grammar")
	}
	var out strings.Builder
	for _, v := range blocks {
		m, ok := v.(map[string]any)
		if !ok || projectNativeRuntimeKeys(m, "type text", "") != nil || m["type"] != "text" || !projectNativeRuntimeText(m["text"], 512<<10) {
			return "", fmt.Errorf("unknown nontext content")
		}
		out.WriteString(m["text"].(string))
	}
	return out.String(), nil
}
func clineAttemptMCPResult(raw []byte) (map[string]any, error) {
	value, err := projectNativeRuntimeJSON(raw)
	if err != nil {
		return nil, err
	}
	m, ok := value.(map[string]any)
	if !ok || projectNativeRuntimeKeys(m, "content isError", "") != nil || m["isError"] != false {
		return nil, fmt.Errorf("exact MCP result envelope required")
	}
	blocks, ok := m["content"].([]any)
	if !ok || len(blocks) != 1 {
		return nil, fmt.Errorf("single probe text result required")
	}
	text, ok := blocks[0].(map[string]any)
	if !ok || projectNativeRuntimeKeys(text, "type text", "") != nil || text["type"] != "text" || !projectNativeRuntimeText(text["text"], 16384) {
		return nil, fmt.Errorf("exact probe text block required")
	}
	return clineAttemptFacts([]byte(text["text"].(string)))
}
func clineAttemptFacts(raw []byte) (map[string]any, error) {
	v, err := projectNativeRuntimeJSON(raw)
	if err != nil {
		return nil, err
	}
	m, ok := v.(map[string]any)
	if !ok || projectNativeRuntimeKeys(m, "nonce revision cwd argv plugin_root plugin_data literal once marker pid session", "") != nil {
		return nil, fmt.Errorf("exact lower-case probe facts required")
	}
	for _, key := range []string{"nonce", "revision", "cwd", "plugin_root", "plugin_data", "literal", "once", "marker", "session"} {
		if _, ok := m[key].(string); !ok || !projectNativeRuntimeText(m[key], 4096) {
			return nil, fmt.Errorf("probe fact type %s", key)
		}
	}
	argv, ok := m["argv"].([]any)
	if !ok || len(argv) > 16 {
		return nil, fmt.Errorf("probe argv bound")
	}
	for _, arg := range argv {
		if !projectNativeRuntimeText(arg, 4096) {
			return nil, fmt.Errorf("probe argv type")
		}
	}
	if !projectNativeRuntimeNumber(m["pid"], 1, 1<<31, true) {
		return nil, fmt.Errorf("probe PID type")
	}
	return m, nil
}
func (p *clineProjectProvider) verify(f *clineProjectAttempt) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.failure != nil {
		return fmt.Errorf("complete Cline native/provider/approval join missing: %w", p.failure)
	}
	if len(p.requests) != 2 || len(p.responses) != 2 || !p.issued || !p.approved || p.facts == nil {
		return errors.New("complete Cline native/provider/approval join missing")
	}
	facts := p.facts
	if !reflect.DeepEqual(f.nativeFacts, facts) {
		return fmt.Errorf("ACP native tool result differs from actual provider history")
	}
	if facts["nonce"] != p.nonce || facts["revision"] != "a-one" || facts["cwd"] != f.active || facts["plugin_root"] != f.active || facts["plugin_data"] != f.data || facts["marker"] != p.marker || facts["literal"] != "" || facts["once"] != "" || !reflect.DeepEqual(facts["argv"], []any{"Owned Cline A", f.active + "/argument"}) {
		return fmt.Errorf("actual installed tool runtime facts differ from receipt/source")
	}
	marker, err := projectNativeRuntimeRead(filepath.Join(f.data, "native-marker.txt"), 256)
	if err != nil || string(marker) != p.marker {
		return fmt.Errorf("actual retained data marker missing")
	}
	raw, err := projectNativeRuntimeRead(f.base.Events, 1<<20)
	if err != nil {
		return err
	}
	return clineAttemptEvents(raw, p.nonce, facts, f.nativeStart, f.nativeEnd)
}
func clineAttemptEvents(raw []byte, nonce string, facts map[string]any, start, end time.Time) error {
	if start.IsZero() || end.Before(start) {
		return fmt.Errorf("native ACP fresh interval missing")
	}
	scan := bufio.NewScanner(bytes.NewReader(raw))
	scan.Buffer(make([]byte, 4096), 1<<20)
	methods := map[string]int{}
	var last time.Time
	phase := 0
	for scan.Scan() {
		v, err := projectNativeRuntimeJSON(scan.Bytes())
		if err != nil {
			return err
		}
		e, ok := v.(map[string]any)
		if !ok || projectNativeRuntimeKeys(e, "method pid session nonce timestamp facts", "") != nil || e["nonce"] != nonce || e["pid"] != facts["pid"] || e["session"] != facts["session"] {
			return fmt.Errorf("probe events not one native-owned process/session")
		}
		stamp, ok := e["timestamp"].(string)
		if !ok {
			return fmt.Errorf("probe event timestamp type")
		}
		at, err := time.Parse(time.RFC3339Nano, stamp)
		if err != nil || at.Before(last) || at.Before(start) || at.After(end) {
			return fmt.Errorf("probe timestamp order")
		}
		last = at
		method, _ := e["method"].(string)
		if method != "initialize" && method != "tools/list" && method != "tools/call" {
			return fmt.Errorf("unknown probe event")
		}
		switch method {
		case "initialize":
			if phase != 0 {
				return fmt.Errorf("probe initialize not first/exactly once")
			}
			phase = 1
		case "tools/list":
			if phase != 1 && phase != 2 {
				return fmt.Errorf("probe tools/list outside discovery phase")
			}
			phase = 2
		case "tools/call":
			if phase != 2 {
				return fmt.Errorf("probe tools/call before discovery or repeated")
			}
			phase = 3
		}
		methods[method]++
		if method == "tools/call" {
			body, _ := json.Marshal(e["facts"])
			actual, err := clineAttemptFacts(body)
			if err != nil || !reflect.DeepEqual(actual, facts) {
				return fmt.Errorf("native/provider/probe result mismatch")
			}
		} else if e["facts"] != nil {
			return fmt.Errorf("unexpected discovery event facts")
		}
	}
	if scan.Err() != nil || phase != 3 || methods["initialize"] != 1 || methods["tools/list"] < 1 || methods["tools/list"] > 3 || methods["tools/call"] != 1 {
		return fmt.Errorf("complete bounded initialize/list/single-call events missing")
	}
	return nil
}

func clineAttemptTitle(args map[string]any) string {
	raw, _ := json.Marshal(args)
	summary := string(raw)
	if len(summary) > 60 {
		summary = summary[:57] + "..."
	}
	return clineAttemptTool + ": " + summary
}
func clineAttemptPermission(params map[string]any, session, callID string, args map[string]any) error {
	if projectNativeRuntimeKeys(params, "sessionId toolCall options", "") != nil || params["sessionId"] != session {
		return fmt.Errorf("permission native session differs")
	}
	tool, ok := params["toolCall"].(map[string]any)
	if !ok || projectNativeRuntimeKeys(tool, "toolCallId title kind status rawInput", "") != nil || tool["toolCallId"] != callID || tool["title"] != clineAttemptTitle(args) || tool["kind"] != "other" || tool["status"] != "pending" || !reflect.DeepEqual(tool["rawInput"], args) {
		return fmt.Errorf("permission exact provider call/tool/input differs")
	}
	expected := []any{map[string]any{"optionId": "allow_once", "name": "Allow once", "kind": "allow_once"}, map[string]any{"optionId": "allow_always", "name": "Allow always", "kind": "allow_always"}, map[string]any{"optionId": "reject_once", "name": "Reject", "kind": "reject_once"}}
	if !reflect.DeepEqual(params["options"], expected) {
		return fmt.Errorf("native permission options differ")
	}
	return nil
}
func (f *clineProjectAttempt) acp(w io.Writer, r io.Reader, transcript *bytes.Buffer) error {
	write := func(m map[string]any) error {
		raw, err := json.Marshal(m)
		if err != nil {
			return err
		}
		if _, err = w.Write(append(raw, '\n')); err != nil {
			return err
		}
		return nil
	}
	send := func(id int, method string, params any) error {
		return write(map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params})
	}
	if err := send(1, "initialize", map[string]any{"protocolVersion": 1, "clientCapabilities": map[string]any{}, "clientInfo": map[string]string{"name": "uap-native-research", "version": "1.0.0"}}); err != nil {
		return err
	}
	scan := bufio.NewScanner(io.TeeReader(io.LimitReader(r, (1<<20)+1), transcript))
	scan.Buffer(make([]byte, 4096), 1<<20)
	stage := 1
	session := ""
	approved := false
	completed := false
	progress, chunks := 0, 0
	infoSeen := false
	text := ""
	requestIDs := map[string]bool{}
	for scan.Scan() {
		if transcript.Len() > 1<<20 {
			return fmt.Errorf("native ACP output bound")
		}
		v, err := projectNativeRuntimeJSON(scan.Bytes())
		if err != nil {
			return err
		}
		m, ok := v.(map[string]any)
		if !ok || m["jsonrpc"] != "2.0" {
			return fmt.Errorf("native ACP JSON-RPC envelope")
		}
		if method, ok := m["method"].(string); ok {
			if projectNativeRuntimeKeys(m, "jsonrpc method params", "id") != nil {
				return fmt.Errorf("unknown ACP request envelope")
			}
			params, ok := m["params"].(map[string]any)
			if !ok {
				return fmt.Errorf("native ACP params object required")
			}
			if method == "session/request_permission" {
				id, idOK := m["id"]
				encoded, _ := json.Marshal(id)
				idText := string(encoded)
				f.provider.mu.Lock()
				issued := f.provider.issued
				f.provider.mu.Unlock()
				validID := idOK && (projectNativeRuntimeText(id, 128) || projectNativeRuntimeNumber(id, 0, 1<<31, true))
				if !validID || requestIDs[idText] || stage != 3 || progress != 1 || approved || !issued || clineAttemptPermission(params, session, f.provider.callID, f.provider.arguments()) != nil {
					if validID {
						_ = write(map[string]any{"jsonrpc": "2.0", "id": id, "result": map[string]any{"outcome": map[string]any{"outcome": clineACPRefusalOutcome}}})
					}
					_ = write(map[string]any{"jsonrpc": "2.0", "method": "session/cancel", "params": map[string]string{"sessionId": session}})
					return fmt.Errorf("unknown, duplicate or mismatched native permission refused")
				}
				requestIDs[idText] = true
				if err := write(map[string]any{"jsonrpc": "2.0", "id": id, "result": map[string]any{"outcome": map[string]any{"outcome": "selected", "optionId": "allow_once"}}}); err != nil {
					return err
				}
				approved = true
				progress = 2
				f.provider.mu.Lock()
				f.provider.approved = true
				f.provider.mu.Unlock()
				continue
			}
			if _, exists := m["id"]; exists {
				_ = write(map[string]any{"jsonrpc": "2.0", "id": m["id"], "error": map[string]any{"code": -32601, "message": "unsupported research client request"}})
				return fmt.Errorf("unknown native ACP client operation refused")
			}
			if method != "session/update" || stage != 3 || session == "" || projectNativeRuntimeKeys(params, "sessionId update", "") != nil || params["sessionId"] != session {
				return fmt.Errorf("unknown native ACP notification/session")
			}
			update, ok := params["update"].(map[string]any)
			if !ok {
				return fmt.Errorf("ACP update object required")
			}
			kind, _ := update["sessionUpdate"].(string)
			switch kind {
			case "tool_call", "tool_call_update":
				if update["toolCallId"] != f.provider.callID {
					return fmt.Errorf("foreign native tool event")
				}
				if status := update["status"]; status == "pending" {
					if projectNativeRuntimeKeys(update, "sessionUpdate toolCallId title kind status rawInput", "") != nil || update["title"] != clineAttemptTitle(f.provider.arguments()) || update["kind"] != "other" || !reflect.DeepEqual(update["rawInput"], f.provider.arguments()) {
						return fmt.Errorf("native pending tool identity differs")
					}
					switch {
					case kind == "tool_call_update" && progress == 0:
						progress = 1
					case kind == "tool_call" && progress == 3:
						progress = 4
					default:
						return fmt.Errorf("native pending tool progress differs")
					}
				} else if status == "in_progress" {
					if !approved || progress != 2 || kind != "tool_call_update" || projectNativeRuntimeKeys(update, "sessionUpdate toolCallId status", "") != nil {
						return fmt.Errorf("native tool ran without exact approval")
					}
					progress = 3
				} else if status == "completed" {
					if !approved || completed || progress != 4 || kind != "tool_call_update" || projectNativeRuntimeKeys(update, "sessionUpdate toolCallId status rawOutput", "") != nil {
						return fmt.Errorf("native completed tool event differs")
					}
					body, _ := json.Marshal(update["rawOutput"])
					actual, err := clineAttemptMCPResult(body)
					if err != nil {
						return err
					}
					f.nativeFacts = actual
					completed = true
					progress = 5
				} else {
					return fmt.Errorf("unknown/failed native tool status")
				}
			case "agent_message_chunk":
				if progress != 5 || infoSeen || projectNativeRuntimeKeys(update, "sessionUpdate content", "") != nil {
					return fmt.Errorf("native message keys")
				}
				content, ok := update["content"].(map[string]any)
				if !ok || projectNativeRuntimeKeys(content, "type text", "") != nil || content["type"] != "text" || !projectNativeRuntimeText(content["text"], 16384) {
					return fmt.Errorf("native message grammar")
				}
				text += content["text"].(string)
				chunks++
				if len(text) > 16384 || chunks > 64 {
					return fmt.Errorf("native final text bound")
				}
			case "session_info_update":
				if progress != 5 || infoSeen || text != "UAP_CLINE_DONE_"+f.base.Nonce || projectNativeRuntimeKeys(update, "sessionUpdate updatedAt", "") != nil {
					return fmt.Errorf("native session info keys")
				}
				stamp, ok := update["updatedAt"].(string)
				if !ok {
					return fmt.Errorf("native session timestamp type")
				}
				if _, err := time.Parse(time.RFC3339Nano, stamp); err != nil {
					return err
				}
				infoSeen = true
			default:
				return fmt.Errorf("unmeasured native ACP update %q", kind)
			}
			continue
		}
		if projectNativeRuntimeKeys(m, "jsonrpc id result", "") != nil {
			return fmt.Errorf("native ACP error/unknown response: stage %d", stage)
		}
		if !projectNativeRuntimeNumber(m["id"], float64(stage), float64(stage), true) {
			return fmt.Errorf("native ACP response ID/order differs")
		}
		result, ok := m["result"].(map[string]any)
		if !ok {
			return fmt.Errorf("native ACP result object required")
		}
		switch stage {
		case 1:
			if projectNativeRuntimeKeys(result, "protocolVersion agentCapabilities agentInfo authMethods", "") != nil || !projectNativeRuntimeNumber(result["protocolVersion"], 1, 1, true) {
				return fmt.Errorf("unmeasured ACP initialize grammar")
			}
			caps := map[string]any{"loadSession": true, "promptCapabilities": map[string]any{"image": true, "audio": false, "embeddedContext": false}}
			auth := []any{map[string]any{"id": "cline", "name": "Sign in with Cline"}, map[string]any{"id": "cline-pass", "name": "Sign in with ClinePass"}, map[string]any{"id": "openai-codex", "name": "Sign in with ChatGPT Subscription"}}
			if !reflect.DeepEqual(result["agentCapabilities"], caps) || !reflect.DeepEqual(result["authMethods"], auth) {
				return fmt.Errorf("unmeasured ACP capabilities/auth method grammar")
			}
			info, ok := result["agentInfo"].(map[string]any)
			if !ok || projectNativeRuntimeKeys(info, "name version", "") != nil || info["version"] != "3.0.73" || info["name"] != "cline" {
				return fmt.Errorf("native ACP identity version differs")
			}
			stage = 2
			if err := send(2, "session/new", map[string]any{"cwd": f.base.Project, "mcpServers": []any{}}); err != nil {
				return err
			}
		case 2:
			if err := clineAttemptSession(result); err != nil {
				return err
			}
			session = result["sessionId"].(string)
			f.evidence["native_session_id"] = session
			stage = 3
			if err := send(3, "session/prompt", map[string]any{"sessionId": session, "prompt": []any{map[string]string{"type": "text", "text": f.provider.prompt + " Call only " + clineAttemptTool + " with the exact supplied harmless nonce/operation/marker. Do not call built-in tools or any other MCP server."}}}); err != nil {
				return err
			}
		case 3:
			if projectNativeRuntimeKeys(result, "stopReason", "") != nil || result["stopReason"] != "end_turn" || !approved || !completed || progress != 5 || !infoSeen || text != "UAP_CLINE_DONE_"+f.base.Nonce {
				return fmt.Errorf("complete native prompt/tool/result/sentinel join missing")
			}
			return nil
		default:
			return fmt.Errorf("repeated native response")
		}
	}
	if scan.Err() != nil {
		return scan.Err()
	}
	return fmt.Errorf("native ACP ended before full prompt result")
}
func clineAttemptSession(result map[string]any) error {
	if projectNativeRuntimeKeys(result, "sessionId modes models configOptions", "") != nil {
		return fmt.Errorf("unmeasured ACP session-new keys")
	}
	session, ok := result["sessionId"].(string)
	if !ok || !regexp.MustCompile(`^\d{13}_[A-Za-z0-9_-]{5}_cli$`).MatchString(session) {
		return fmt.Errorf("native generated ACP session identity grammar")
	}
	modes, ok := result["modes"].(map[string]any)
	if !ok || projectNativeRuntimeKeys(modes, "availableModes currentModeId", "") != nil || modes["currentModeId"] != "act" {
		return fmt.Errorf("native ACP mode differs")
	}
	wantModes := []any{map[string]any{"id": "plan", "name": "Plan", "description": "Explore the codebase and plan changes without modifying files"}, map[string]any{"id": "act", "name": "Act", "description": "Make changes to the codebase"}}
	if !reflect.DeepEqual(modes["availableModes"], wantModes) {
		return fmt.Errorf("unmeasured native available modes")
	}
	models, ok := result["models"].(map[string]any)
	if !ok || projectNativeRuntimeKeys(models, "availableModels currentModelId", "") != nil || models["currentModelId"] != clineAttemptModel || !reflect.DeepEqual(models["availableModels"], []any{map[string]any{"modelId": clineAttemptModel, "name": "UAP scripted model"}}) {
		return fmt.Errorf("native user-catalog model differs")
	}
	options, ok := result["configOptions"].([]any)
	if !ok || len(options) != 4 {
		return fmt.Errorf("native fixed ACP configuration options required")
	}
	seen := map[string]bool{}
	for _, v := range options {
		option, ok := v.(map[string]any)
		if !ok {
			return fmt.Errorf("ACP config object required")
		}
		id, _ := option["id"].(string)
		if seen[id] {
			return fmt.Errorf("duplicate ACP config")
		}
		seen[id] = true
		switch id {
		case "auto_approve":
			if !reflect.DeepEqual(option, map[string]any{"type": "boolean", "id": "auto_approve", "name": "Auto-approve tools", "description": "Automatically approve all tool calls without asking for permission", "currentValue": false}) {
				return fmt.Errorf("native autoapproval must remain false")
			}
		case "provider", "model", "mode":
			if projectNativeRuntimeKeys(option, "type id name category currentValue options", "description") != nil || option["type"] != "select" {
				return fmt.Errorf("native select configuration grammar")
			}
			want := map[string]string{"provider": clineAttemptProviderID, "model": clineAttemptModel, "mode": "act"}[id]
			if option["currentValue"] != want {
				return fmt.Errorf("native selected config differs")
			}
			if _, ok := option["options"].([]any); !ok {
				return fmt.Errorf("native select options type")
			}
		default:
			return fmt.Errorf("unknown native config option")
		}
	}
	return nil
}
