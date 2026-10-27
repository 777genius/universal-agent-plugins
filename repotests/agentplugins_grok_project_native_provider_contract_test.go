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
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func grokProjectContractProvider(t *testing.T) *grokProjectProvider {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	p, err := newGrokProjectProvider(ctx, cancel, time.Now().Add(10*time.Second))
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	t.Cleanup(func() { p.stop(); cancel() })
	if err := p.start(); err != nil {
		t.Fatal(err)
	}
	return p
}

func grokProjectContractRequest(t *testing.T, p *grokProjectProvider, body []byte) *http.Request {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	t.Cleanup(cancel)
	r, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://"+p.listener.Addr().String()+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Authorization", "Bearer "+p.key)
	return r
}

func grokProjectContractSend(t *testing.T, r *http.Request) (int, []byte) {
	t.Helper()
	transport := &http.Transport{DisableKeepAlives: true}
	defer transport.CloseIdleConnections()
	response, err := (&http.Client{Transport: transport, Timeout: 3 * time.Second}).Do(r)
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 2049))
	closeErr := response.Body.Close()
	if err != nil || closeErr != nil || len(body) > 2048 {
		t.Fatal("bounded inert HTTP response failed")
	}
	return response.StatusCode, body
}

func grokProjectContractBody(t *testing.T, p *grokProjectProvider) []byte {
	t.Helper()
	body, err := json.Marshal(map[string]any{"model": "uap-grok-fixture", "stream": true,
		"messages": []any{map[string]string{"role": "user", "content": "inert request"}}, "opaque_research_field": p.key})
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func TestGrokProjectNativeProviderOneOpaqueExchangeKeepsClosedRedactedEvidence(t *testing.T) {
	p := grokProjectContractProvider(t)
	status, body := grokProjectContractSend(t, grokProjectContractRequest(t, p, grokProjectContractBody(t, p)))
	if status != http.StatusOK || !bytes.Equal(body, grokProjectProviderResponse()) || bytes.Contains(body, []byte("tool_calls")) {
		t.Fatal("fixed text-only provider response differs")
	}
	f := &projectNativeCLIAttempt{grok: p, files: map[string][]byte{}}
	f.keep(t, "native-attempt.stderr.log", []byte("private diagnostic "+p.key))
	p.finish(t, f)
	if len(f.files) != 4 || p.receipt.Requests != 1 || p.receipt.Responses != 1 || !p.receipt.CredentialMatched || !p.receipt.ShutdownComplete || p.receipt.ForcedClose || p.receipt.Failure != "none" {
		t.Fatal("actual completed request/response/cleanup receipt differs")
	}
	for _, captured := range f.files {
		if bytes.Contains(captured, []byte(p.key)) || bytes.Contains(captured, []byte("uap-grok-private-")) {
			t.Fatal("private credential escaped into a captured member")
		}
	}
	if !bytes.Contains(f.files["grok-provider.request.json"], []byte("opaque_research_field")) || !bytes.Contains(f.files["grok-provider.request.json"], []byte("[redacted-grok-fixture-key]")) {
		t.Fatal("opaque first request or its redaction was lost")
	}
}

func TestGrokProjectNativeProviderRefusesWrongHTTPAndBodyWithoutFurtherResponse(t *testing.T) {
	for _, kind := range []string{"query", "method", "host", "credential", "duplicate-credential", "content-type", "duplicate-content-type", "model", "stream", "empty-messages", "invalid-json", "duplicate-json", "oversized", "late", "duplicate"} {
		t.Run(kind, func(t *testing.T) {
			p := grokProjectContractProvider(t)
			body := grokProjectContractBody(t, p)
			switch kind {
			case "model":
				body = bytes.ReplaceAll(body, []byte("uap-grok-fixture"), []byte("foreign"))
			case "stream":
				body = bytes.ReplaceAll(body, []byte(`"stream":true`), []byte(`"stream":false`))
			case "empty-messages":
				body = []byte(`{"model":"uap-grok-fixture","stream":true,"messages":[]}`)
			case "invalid-json":
				body = []byte(`{"model":`)
			case "duplicate-json":
				body = []byte(`{"model":"uap-grok-fixture","model":"uap-grok-fixture"}`)
			case "oversized":
				body = bytes.Repeat([]byte{' '}, (1<<20)+1)
			case "late":
				p.cancel()
			case "duplicate":
				if status, _ := grokProjectContractSend(t, grokProjectContractRequest(t, p, body)); status != http.StatusOK {
					t.Fatal("first request refused before duplicate control")
				}
			}
			r := grokProjectContractRequest(t, p, body)
			grokProjectContractMutateHTTP(r, kind)
			status, _ := grokProjectContractSend(t, r)
			p.stop()
			wantResponses := 0
			if kind == "duplicate" {
				wantResponses = 1
			}
			if status != http.StatusBadRequest || p.receipt.Failure == "none" || p.receipt.Responses != wantResponses || !p.receipt.ShutdownComplete || p.connections != 0 {
				t.Fatal("invalid request continued or actual cleanup was not proved")
			}
		})
	}
}

func grokProjectContractMutateHTTP(r *http.Request, kind string) {
	switch kind {
	case "query":
		r.URL.RawQuery = "extra=1"
	case "method":
		r.Method = http.MethodPut
	case "host":
		r.Host = "foreign.invalid"
	case "credential":
		r.Header.Set("Authorization", "Bearer foreign")
	case "duplicate-credential":
		r.Header.Add("Authorization", r.Header.Get("Authorization"))
	case "content-type":
		r.Header.Set("Content-Type", "text/plain")
	case "duplicate-content-type":
		r.Header.Add("Content-Type", "application/json")
	}
}

func TestGrokProjectNativeProviderRouteRequiresLiteralOriginAndLoopback(t *testing.T) {
	for _, kind := range []string{"valid", "absolute", "escaped", "foreign-peer", "missing-peer"} {
		r := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
		r.Host, r.RemoteAddr = "127.0.0.1:12345", "127.0.0.1:12346"
		r.Header.Set("Content-Type", "application/json")
		switch kind {
		case "absolute":
			r.RequestURI = "http://127.0.0.1:12345/v1/chat/completions"
		case "escaped":
			r.URL.RawPath = "/v1/chat/%63ompletions"
		case "foreign-peer":
			r.RemoteAddr = "192.0.2.1:12346"
		case "missing-peer":
			r.RemoteAddr = ""
		}
		if grokProjectProviderRoute(r, "127.0.0.1:12345") != (kind == "valid") {
			t.Fatal("literal route or peer boundary differs", kind)
		}
	}
}

func TestGrokProjectNativeProviderEncodedCredentialIsRefusedBeforeCapture(t *testing.T) {
	p := grokProjectContractProvider(t)
	encoded := []byte(`{"opaque":"\u0075` + p.key[1:] + `"}`)
	if bytes.Contains(encoded, []byte(p.key)) || !grokProjectPrivateCredential(p.redact(encoded)) {
		t.Fatal("encoded private fixture credential escaped final refusal")
	}
	if grokProjectPrivateCredential(p.redact([]byte(p.key))) {
		t.Fatal("exact fixture credential was not redacted")
	}
}

func TestGrokProjectNativeProviderPartialBodyIsClosedWithinOriginalCleanupBudget(t *testing.T) {
	p := grokProjectContractProvider(t)
	conn, err := (&net.Dialer{Timeout: time.Second}).DialContext(p.ctx, "tcp4", p.listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	if _, err := fmt.Fprintf(conn, "POST /v1/chat/completions HTTP/1.1\r\nHost: %s\r\nAuthorization: Bearer %s\r\nContent-Type: application/json\r\nContent-Length: 1000\r\n\r\n{", p.listener.Addr(), p.key); err != nil {
		t.Fatal("inert partial request failed")
	}
	deadline := time.Now().Add(time.Second)
	for {
		p.mu.Lock()
		entered := p.attempted
		p.mu.Unlock()
		if entered {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("partial request never reached the actual handler")
		}
		time.Sleep(time.Millisecond)
	}
	start := time.Now()
	ordinary := p.stop()
	if ordinary || !p.receipt.ForcedClose || !p.receipt.ShutdownComplete || p.connections != 0 || time.Since(start) > 2500*time.Millisecond {
		t.Fatal("partial-input cleanup was unbounded or mislabeled ordinary")
	}
	select {
	case <-p.serveDone:
	default:
		t.Fatal("owned HTTP serve goroutine survived cleanup")
	}
}

func TestGrokProjectNativeProviderUsesOriginalHomeAndPreservesConfigAuthority(t *testing.T) {
	root := t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	f := &projectNativeCLIAttempt{spec: projectNativeCLISpec{client: "grok"}, ctx: ctx, cancel: cancel, started: time.Now(),
		base:      &nativeFixture{Root: root, Home: filepath.Join(root, "original-home"), Project: filepath.Join(root, "project")},
		protected: map[string]string{}, evidence: map[string]any{}}
	t.Cleanup(func() {
		if f.grok != nil {
			f.grok.stop()
		}
		cancel()
	})
	f.configureGrokProvider(t)
	path := filepath.Join(f.base.Home, ".grok", "config.toml")
	body, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(body, f.grok.configuration()) || bytes.Contains(body, []byte(f.grok.key)) || f.protected[path] != nativeSHA(t, path) {
		t.Fatal("original user config or immutable baseline differs")
	}
	if _, err := os.Stat(filepath.Join(f.base.Project, ".grok", "config.toml")); !os.IsNotExist(err) {
		t.Fatal("project was substituted for original user profile")
	}
	joined := strings.Join(f.env(), "\n")
	if !strings.Contains(joined, "HOME="+f.base.Home+"\n") || !strings.Contains(joined, "UAP_GROK_TEST_PROVIDER_KEY="+f.grok.key) {
		t.Fatal("held original HOME or explicit private environment missing")
	}
	for _, client := range []string{"qoder", "kiro", "windsurf"} {
		f.spec.client = client
		if strings.Contains(strings.Join(f.env(), "\n"), "UAP_GROK_TEST_PROVIDER_KEY=") {
			t.Fatal("another client inherited Grok provider authority")
		}
	}
}
