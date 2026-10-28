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
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

const grokProjectTitleModel = "uap-grok-title-fixture"
const grokProjectTitleTransport = "scripted loopback OpenAI session-title SSE; no real model"
const grokProjectTitlePrompt = "Describe only the installed uap-grok-attempt skill marker. Do not run any tool or change any file."
const grokProjectTitleSystem = `You are tasked with generating the session title. The user is asking almost always software engineering related questions on their codebase.
We describe the session title below
# Session Title
A short and distinctive 5-10 word descriptive title for the session. Super info dense, no filler.

You will be given the user query below encapsulated in <user_query></user_query>.

Just generate the session_title and nothing else`

// The source-defined auxiliary title has a separate endpoint, key and one-call
// counter. It never changes the original primary provider's model admission.
type grokProjectTitleProvider struct {
	transport *grokProjectProvider
	mainKey   string
}

func (f *projectNativeCLIAttempt) configureGrokTitleProvider(t *testing.T) {
	t.Helper()
	if f.spec.client != "grok" {
		t.Fatal("fixed Grok title fixture requires Grok")
	}
	var err error
	f.grok, err = newGrokProjectProvider(f.ctx, f.cancel, f.started.Add(180*time.Second))
	if err != nil {
		t.Fatal("Grok main fixture credential generation failed")
	}
	if err = f.grok.start(); err != nil {
		t.Fatal("Grok main fixture listener could not start")
	}
	transport, err := newGrokProjectProvider(f.ctx, f.cancel, f.started.Add(180*time.Second))
	if err != nil {
		t.Fatal("Grok title fixture credential generation failed")
	}
	if transport.key == f.grok.key {
		t.Fatal("Grok title and main fixture keys must differ")
	}
	f.grok.title = &grokProjectTitleProvider{transport: transport, mainKey: f.grok.key}
	transport.receipt.Transport, transport.receipt.Model = grokProjectTitleTransport, grokProjectTitleModel
	if err = f.grok.title.start(); err != nil {
		t.Fatal("Grok title fixture listener could not start")
	}
	trust, err := grokProjectSelectionTrust(f)
	if err != nil {
		t.Fatal(err)
	}
	trustPath := filepath.Join(f.base.Home, ".grok", "trusted_folders.toml")
	projectNativeRuntimeWrite(t, trustPath, trust, 0600)
	f.protected[trustPath] = nativeSHA(t, trustPath)
	// Both held endpoints and the supported name selector precede the only freeze.
	configuration := append(f.grok.title.configuration(f.grok.configuration()), []byte("\n[plugins]\nenabled = [\"uap-grok-attempt\"]\n\n[features]\nturn_summary = false\n")...)
	path := filepath.Join(f.base.Home, ".grok", "config.toml")
	projectNativeRuntimeWrite(t, path, configuration, 0600)
	f.grok.authoredConfig = bytes.Clone(configuration)
	f.protected[path] = nativeSHA(t, path)
	f.evidence["provider_mode"] = "documented original-HOME custom main/title fixtures; bounded startup capture; no vendor account"
}

func (p *grokProjectTitleProvider) configuration(primary []byte) []byte {
	model := []byte(fmt.Sprintf("session_summary = %q\n", grokProjectTitleModel))
	configuration := bytes.Replace(primary, []byte("[models]\n"), append([]byte("[models]\n"), model...), 1)
	return append(configuration, []byte(fmt.Sprintf("\n[model.%s]\nmodel = %q\nbase_url = %q\nenv_key = \"UAP_GROK_TITLE_PROVIDER_KEY\"\napi_backend = \"chat_completions\"\nsupports_backend_search = false\nsupports_reasoning_effort = false\nmax_retries = 0\n", grokProjectTitleModel, grokProjectTitleModel, "http://"+p.transport.listener.Addr().String()+"/v1"))...)
}

func (p *grokProjectTitleProvider) start() error {
	t := p.transport
	listener, err := (&net.ListenConfig{}).Listen(t.ctx, "tcp4", "127.0.0.1:0")
	if err != nil {
		t.fail("listen")
		return err
	}
	t.listener = listener
	t.server = &http.Server{Handler: http.HandlerFunc(p.serve), ReadHeaderTimeout: 2 * time.Second,
		ReadTimeout: 5 * time.Second, WriteTimeout: 5 * time.Second, IdleTimeout: 2 * time.Second, MaxHeaderBytes: 16 << 10,
		BaseContext: func(net.Listener) context.Context { return t.ctx }, ConnState: t.connectionState}
	t.receipt.Started = true
	go func() {
		defer close(t.serveDone)
		if err := t.server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			t.fail("serve")
		}
	}()
	return nil
}

