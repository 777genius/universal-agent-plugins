//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

const grokProjectProviderModel = "uap-grok-fixture"
const grokProjectProviderTransport = "scripted loopback OpenAI chat-completions SSE; no real model"

type grokProjectProviderReceipt struct {
	Transport         string                       `json:"transport"`
	Model             string                       `json:"model"`
	Started           bool                         `json:"started"`
	Requests          int                          `json:"requests"`
	Responses         int                          `json:"responses"`
	CredentialMatched bool                         `json:"credential_matched"`
	Failure           string                       `json:"failure"`
	ShutdownComplete  bool                         `json:"shutdown_complete"`
	ForcedClose       bool                         `json:"forced_close"`
	RouteObservation  *grokProjectRouteObservation `json:"route_observation,omitempty"`
}

type grokProjectProvider struct {
	mu             sync.Mutex
	ctx            context.Context
	cancel         context.CancelFunc
	totalEnd       time.Time
	key            string
	server         *http.Server
	listener       net.Listener
	serveDone      chan struct{}
	stateChanged   chan struct{}
	connections    int
	attempted      bool
	request, reply []byte
	authoredConfig []byte
	receipt        grokProjectProviderReceipt
	title          *grokProjectTitleProvider
	duplicate      *grokProjectDuplicateCapture
}

func newGrokProjectProvider(ctx context.Context, cancel context.CancelFunc, totalEnd time.Time) (*grokProjectProvider, error) {
	var secret [16]byte
	if _, err := rand.Read(secret[:]); err != nil {
		return nil, err
	}
	return &grokProjectProvider{ctx: ctx, cancel: cancel, totalEnd: totalEnd,
		key: "uap-grok-private-" + hex.EncodeToString(secret[:]), serveDone: make(chan struct{}), stateChanged: make(chan struct{}, 1),
		receipt: grokProjectProviderReceipt{Transport: grokProjectProviderTransport, Model: grokProjectProviderModel, Failure: "none"}}, nil
}

func (f *projectNativeCLIAttempt) configureGrokProvider(t *testing.T) {
	t.Helper()
	var err error
	f.grok, err = newGrokProjectProvider(f.ctx, f.cancel, f.started.Add(180*time.Second))
	if err != nil {
		t.Fatal("Grok fixture credential generation failed")
	}
	if err = f.grok.start(); err != nil {
		t.Fatal("Grok loopback listener could not start")
	}
	path := filepath.Join(f.base.Home, ".grok", "config.toml")
	configuration := f.grok.configuration()
	projectNativeRuntimeWrite(t, path, configuration, 0600)
	f.grok.authoredConfig = bytes.Clone(configuration)
	f.protected[path] = nativeSHA(t, path)
	f.evidence["provider_mode"] = "documented original-HOME custom provider; first opaque request observation; no vendor account"
}

func (p *grokProjectProvider) start() error {
	listener, err := (&net.ListenConfig{}).Listen(p.ctx, "tcp4", "127.0.0.1:0")
	if err != nil {
		p.fail("listen")
		return err
	}
	p.listener = listener
	p.server = &http.Server{Handler: http.HandlerFunc(p.serve), ReadHeaderTimeout: 2 * time.Second,
		ReadTimeout: 5 * time.Second, WriteTimeout: 5 * time.Second, IdleTimeout: 2 * time.Second, MaxHeaderBytes: 16 << 10,
		BaseContext: func(net.Listener) context.Context { return p.ctx }, ConnState: p.connectionState}
	p.receipt.Started = true
	go func() {
		defer close(p.serveDone)
		if err := p.server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			p.fail("serve")
		}
	}()
	return nil
}

func (p *grokProjectProvider) configuration() []byte {
	// This fresh original HOME has no legacy marketplace installs to purge.
	// Author the source-defined migration sentinel before freezing the full file.
	return []byte(fmt.Sprintf("[models]\ndefault = %q\n[model.%s]\nmodel = %q\nbase_url = %q\nenv_key = \"UAP_GROK_TEST_PROVIDER_KEY\"\napi_backend = \"chat_completions\"\nsupports_backend_search = false\nsupports_reasoning_effort = false\nmax_retries = 0\n\n[marketplace]\ndefault_skills_installs_purged = true\n", grokProjectProviderModel, grokProjectProviderModel, grokProjectProviderModel, "http://"+p.listener.Addr().String()+"/v1"))
}

func (p *grokProjectProvider) connectionState(_ net.Conn, state http.ConnState) {
	p.mu.Lock()
	switch state {
	case http.StateNew:
		p.connections++
	case http.StateClosed, http.StateHijacked:
		p.connections--
	}
	p.mu.Unlock()
	select {
	case p.stateChanged <- struct{}{}:
	default:
	}
}

func (p *grokProjectProvider) fail(reason string) {
	p.failWithRequest(reason, nil)
}

func grokProjectProviderRoute(r *http.Request, host string) bool {
	peer, _, err := net.SplitHostPort(r.RemoteAddr)
	return err == nil && peer == "127.0.0.1" && r.Method == http.MethodPost && r.Host == host &&
		r.RequestURI == "/v1/chat/completions" && !r.URL.IsAbs() && r.URL.Host == "" && r.URL.RawPath == "" &&
		r.Header.Get("Content-Type") == "application/json" && len(r.Header.Values("Content-Type")) == 1
}

