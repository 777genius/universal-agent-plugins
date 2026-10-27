//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// Actual611 title bytes, changing only the prospective explicitly configured
// auxiliary model. This inert input is not successful native evidence.
const grokTitleContractGolden = `{"model":"uap-grok-title-fixture","messages":[{"role":"system","content":"You are tasked with generating the session title. The user is asking almost always software engineering related questions on their codebase.\nWe describe the session title below\n# Session Title\nA short and distinctive 5-10 word descriptive title for the session. Super info dense, no filler.\n\nYou will be given the user query below encapsulated in <user_query></user_query>.\n\nJust generate the session_title and nothing else"},{"role":"user","content":"<user_query>\nDescribe only the installed uap-grok-attempt skill marker. Do not run any tool or change any file.\n</user_query>"}],"temperature":1.0,"max_tokens":100,"tools":[{"type":"function","function":{"name":"session_title","description":"Generate the session_title which we use for the user_message","parameters":{"type":"object","required":["session_title"],"properties":{"session_title":{"type":"string","description":"Final session title, just 5-10 word descriptive title for the session. Super info dense, no filler."}},"additionalProperties":false}}}],"tool_choice":{"type":"function","function":{"name":"session_title"}},"stream":true,"stream_options":{"include_usage":true}}`

func grokTitleContractProvider(t *testing.T) (*grokProjectProvider, *grokProjectTitleProvider) {
	t.Helper()
	main := grokProjectContractProvider(t)
	transport, err := newGrokProjectProvider(main.ctx, main.cancel, main.totalEnd)
	if err != nil {
		t.Fatal(err)
	}
	title := &grokProjectTitleProvider{transport: transport, mainKey: main.key}
	main.title = title
	transport.receipt.Transport, transport.receipt.Model = grokProjectTitleTransport, grokProjectTitleModel
	if err := title.start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { transport.stop() })
	return main, title
}

