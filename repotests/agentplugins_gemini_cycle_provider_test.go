//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

type geminiCyclePhase struct {
	mu                                                          sync.Mutex
	stage, scope, operation, tool, id, wireID, prompt, sentinel string
	r                                                           *geminiCycleRecipe
	tools                                                       []string
	requests, responses                                         [][]byte
	initialContents                                             []any
	initialEnvelope                                             map[string]any
	err                                                         error
	issued, returned, recorded                                  bool
	facts                                                       nativeFacts
}

func (q *geminiCyclePhase) arguments() map[string]any {
	if q.operation == "none" {
		return map[string]any{}
	}
	return map[string]any{"nonce": q.r.Nonce, "operation": q.operation, "marker": "retained-" + q.r.Nonce}
}

type geminiCycleProvider struct {
	mu          sync.Mutex
	listener    net.Listener
	server      *http.Server
	done        chan struct{}
	idle        chan struct{}
	c           *geminiProjectCycle
	current     *geminiCyclePhase
	completed   []string
	busy, total int
	failure     bool
}

func (c *geminiProjectCycle) startProvider(t *testing.T) {
	t.Helper()
	listener, err := (&net.ListenConfig{}).Listen(c.f.ctx, "tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	p := &geminiCycleProvider{listener: listener, done: make(chan struct{}), idle: make(chan struct{}, 1), c: c, completed: []string{}}
	c.owner = p
	p.server = &http.Server{Handler: http.HandlerFunc(p.serve), ReadHeaderTimeout: time.Second, ReadTimeout: 5 * time.Second, WriteTimeout: 5 * time.Second, IdleTimeout: time.Second, MaxHeaderBytes: 16 << 10, BaseContext: func(net.Listener) context.Context { return c.f.ctx }}
	go func() {
		defer close(p.done)
		if err := p.server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			p.mu.Lock()
			p.failure = true
			p.mu.Unlock()
			c.f.cancel()
		}
	}()
}
func (p *geminiCycleProvider) arm(stage, scope string) (*geminiCyclePhase, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	c := p.c
	if p.current != nil || p.failure || p.busy != 0 || len(p.completed) >= len(geminiCycleNative) || geminiCycleNative[len(p.completed)] != stage || geminiCycleScope(stage) != scope || c.f.ctx.Err() != nil {
		return nil, fmt.Errorf("Gemini cycle provider finite arm/closure")
	}
	r := c.recipes[scope]
	if r == nil {
		return nil, fmt.Errorf("Gemini cycle provider held recipe")
	}
	operation := "none"
	switch stage {
	case "installed-user", "installed-a", "installed-b":
		operation = "write"
	case "restarted-a", "updated-a", "repaired-a":
		operation = "read"
	}
	q := &geminiCyclePhase{stage: stage, scope: scope, operation: operation, r: r, tools: []string{}, prompt: "Disposable Gemini fixture stage " + stage + ". Call only the preselected installed inspect_runtime tool when requested.", sentinel: "UAP_GEMINI_DONE_" + stage + "_" + r.Nonce}
	for _, key := range []string{"user", scope} {
		other := c.recipes[key]
		if other.Binding.ClientBindingID != "" && !other.Removed {
			tool := "mcp_" + other.Name + "_inspect_runtime"
			if len(q.tools) == 0 || q.tools[len(q.tools)-1] != tool {
				q.tools = append(q.tools, tool)
			}
		}
	}
	if operation != "none" {
		if r.Removed || r.Binding.ClientBindingID == "" {
			return nil, fmt.Errorf("Gemini cycle selected caller is not installed")
		}
		q.tool = "mcp_" + r.Name + "_inspect_runtime"
		q.wireID = "uap_cycle_" + geminiCycleSHA([]byte(stage + r.Nonce))[:24]
		q.id = q.tool + "__" + q.wireID
	}
	p.current = q
	return q, nil
}
func (p *geminiCycleProvider) serve(w http.ResponseWriter, r *http.Request) {
	peer, _, peerErr := net.SplitHostPort(r.RemoteAddr)
	p.mu.Lock()
	q := p.current
	if q == nil || p.failure || p.busy != 0 || p.total >= 16 || peerErr != nil || peer != "127.0.0.1" || p.c.f.ctx.Err() != nil {
		p.failure = true
		p.mu.Unlock()
		p.c.f.cancel()
		http.Error(w, "Gemini cycle unarmed request refused", 400)
		return
	}
	p.busy++
	p.total++
	p.mu.Unlock()
	defer func() {
		p.mu.Lock()
		p.busy--
		p.mu.Unlock()
		select {
		case p.idle <- struct{}{}:
		default:
		}
	}()
	q.mu.Lock()
	defer q.mu.Unlock()
	fail := func() {
		if q.err == nil {
			q.err = fmt.Errorf("Gemini cycle provider request/response refused")
		}
		p.c.f.cancel()
		http.Error(w, "Gemini cycle provider refused", 400)
	}
	if r.Method != "POST" || r.URL.RequestURI() != "/v1beta/models/"+geminiProjectModel+":streamGenerateContent?alt=sse" || r.Host != p.listener.Addr().String() || r.Header.Get("x-goog-api-key") != p.c.f.secret || r.Header.Get("Authorization") != "" || strings.Split(r.Header.Get("Content-Type"), ";")[0] != "application/json" {
		fail()
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, (1<<20)+1))
	if err != nil || len(body) > 1<<20 {
		fail()
		return
	}
	body = bytes.ReplaceAll(body, []byte(p.c.f.secret), []byte("[redacted]"))
	q.requests = append(q.requests, body)
	want := 2
	if q.operation == "none" {
		want = 1
	}
	if len(q.requests) > want {
		fail()
		return
	}
	parts, err := geminiCycleRequest(body, q)
	if err != nil {
		fail()
		return
	}
	inner, err := geminiCycleHistory(parts, q, len(q.requests)-1)
	if err != nil {
		fail()
		return
	}
	part := map[string]any{"text": q.sentinel}
	if q.operation != "none" && len(q.requests) == 1 {
		q.issued = true
		part = map[string]any{"functionCall": map[string]any{"id": q.id, "name": q.tool, "args": q.arguments()}}
	} else if q.operation != "none" {
		facts, err := geminiCycleFacts(inner, q.r, p.c.f.base.Home)
		if err != nil {
			fail()
			return
		}
		q.facts = facts
		q.returned = true
	}
	encoded, _ := json.Marshal(map[string]any{"candidates": []any{map[string]any{"index": 0, "content": map[string]any{"role": "model", "parts": []any{part}}, "finishReason": "STOP"}}, "usageMetadata": map[string]int{"promptTokenCount": 1, "candidatesTokenCount": 1, "totalTokenCount": 2}})
	response := append(append([]byte("data: "), encoded...), []byte("\n\n")...)
	q.responses = append(q.responses, response)
	w.Header().Set("Content-Type", "text/event-stream")
	if _, err := w.Write(response); err != nil {
		q.err = fmt.Errorf("Gemini cycle response write failed")
		p.c.f.cancel()
	}
}
func (p *geminiCycleProvider) waitIdle(ctx context.Context) error {
	for {
		p.mu.Lock()
		idle := p.busy == 0
		p.mu.Unlock()
		if idle {
			return nil
		}
		select {
		case <-p.idle:
		case <-ctx.Done():
			return fmt.Errorf("Gemini cycle handler original deadline")
		}
	}
}
func (p *geminiCycleProvider) complete(t *testing.T, stage string) {
	t.Helper()
	if err := p.waitIdle(p.c.f.ctx); err != nil {
		t.Fatal(err)
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	q := p.current
	if p.failure || q == nil || q.stage != stage || p.busy != 0 {
		t.Fatal("Gemini cycle provider completion ownership")
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	want := 2
	if q.operation == "none" {
		want = 1
	}
	if q.err != nil || len(q.requests) != want || len(q.responses) != want || q.issued != (q.operation != "none") || q.returned != (q.operation != "none") {
		t.Fatal("Gemini exact native-driven provider stage incomplete")
	}
	p.record(t, q)
	p.completed = append(p.completed, stage)
	p.current = nil
}
func (p *geminiCycleProvider) record(t *testing.T, q *geminiCyclePhase) {
	t.Helper()
	if q.recorded {
		return
	}
	q.recorded = true
	for index, body := range q.requests {
		p.c.append(t, "cycle-provider-requests.jsonl", q.stage, map[string]any{"index": index, "stream": p.c.stream(t, body)})
	}
	for index, body := range q.responses {
		p.c.append(t, "cycle-provider-responses.jsonl", q.stage, map[string]any{"index": index, "stream": p.c.stream(t, body)})
	}
	p.c.append(t, "cycle-provider-receipts.jsonl", q.stage, map[string]any{"scope": q.scope, "operation": q.operation, "tools": q.tools, "native_call_id": q.id, "wire_call_id": q.wireID, "prompt": q.prompt, "sentinel": q.sentinel, "requests": len(q.requests), "responses": len(q.responses), "issued": q.issued, "returned": q.returned, "error": q.err != nil})
}
func (p *geminiCycleProvider) close(t *testing.T) bool {
	t.Helper()
	remaining := time.Until(p.c.f.started.Add(180 * time.Second))
	if remaining <= 0 {
		_ = p.server.Close()
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), remaining)
	defer cancel()
	clean := p.server.Shutdown(ctx) == nil
	if !clean {
		_ = p.server.Close()
	}
	select {
	case <-p.done:
	case <-ctx.Done():
		clean = false
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.current != nil {
		p.current.mu.Lock()
		p.record(t, p.current)
		p.current.mu.Unlock()
	}
	p.c.f.evidence["cycle_provider_completed"] = append([]string{}, p.completed...)
	return clean && !p.failure && p.busy == 0 && p.current == nil && reflect.DeepEqual(p.completed, geminiCycleNative) && p.total == 16
}
