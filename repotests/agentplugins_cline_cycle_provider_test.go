//go:build linux

package pluginkitairepo_test

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"
)

type clineCycleProvider struct {
	mu        sync.Mutex
	listener  net.Listener
	server    *http.Server
	done      chan struct{}
	idle      chan struct{}
	current   *clineProjectProvider
	stage     string
	completed []string
	busy      int
	failure   bool
	cancel    context.CancelFunc
	key       string
}

func (c *clineProjectCycle) startProvider(t *testing.T) {
	t.Helper()
	l, err := (&net.ListenConfig{}).Listen(c.f.ctx, "tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	p := &clineCycleProvider{listener: l, done: make(chan struct{}), idle: make(chan struct{}, 1), cancel: c.f.cancel, key: c.f.key}
	c.owner = p
	p.server = &http.Server{Handler: http.HandlerFunc(p.serve), ReadHeaderTimeout: 3 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 3 * time.Second, MaxHeaderBytes: 16 << 10, BaseContext: func(net.Listener) context.Context { return c.f.ctx }}
	go func() {
		defer close(p.done)
		if err := p.server.Serve(l); err != nil && !errors.Is(err, http.ErrServerClosed) {
			p.fail()
		}
	}()
	// Configure once in the original user HOME, before any native command.
	(&clineProjectProvider{url: "http://" + l.Addr().String() + "/v1", key: c.f.key}).configure(t, c.f)
}
func (p *clineCycleProvider) fail() {
	p.mu.Lock()
	p.failure = true
	p.mu.Unlock()
	p.cancel()
}
func (p *clineCycleProvider) serve(w http.ResponseWriter, r *http.Request) {
	peer, _, err := net.SplitHostPort(r.RemoteAddr)
	p.mu.Lock()
	current := p.current
	if err != nil || peer != "127.0.0.1" || current == nil || p.failure || p.busy != 0 {
		p.mu.Unlock()
		http.Error(w, "unarmed Cline cycle request refused", 400)
		p.fail()
		return
	}
	p.busy++
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
	current.serve(w, r)
}
func (p *clineCycleProvider) arm(stage string, r *clineCycleRecipe) (*clineProjectProvider, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	index := len(p.completed)
	if p.failure || p.busy != 0 || p.current != nil || index >= len(clineCycleACP) || clineCycleACP[index] != stage || r == nil || clineCycleScope(stage) != r.Scope {
		return nil, fmt.Errorf("Cline cycle provider arm/order/previous-handler boundary")
	}
	operation := "empty"
	switch stage {
	case "installed-a", "installed-b":
		operation = "write"
	case "updated-a", "repaired-a", "remaining-b":
		operation = "read"
	}
	nonce := r.Nonce
	q := &clineProjectProvider{listener: p.listener, key: p.key, nonce: nonce, callID: "uap-cline-cycle-" + clineAttemptSHA([]byte(stage + nonce))[:24], marker: "retained-" + nonce, prompt: "UAP_CLINE_CYCLE_" + stage + "_" + nonce, sentinel: "UAP_CLINE_DONE_" + stage + "_" + nonce, operation: operation, empty: operation == "empty", cancel: p.cancel}
	p.current, p.stage = q, stage
	return q, nil
}
func (p *clineCycleProvider) complete(t *testing.T, c *clineProjectCycle, stage string) {
	t.Helper()
	if err := p.waitIdle(c.f.ctx); err != nil {
		p.fail()
		t.Fatal(err)
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	q := p.current
	if q == nil || p.stage != stage || p.busy != 0 || p.failure {
		t.Fatal("Cline cycle handler did not close before stage completion")
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	want := 2
	if q.empty {
		want = 1
	}
	if q.failure != nil || len(q.requests) != want || len(q.responses) != want || q.issued == q.empty || q.approved == q.empty {
		t.Fatal("Cline cycle exact provider stage closure")
	}
	clineCycleProviderRecord(t, c, stage, q)
	p.current = nil
	p.stage = ""
	p.completed = append(p.completed, stage)
}
func (p *clineCycleProvider) waitIdle(ctx context.Context) error {
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
			return fmt.Errorf("Cline previous provider handler did not close in original work budget")
		}
	}
}
func clineCycleProviderRecord(t *testing.T, c *clineProjectCycle, stage string, q *clineProjectProvider) {
	t.Helper()
	for index, raw := range q.requests {
		c.append(t, "cycle-provider-requests.jsonl", stage, map[string]any{"index": index, "stream": c.stream(t, raw)})
	}
	for index, raw := range q.responses {
		c.append(t, "cycle-provider-responses.jsonl", stage, map[string]any{"index": index, "stream": c.stream(t, raw)})
	}
	c.append(t, "cycle-provider-receipts.jsonl", stage, map[string]any{"requests": len(q.requests), "responses": len(q.responses), "empty": q.empty, "exact_call_issued": q.issued, "single_call_approved": q.approved, "error": q.failure != nil, "call_id": q.callID, "prompt": q.prompt, "sentinel": q.doneText(), "operation": q.operation})
}
func (p *clineCycleProvider) close(t *testing.T, c *clineProjectCycle) {
	t.Helper()
	remaining := time.Duration(c.f.totalEnd - clineAttemptMonotonic())
	if remaining < 0 {
		remaining = 0
	}
	ctx, cancel := context.WithTimeout(context.Background(), min(time.Second, remaining))
	err := p.server.Shutdown(ctx)
	cancel()
	if err != nil {
		if closeErr := p.server.Close(); closeErr != nil {
			t.Error("Cline cycle provider forced close failed")
		}
		t.Error("Cline cycle provider ordinary shutdown failed")
	}
	remaining = time.Duration(c.f.totalEnd - clineAttemptMonotonic())
	if remaining < 0 {
		remaining = 0
	}
	ctx, cancel = context.WithTimeout(context.Background(), min(time.Second, remaining))
	defer cancel()
	select {
	case <-p.done:
	case <-ctx.Done():
		t.Error("Cline cycle provider serve did not finish")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.busy != 0 || p.failure {
		t.Error("Cline cycle provider outstanding handler/failure")
	}
	if p.current != nil {
		p.current.mu.Lock()
		clineCycleProviderRecord(t, c, p.stage, p.current)
		p.current.mu.Unlock()
	}
	c.f.evidence["cycle_provider_cleanup"] = map[string]any{"shutdown_complete": err == nil && p.busy == 0, "forced_close": err != nil, "error": p.failure, "completed_stages": p.completed}
}