func (p *grokProjectProvider) admit(w http.ResponseWriter, r *http.Request) bool {
	reason := "none"
	switch {
	case p.ctx.Err() != nil:
		reason = "deadline"
	case !grokProjectProviderRoute(r, p.listener.Addr().String()):
		reason = "route"
	case len(r.Header.Values("Authorization")) != 1 || r.Header.Get("Authorization") != "Bearer "+p.key:
		reason = "credential"
	}
	p.mu.Lock()
	var duplicate *grokProjectDuplicateCapture
	if reason == "none" && p.attempted {
		reason = "duplicate"
		duplicate = p.claimDuplicate()
	}
	if reason == "none" {
		p.attempted, p.receipt.CredentialMatched = true, true
	}
	p.mu.Unlock()
	if duplicate != nil {
		p.observeDuplicate(w, r, duplicate)
		return false
	}
	if reason != "none" {
		http.Error(w, "closed Grok provider request refused", http.StatusBadRequest)
		p.failWithRequest(reason, r)
		return false
	}
	return true
}

func (p *grokProjectProvider) captureRequest(raw []byte) bool {
	value, err := projectNativeRuntimeJSON(raw)
	object, ok := value.(map[string]any)
	if err != nil || !ok {
		return false
	}
	p.mu.Lock()
	p.request, p.receipt.Requests = bytes.Clone(raw), 1
	p.mu.Unlock()
	messages, ok := object["messages"].([]any)
	return object["model"] == grokProjectProviderModel && object["stream"] == true && ok && len(messages) > 0 && len(messages) <= 128
}

func grokProjectProviderResponse() []byte {
	return []byte("data: {\"id\":\"uap-grok-capture\",\"object\":\"chat.completion.chunk\",\"created\":1,\"model\":\"uap-grok-fixture\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"UAP_GROK_PROVIDER_CAPTURE\"},\"finish_reason\":null}]}\n\ndata: {\"id\":\"uap-grok-capture\",\"object\":\"chat.completion.chunk\",\"created\":1,\"model\":\"uap-grok-fixture\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")
}

func (p *grokProjectProvider) serve(w http.ResponseWriter, r *http.Request) {
	if !p.admit(w, r) {
		return
	}
	raw, err := io.ReadAll(io.LimitReader(r.Body, (1<<20)+1))
	if err != nil || len(raw) > 1<<20 {
		http.Error(w, "Grok provider body bound", http.StatusBadRequest)
		p.fail("body")
		return
	}
	if !p.captureRequest(raw) {
		http.Error(w, "Grok provider request shape refused", http.StatusBadRequest)
		p.fail("request-shape")
		return
	}
	response := grokProjectProviderResponse()
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	if count, err := w.Write(response); err != nil || count != len(response) {
		p.fail("response-write")
		return
	}
	p.mu.Lock()
	p.reply, p.receipt.Responses = bytes.Clone(response), 1
	p.mu.Unlock()
}

func (p *grokProjectProvider) stop() bool {
	if p.server == nil {
		return false
	}
	p.mu.Lock()
	closed, ordinary := p.receipt.ShutdownComplete, !p.receipt.ForcedClose
	p.mu.Unlock()
	if closed {
		return ordinary
	}
	end := time.Now().Add(2 * time.Second)
	if end.After(p.totalEnd) {
		end = p.totalEnd
	}
	cleanup, cancel := context.WithDeadline(context.Background(), end)
	defer cancel()
	grace, stopGrace := context.WithTimeout(p.ctx, time.Second)
	err := p.server.Shutdown(grace)
	stopGrace()
	if err != nil {
		p.mu.Lock()
		p.receipt.ForcedClose = true
		p.mu.Unlock()
		p.fail("cleanup")
		if closeErr := p.server.Close(); closeErr != nil {
			p.fail("cleanup")
		}
	}
	select {
	case <-p.serveDone:
	case <-cleanup.Done():
		p.fail("cleanup")
		return false
	}
	for {
		p.mu.Lock()
		closed := p.connections == 0
		p.receipt.ShutdownComplete = closed
		ordinary := closed && !p.receipt.ForcedClose
		p.mu.Unlock()
		if closed {
			return ordinary
		}
		select {
		case <-p.stateChanged:
		case <-cleanup.Done():
			p.fail("cleanup")
			return false
		}
	}
}

func (p *grokProjectProvider) redact(body []byte) []byte {
	body = bytes.ReplaceAll(body, []byte(p.key), []byte("[redacted-grok-fixture-key]"))
	if p.title != nil {
		body = bytes.ReplaceAll(body, []byte(p.title.transport.key), []byte("[redacted-grok-fixture-key]"))
	}
	return body
}

func (p *grokProjectProvider) finish(t *testing.T, f *projectNativeCLIAttempt) {
	t.Helper()
	if p.title != nil {
		p.title.finish(t, f)
	}
	if !p.stop() {
		t.Error("Grok test provider ordinary shutdown did not complete")
	}
	defer p.finishDuplicate(t, f)
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.request) != 0 {
		f.keep(t, "grok-provider.request.json", p.request)
	}
	if len(p.reply) != 0 {
		f.keep(t, "grok-provider.response.sse", p.reply)
	}
	raw, err := json.Marshal(p.receipt)
	if err != nil || len(raw) > 2<<10 {
		t.Error("Grok provider finite receipt refused")
		return
	}
	f.keep(t, "grok-provider.json", raw)
}