func grokTitleContractInput(t *testing.T, kind string, main *grokProjectProvider, title *grokProjectTitleProvider) []byte {
	t.Helper()
	body := []byte(grokTitleContractGolden)
	switch kind {
	case "array":
		return []byte(`[]`)
	case "null":
		return []byte(`null`)
	case "depth":
		return []byte(strings.Repeat("[", 34) + "0" + strings.Repeat("]", 34))
	case "utf8":
		return []byte{255}
	case "invalid":
		return []byte(`{"model":`)
	case "duplicate":
		return []byte(`{"model":"one","model":"two"}`)
	case "trailing":
		return append(body, []byte("{}")...)
	case "nonfinite":
		return []byte(`{"temperature":1e9999}`)
	case "oversized":
		return bytes.Repeat([]byte{' '}, (1<<20)+1)
	case "privacy-main":
		return []byte(`{"opaque":"` + main.key + `"}`)
	case "privacy-title":
		return []byte(`{"opaque":"` + title.transport.key + `"}`)
	case "encoded-main":
		return []byte(`{"opaque":"\u0075` + main.key[1:] + `"}`)
	case "encoded-title":
		return []byte(`{"opaque":"\u0075` + title.transport.key[1:] + `"}`)
	}
	var value map[string]any
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		t.Fatal(err)
	}
	switch kind {
	case "main-model":
		value["model"] = "uap-grok-fixture"
	case "unknown-model":
		value["model"] = "grok-4.6"
	case "extra":
		value["unknown"] = "opaque but safe"
	case "missing":
		delete(value, "stream")
	case "system":
		value["messages"].([]any)[0].(map[string]any)["content"] = "changed system"
	case "prompt":
		value["messages"].([]any)[1].(map[string]any)["content"] = "changed user"
	case "messages":
		value["messages"] = append(value["messages"].([]any), map[string]any{"role": "tool", "content": "extra"})
	case "schema":
		value["tools"].([]any)[0].(map[string]any)["function"].(map[string]any)["parameters"].(map[string]any)["additionalProperties"] = true
	case "tool":
		value["tools"].([]any)[0].(map[string]any)["function"].(map[string]any)["name"] = "inspect_runtime"
	case "options":
		value["stream_options"].(map[string]any)["unknown"] = true
	case "temperature":
		value["temperature"] = true
	case "tokens":
		value["max_tokens"] = float64(101)
	case "choice":
		value["tool_choice"] = "auto"
	}
	body, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func TestGrokProjectTitleFixtureKeepsOneExactSourceDefinedExchange(t *testing.T) {
	if len(grokTitleContractGolden) != 1220 || fmt.Sprintf("%x", sha256.Sum256([]byte(grokTitleContractGolden))) != "6310901ba3d8138dbf06eb01e447139f815a42424513ed048263f39b1ced658a" {
		t.Fatal("independently frozen measured-title input differs")
	}
	main, title := grokTitleContractProvider(t)
	status, body := grokProjectContractSend(t, grokProjectContractRequest(t, title.transport, []byte(grokTitleContractGolden)))
	if status != http.StatusOK || len(body) != 570 || fmt.Sprintf("%x", sha256.Sum256(body)) != "eea6c4c389c7e27cf0e6d1cc9e705bc84a4d77ae8c5afddaa846ec8255e6ed68" {
		t.Fatal("fixed title response bytes differ")
	}
	events := bytes.Split(body, []byte("\n\n"))
	if len(events) != 4 || !bytes.Equal(events[2], []byte("data: [DONE]")) || len(events[3]) != 0 {
		t.Fatal("title SSE termination differs")
	}
	var first, last map[string]any
	if json.Unmarshal(bytes.TrimPrefix(events[0], []byte("data: ")), &first) != nil || json.Unmarshal(bytes.TrimPrefix(events[1], []byte("data: ")), &last) != nil {
		t.Fatal("title SSE JSON differs")
	}
	choice := first["choices"].([]any)[0].(map[string]any)
	delta := choice["delta"].(map[string]any)
	call := delta["tool_calls"].([]any)[0].(map[string]any)
	function := call["function"].(map[string]any)
	if first["model"] != "uap-grok-title-fixture" || first["id"] != "uap-grok-title-capture" || call["id"] != "uap-grok-title-call" || function["name"] != "session_title" || function["arguments"] != `{"session_title":"Disposable plugin startup observation"}` || last["choices"].([]any)[0].(map[string]any)["finish_reason"] != "tool_calls" || len(delta) != 2 {
		t.Fatal("title function identity/result differs")
	}
	f := &projectNativeCLIAttempt{grok: main, files: map[string][]byte{}}
	f.keep(t, "native-attempt.stderr.log", []byte(main.key+" "+title.transport.key))
	main.finish(t, f)
	if len(f.files) != 5 || title.transport.receipt.Requests != 1 || title.transport.receipt.Responses != 1 || !title.transport.receipt.ShutdownComplete || title.transport.receipt.ForcedClose || title.transport.receipt.Failure != "none" || main.receipt.Requests != 0 || main.receipt.Responses != 0 {
		t.Fatal("title contaminated primary counters or failed ordinary closure")
	}
	for _, raw := range f.files {
		if grokProjectPrivateCredential(raw) || bytes.Contains(raw, []byte(main.key)) || bytes.Contains(raw, []byte(title.transport.key)) {
			t.Fatal("either private fixture key escaped retention")
		}
	}
	var receipt map[string]any
	if json.Unmarshal(f.files["grok-title-provider.json"], &receipt) != nil || receipt["role"] != "session-title" || receipt["model"] != "uap-grok-title-fixture" || len(f.files["grok-title-provider.json"]) > 2048 {
		t.Fatal("separate finite title role receipt differs")
	}
}

func TestGrokProjectTitleFixtureRefusesUnmeasuredSchemaAndPrivateBodies(t *testing.T) {
	kinds := []string{"main-model", "unknown-model", "extra", "missing", "system", "prompt", "messages", "schema", "tool", "options", "temperature", "tokens", "choice", "array", "null", "depth", "utf8", "invalid", "duplicate", "trailing", "nonfinite", "oversized", "privacy-main", "privacy-title", "encoded-main", "encoded-title"}
	for _, kind := range kinds {
		t.Run(kind, func(t *testing.T) {
			main, title := grokTitleContractProvider(t)
			raw := grokTitleContractInput(t, kind, main, title)
			status, _ := grokProjectContractSend(t, grokProjectContractRequest(t, title.transport, raw))
			title.transport.stop()
			uncaptured := kind == "array" || kind == "null" || kind == "depth" || kind == "utf8" || kind == "invalid" || kind == "duplicate" || kind == "trailing" || kind == "nonfinite" || kind == "oversized" || strings.HasPrefix(kind, "privacy-") || strings.HasPrefix(kind, "encoded-")
			requests := 1
			if uncaptured {
				requests = 0
			}
			if status != http.StatusBadRequest || title.transport.receipt.Failure == "none" || title.transport.receipt.Requests != requests || title.transport.receipt.Responses != 0 || (len(title.transport.request) == 0) != uncaptured || main.receipt.Requests != 0 || main.ctx.Err() == nil {
				t.Fatal("unknown/private body escaped failure or retained forbidden bytes")
			}
			if !uncaptured && !bytes.Equal(title.transport.request, raw) {
				t.Fatal("safe shape-refused body was not retained exactly")
			}
		})
	}
}