func grokProjectTitleExpected() map[string]any {
	return map[string]any{
		"model": grokProjectTitleModel, "stream": true, "stream_options": map[string]any{"include_usage": true},
		"temperature": json.Number("1.0"), "max_tokens": json.Number("100"),
		"messages": []any{map[string]any{"role": "system", "content": grokProjectTitleSystem},
			map[string]any{"role": "user", "content": "<user_query>\n" + grokProjectTitlePrompt + "\n</user_query>"}},
		"tools": []any{map[string]any{"type": "function", "function": map[string]any{
			"name": "session_title", "description": "Generate the session_title which we use for the user_message",
			"parameters": map[string]any{"type": "object", "required": []any{"session_title"}, "additionalProperties": false,
				"properties": map[string]any{"session_title": map[string]any{"type": "string", "description": "Final session title, just 5-10 word descriptive title for the session. Super info dense, no filler."}}}}}},
		"tool_choice": map[string]any{"type": "function", "function": map[string]any{"name": "session_title"}},
	}
}

func (p *grokProjectTitleProvider) capture(raw []byte) string {
	value, err := projectNativeRuntimeJSON(raw)
	object, ok := value.(map[string]any)
	if err != nil || !ok {
		return "request-shape"
	}
	// Refuse both literal and decoded credentials before retaining an opaque
	// shape refusal. The existing bounded scanner also catches nested encodings.
	decoded, err := json.Marshal(object)
	if err != nil || grokProjectPrivateCredential(raw) || grokProjectPrivateCredential(decoded) {
		return "privacy"
	}
	for _, key := range []string{p.mainKey, p.transport.key} {
		if key == "" || bytes.Contains(raw, []byte(key)) || bytes.Contains(decoded, []byte(key)) {
			return "privacy"
		}
	}
	p.transport.mu.Lock()
	p.transport.request, p.transport.receipt.Requests = bytes.Clone(raw), 1
	p.transport.mu.Unlock()
	if !reflect.DeepEqual(object, grokProjectTitleExpected()) {
		return "request-shape"
	}
	return "none"
}

func grokProjectTitleResponse() []byte {
	return []byte("data: {\"id\":\"uap-grok-title-capture\",\"object\":\"chat.completion.chunk\",\"created\":1,\"model\":\"uap-grok-title-fixture\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"tool_calls\":[{\"index\":0,\"id\":\"uap-grok-title-call\",\"type\":\"function\",\"function\":{\"name\":\"session_title\",\"arguments\":\"{\\\"session_title\\\":\\\"Disposable plugin startup observation\\\"}\"}}]},\"finish_reason\":null}]}\n\ndata: {\"id\":\"uap-grok-title-capture\",\"object\":\"chat.completion.chunk\",\"created\":1,\"model\":\"uap-grok-title-fixture\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"tool_calls\"}]}\n\ndata: [DONE]\n\n")
}

func (p *grokProjectTitleProvider) serve(w http.ResponseWriter, r *http.Request) {
	t := p.transport
	if !t.admit(w, r) {
		return
	}
	raw, err := io.ReadAll(io.LimitReader(r.Body, (1<<20)+1))
	if err != nil || len(raw) > 1<<20 {
		http.Error(w, "Grok title body bound", http.StatusBadRequest)
		t.fail("body")
		return
	}
	if reason := p.capture(raw); reason != "none" {
		http.Error(w, "Grok title request refused", http.StatusBadRequest)
		t.fail(reason)
		return
	}
	response := grokProjectTitleResponse()
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	if count, err := w.Write(response); err != nil || count != len(response) {
		t.fail("response-write")
		return
	}
	t.mu.Lock()
	t.reply, t.receipt.Responses = bytes.Clone(response), 1
	t.mu.Unlock()
}

func (p *grokProjectTitleProvider) finish(t *testing.T, f *projectNativeCLIAttempt) {
	t.Helper()
	transport := p.transport
	if !transport.stop() {
		t.Error("Grok title fixture ordinary shutdown did not complete")
	}
	transport.mu.Lock()
	defer transport.mu.Unlock()
	for name, body := range map[string][]byte{"grok-title-provider.request.json": transport.request, "grok-title-provider.response.sse": transport.reply} {
		if len(body) != 0 {
			f.keep(t, name, body)
		}
	}
	raw, err := json.Marshal(struct {
		Role string `json:"role"`
		grokProjectProviderReceipt
	}{Role: "session-title", grokProjectProviderReceipt: transport.receipt})
	if err != nil || len(raw) > 2<<10 {
		t.Error("Grok title finite receipt refused")
		return
	}
	f.keep(t, "grok-title-provider.json", raw)
}
