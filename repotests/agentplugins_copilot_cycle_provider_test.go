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
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

type copilotCycleProvider struct {
	mu                 sync.Mutex
	p                  *copilotProjectProvider
	operation, url     string
	server             *http.Server
	listener           net.Listener
	done               chan error
	connections        map[net.Conn]bool
	connectionsChanged chan struct{}
	responses          [][]byte
	closed             bool
}

func newCopilotCycleProvider(nonce, stage, tool, operation string, validate func(string) error) *copilotCycleProvider {
	p := newCopilotProjectProvider(tool, nonce, validate)
	p.id, p.sentinel = "uap_"+nonce+"_"+stage, "UAP_DONE_"+nonce+"_"+stage
	return &copilotCycleProvider{p: p, operation: operation, done: make(chan error, 1), connections: map[net.Conn]bool{}, connectionsChanged: make(chan struct{}, 1)}
}

func (p *copilotCycleProvider) start(ctx context.Context) error {
	listener, err := (&net.ListenConfig{}).Listen(ctx, "tcp4", "127.0.0.1:0")
	if err != nil {
		return err
	}
	p.listener, p.url = listener, "http://"+listener.Addr().String()
	p.server = &http.Server{Handler: http.HandlerFunc(p.serve), ReadHeaderTimeout: time.Second, ReadTimeout: 5 * time.Second, WriteTimeout: 5 * time.Second, IdleTimeout: time.Second,
		ConnState: func(conn net.Conn, state http.ConnState) {
			p.mu.Lock()
			defer p.mu.Unlock()
			if state == http.StateClosed || state == http.StateHijacked {
				delete(p.connections, conn)
			} else {
				p.connections[conn] = true
			}
			select {
			case p.connectionsChanged <- struct{}{}:
			default:
			}
		}}
	go func() { p.done <- p.server.Serve(listener) }()
	return nil
}

func copilotCycleRequest(body []byte, tool string) (map[string]any, error) {
	if tool != "" {
		return copilotProjectRequest(body, tool)
	}
	m, err := copilotProjectEnvelope(body, "")
	if err != nil {
		return nil, err
	}
	if tools, exists := m["tools"]; exists && !reflect.DeepEqual(tools, []any{}) {
		return nil, fmt.Errorf("Copilot no-call provider has tool declarations")
	}
	if choice, exists := m["tool_choice"]; exists && choice != "none" && choice != "auto" {
		return nil, fmt.Errorf("Copilot no-call tool choice")
	}
	return m, nil
}

func (p *copilotCycleProvider) serve(w http.ResponseWriter, r *http.Request) {
	p.mu.Lock()
	defer p.mu.Unlock()
	fail := func(err error) {
		if p.p.err == nil {
			p.p.err = err
		}
		http.Error(w, "scripted provider rejected request", http.StatusBadRequest)
	}
	limit := 2
	if p.p.tool == "" {
		limit = 1
	}
	if len(p.p.requests) >= limit || r.Method != "POST" || r.URL.RequestURI() != "/v1/chat/completions" || r.Host != strings.TrimPrefix(p.url, "http://") || r.Header.Get("Authorization") != "" || r.Header.Get("api-key") != "" || strings.Split(r.Header.Get("Content-Type"), ";")[0] != "application/json" {
		fail(fmt.Errorf("Copilot cycle provider route/credential/count"))
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, copilotProjectBodyLimit+1))
	if err != nil || len(body) > copilotProjectBodyLimit || copilotCyclePrivate(body) {
		fail(fmt.Errorf("Copilot cycle provider body/privacy bound"))
		return
	}
	p.p.requests = append(p.p.requests, bytes.Clone(body))
	m, err := copilotCycleRequest(body, p.p.tool)
	if err != nil {
		fail(err)
		return
	}
	recorder := httptest.NewRecorder()
	if err = p.respond(recorder, m); err != nil {
		fail(err)
		return
	}
	_, _ = io.WriteString(recorder, "data: [DONE]\n\n")
	response := bytes.Clone(recorder.Body.Bytes())
	if copilotCyclePrivate(response) {
		fail(fmt.Errorf("Copilot cycle provider response privacy"))
		return
	}
	p.responses = append(p.responses, response)
	w.Header().Set("Content-Type", "text/event-stream")
	_, _ = w.Write(response)
}