func TestGrokProjectTitleFixtureRetainsOriginalHTTPAndSingleAttemptBoundary(t *testing.T) {
	for _, kind := range []string{"query", "method", "get-root", "host", "credential", "main-credential", "duplicate-credential", "content-type", "duplicate-content-type", "deadline", "second"} {
		t.Run(kind, func(t *testing.T) {
			main, title := grokTitleContractProvider(t)
			if kind == "second" {
				status, _ := grokProjectContractSend(t, grokProjectContractRequest(t, title.transport, []byte(grokTitleContractGolden)))
				if status != 200 {
					t.Fatal("first title request failed")
				}
			}
			request := grokProjectContractRequest(t, title.transport, []byte(grokTitleContractGolden))
			grokProjectContractMutateHTTP(request, kind)
			switch kind {
			case "main-credential":
				request.Header.Set("Authorization", "Bearer "+main.key)
			case "get-root":
				request.Method = http.MethodGet
				request.URL.Path = "/"
			case "deadline":
				main.cancel()
			}
			status, _ := grokProjectContractSend(t, request)
			title.transport.stop()
			responses := 0
			if kind == "second" {
				responses = 1
			}
			if status != 400 || title.transport.receipt.Failure == "none" || title.transport.receipt.Responses != responses || main.receipt.Requests != 0 {
				t.Fatal("title route/key/deadline/one-attempt boundary widened")
			}
		})
	}
}

type grokTitleContractWriteFailure struct{ headers http.Header }

func (w grokTitleContractWriteFailure) Header() http.Header { return w.headers }
func (grokTitleContractWriteFailure) WriteHeader(int)       {}
func (grokTitleContractWriteFailure) Write([]byte) (int, error) {
	return 0, errors.New("inert title write refusal")
}

func TestGrokProjectTitleFixtureResponseWriteFailureCannotRecordCompletion(t *testing.T) {
	_, title := grokTitleContractProvider(t)
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(grokTitleContractGolden))
	request.Host = title.transport.listener.Addr().String()
	request.RemoteAddr = "127.0.0.1:12345"
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+title.transport.key)
	title.serve(grokTitleContractWriteFailure{headers: http.Header{}}, request)
	title.transport.stop()
	if title.transport.receipt.Requests != 1 || title.transport.receipt.Responses != 0 || title.transport.receipt.Failure != "response-write" || len(title.transport.reply) != 0 {
		t.Fatal("failed response write claimed a completed title")
	}
}

func TestGrokProjectTitleFixtureUsesOriginalForcedCleanupRefusal(t *testing.T) {
	_, title := grokTitleContractProvider(t)
	p := title.transport
	connection, err := (&net.Dialer{Timeout: time.Second}).DialContext(p.ctx, "tcp4", p.listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = connection.Close() }()
	if _, err := fmt.Fprintf(connection, "POST /v1/chat/completions HTTP/1.1\r\nHost: %s\r\nAuthorization: Bearer %s\r\nContent-Type: application/json\r\nContent-Length: 1000\r\n\r\n{", p.listener.Addr(), p.key); err != nil {
		t.Fatal("inert partial title request failed")
	}
	end := time.Now().Add(time.Second)
	for {
		p.mu.Lock()
		entered := p.attempted
		p.mu.Unlock()
		if entered {
			break
		}
		if time.Now().After(end) {
			t.Fatal("actual title handler never received partial body")
		}
		time.Sleep(time.Millisecond)
	}
	began := time.Now()
	ordinary := p.stop()
	if ordinary || !p.receipt.ForcedClose || !p.receipt.ShutdownComplete || p.connections != 0 || time.Since(began) > 2500*time.Millisecond || p.receipt.Failure != "cleanup" {
		t.Fatal("incomplete title body was not bounded and failed on forced cleanup")
	}
	select {
	case <-p.serveDone:
	default:
		t.Fatal("title HTTP serve goroutine survived cleanup")
	}
}