func (p *copilotCycleProvider) arguments() map[string]any {
	return map[string]any{"nonce": p.p.nonce, "operation": p.operation, "marker": p.p.marker}
}

func (p *copilotCycleProvider) respond(w http.ResponseWriter, m map[string]any) error {
	messages := m["messages"].([]any)
	if len(p.p.requests) == 1 {
		for _, value := range messages {
			row := value.(map[string]any)
			if row["role"] == "tool" || row["tool_calls"] != nil {
				return fmt.Errorf("Copilot cycle tool history before call")
			}
		}
		if p.p.tool == "" {
			p.p.returned = true
			p.p.chunk(w, map[string]any{"role": "assistant", "content": p.p.sentinel}, nil)
			p.p.chunk(w, map[string]any{}, "stop")
			return nil
		}
		args, _ := json.Marshal(p.arguments())
		p.p.chunk(w, map[string]any{"role": "assistant", "tool_calls": []any{map[string]any{"index": 0, "id": p.p.id, "type": "function", "function": map[string]any{"name": p.p.tool, "arguments": string(args)}}}}, nil)
		p.p.chunk(w, map[string]any{}, "tool_calls")
		return nil
	}
	if len(messages) < 2 {
		return fmt.Errorf("Copilot cycle missing returned history")
	}
	args, _ := json.Marshal(p.arguments())
	want := map[string]any{"role": "assistant", "content": nil, "refusal": nil, "tool_calls": []any{map[string]any{"id": p.p.id, "type": "function", "function": map[string]any{"name": p.p.tool, "arguments": string(args)}}}}
	last := messages[len(messages)-1].(map[string]any)
	if !reflect.DeepEqual(messages[len(messages)-2], want) || last["role"] != "tool" || last["tool_call_id"] != p.p.id || !projectNativeRuntimeText(last["content"], copilotProjectBodyLimit) {
		return fmt.Errorf("Copilot cycle returned call identity")
	}
	if err := p.p.validate(last["content"].(string)); err != nil {
		return err
	}
	p.p.returned = true
	p.p.chunk(w, map[string]any{"role": "assistant", "content": p.p.sentinel}, nil)
	p.p.chunk(w, map[string]any{}, "stop")
	return nil
}

func (p *copilotCycleProvider) shutdown(t *testing.T, c *copilotProjectCycle, stage string) {
	t.Helper()
	if p.closed {
		return
	}
	p.closed = true
	ctx, cancel := context.WithTimeout(c.f.ctx, 5*time.Second)
	defer cancel()
	err := p.server.Shutdown(ctx)
	serveClosed := false
	select {
	case serveErr := <-p.done:
		serveClosed = errors.Is(serveErr, http.ErrServerClosed)
	case <-ctx.Done():
	}
	connectionsClosed := p.connectionsQuiet(ctx)
	forced := err != nil || !serveClosed || !connectionsClosed
	if forced {
		_ = p.server.Close()
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	for i, body := range p.p.requests {
		c.append(t, "cycle-provider-requests.jsonl", stage, map[string]any{"index": i + 1, "body": copilotCycleStream(t, body)})
	}
	for i, body := range p.responses {
		c.append(t, "cycle-provider-responses.jsonl", stage, map[string]any{"index": i + 1, "body": copilotCycleStream(t, body)})
	}
	c.append(t, "cycle-provider-receipts.jsonl", stage, map[string]any{"requests": len(p.p.requests), "responses": len(p.responses), "error": p.p.err != nil, "returned": p.p.returned, "shutdown": err == nil, "serve_closed": serveClosed, "connections_closed": connectionsClosed, "forced": forced})
	limit := 2
	if p.p.tool == "" {
		limit = 1
	}
	if forced || p.p.err != nil || !p.p.returned || len(p.p.requests) != limit || len(p.responses) != limit {
		t.Error("Copilot cycle provider incomplete or ordinary closure failed")
	}
}

func (p *copilotCycleProvider) connectionsQuiet(ctx context.Context) bool {
	for {
		p.mu.Lock()
		quiet := len(p.connections) == 0
		p.mu.Unlock()
		if quiet {
			return true
		}
		select {
		case <-p.connectionsChanged:
		case <-ctx.Done():
			return false
		}
	}
}
