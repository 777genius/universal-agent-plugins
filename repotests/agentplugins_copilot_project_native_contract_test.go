//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func copilotProjectContractRequest() map[string]any {
	return map[string]any{"model": "uap-fixture-model", "stream": true, "messages": []any{map[string]any{"role": "user", "content": "fixture"}}, "tools": []any{map[string]any{"type": "function", "function": map[string]any{"name": "uapcopilota-inspect_runtime", "description": "Inspect only disposable fixture runtime facts.", "parameters": map[string]any{"type": "object", "properties": map[string]any{"nonce": map[string]any{"type": "string"}, "operation": map[string]any{"type": "string"}, "marker": map[string]any{"type": "string"}}, "required": []any{"nonce"}, "additionalProperties": false}}}}}
}

func copilotProjectOriginalConfigFixture(t *testing.T) *copilotProjectAttempt {
	t.Helper()
	root := t.TempDir()
	ctx, cancel := context.WithDeadline(context.Background(), time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC))
	t.Cleanup(cancel)
	f := &copilotProjectAttempt{
		base:  &nativeFixture{Home: filepath.Join(root, "home"), Project: filepath.Join(root, "A")},
		other: filepath.Join(root, "B"), ctx: ctx, protected: map[string]string{}, evidence: map[string]any{},
		started: time.Date(2026, 10, 20, 12, 34, 56, 123456789, time.FixedZone("fixture-origin", 5*3600+30*60)),
	}
	for _, path := range []string{f.base.Home, f.base.Project, f.other} {
		if err := os.Mkdir(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	return f
}

func TestCopilotProjectNativeOriginalConfigAuthorsHeldFirstLaunchWithoutChangingOrigin(t *testing.T) {
	f := copilotProjectOriginalConfigFixture(t)
	started, originalContext := f.started, f.ctx
	deadline, _ := f.ctx.Deadline()
	if err := f.authorOriginalConfig(t); err != nil {
		t.Fatal(err)
	}
	want := []byte(fmt.Sprintf("{\"firstLaunchAt\":\"2026-10-20T07:04:56.123456789Z\",\"trustedFolders\":[%q,%q]}\n", f.base.Project, f.other))
	body, err := projectNativeRuntimeRead(f.originalConfigPath, copilotProjectConfigDiagnosticLimit)
	info, statErr := os.Lstat(f.originalConfigPath)
	if err != nil || statErr != nil || info.Mode() != 0600 || !bytes.Equal(body, want) || !bytes.Equal(f.originalConfigBaseline, want) {
		t.Fatal("exact independently authored two-field original config differs", err, statErr)
	}
	wantSHA := fmt.Sprintf("%x", sha256.Sum256(want))
	if len(f.protected) != 1 || f.protected[f.originalConfigPath] != wantSHA || f.originalConfigPath != filepath.Join(f.base.Home, ".copilot", "config.json") {
		t.Fatal("original config expectation was not frozen from the independent recipe")
	}
	afterDeadline, ok := f.ctx.Deadline()
	if !ok || afterDeadline != deadline || f.ctx != originalContext || f.started != started {
		t.Fatal("metadata formatting changed the original clock or deadline")
	}
	f.check(t)
	if len(f.evidence) != 0 {
		t.Fatal("unchanged authored config acquired a mutation exception")
	}
}

func TestCopilotProjectNativeOriginalConfigRejectsZeroStartBeforeAnyWrite(t *testing.T) {
	f := copilotProjectOriginalConfigFixture(t)
	f.started = time.Time{}
	if err := f.authorOriginalConfig(t); err == nil {
		t.Fatal("zero attempt origin was replaced by a fresh clock")
	}
	_, err := os.Lstat(filepath.Join(f.base.Home, ".copilot", "config.json"))
	entries, readErr := os.ReadDir(f.base.Home)
	if !os.IsNotExist(err) || readErr != nil || len(entries) != 0 || f.originalConfigPath != "" || len(f.originalConfigBaseline) != 0 || len(f.protected) != 0 {
		t.Fatal("zero-origin refusal wrote or froze an original profile")
	}
}

func copilotProjectOriginalConfigMutation(t *testing.T, f *copilotProjectAttempt, kind string) []byte {
	t.Helper()
	if kind == "comments" {
		return append([]byte("// fixture comment one\n// fixture comment two\n"), f.originalConfigBaseline...)
	}
	value, err := projectNativeRuntimeJSON(f.originalConfigBaseline)
	if err != nil {
		t.Fatal(err)
	}
	object := value.(map[string]any)
	switch kind {
	case "changed-time":
		object["firstLaunchAt"] = "2026-10-21T07:04:56.123456789Z"
	case "removed-time":
		delete(object, "firstLaunchAt")
	case "trust":
		object["trustedFolders"] = []string{f.base.Project}
	case "foreign":
		object["foreignState"] = "must remain refused"
	default:
		t.Fatal("unregistered original-config mutation")
	}
	return append(copilotProjectContractBody(t, object), '\n')
}

func TestCopilotProjectNativeOriginalConfigKeepsCompleteByteMutationGuard(t *testing.T) {
	for _, kind := range []string{"changed-time", "removed-time", "trust", "foreign", "comments"} {
		t.Run(kind, func(t *testing.T) {
			f := copilotProjectOriginalConfigFixture(t)
			if err := f.authorOriginalConfig(t); err != nil {
				t.Fatal(err)
			}
			baseline := bytes.Clone(f.originalConfigBaseline)
			wantSHA := f.protected[f.originalConfigPath]
			changed := copilotProjectOriginalConfigMutation(t, f, kind)
			if err := os.WriteFile(f.originalConfigPath, changed, 0600); err != nil {
				t.Fatal(err)
			}
			got, err := f.protectedDigest(f.originalConfigPath, wantSHA)
			change, ok := f.evidence["original_config_change"].(copilotProjectConfigChange)
			if err != nil || got == wantSHA || got != fmt.Sprintf("%x", sha256.Sum256(changed)) || !ok || change.ExpectedSHA256 != wantSHA || change.ObservedSHA256 != got {
				t.Fatal("complete original byte guard or exact mutation diagnostic changed", err)
			}
			after, err := projectNativeRuntimeRead(f.originalConfigPath, copilotProjectConfigDiagnosticLimit)
			if err != nil || !bytes.Equal(after, changed) || !bytes.Equal(f.originalConfigBaseline, baseline) || f.protected[f.originalConfigPath] != wantSHA {
				t.Fatal("mutation projection rewrote or adopted original authority", err)
			}
		})
	}
}

func TestCopilotProjectNativeReturnedFactsRequireExactInstalledAuthorityAndEvents(t *testing.T) {
	for _, kind := range []string{"valid", "literal-unexpanded", "literal-foreign-home", "literal-project-home", "literal-unknown", "nonce", "revision", "root", "data", "cwd", "session", "event-session", "out-of-order", "call-missing", "duplicate-call", "field-case", "extra-event-field"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			f := &copilotProjectAttempt{base: &nativeFixture{Root: root, Home: filepath.Join(root, "home"), Project: filepath.Join(root, "project"), Nonce: "nonce", Events: filepath.Join(root, "events.jsonl")}, binding: domain.ClientBinding{TargetLocator: filepath.Join(root, "managed")}, data: filepath.Join(root, "data")}
			facts := nativeFacts{Nonce: "nonce", Revision: "A-one", Root: f.binding.TargetLocator, Data: f.data, CWD: f.binding.TargetLocator, Argv: []string{"argument with spaces", f.binding.TargetLocator + "/argument", "${UNKNOWN}"}, Literal: "literal ${UNKNOWN} " + f.base.Home, Once: f.binding.TargetLocator + "/${UNKNOWN}", Marker: "retained-nonce", PID: 123, Session: "123-456"}
			// Keep the negative event and returned payload consistent so only the
			// native environment contract can reject these literal variations.
			switch kind {
			case "literal-unexpanded":
				facts.Literal = "literal ${UNKNOWN} $HOME"
			case "literal-foreign-home":
				facts.Literal = "literal ${UNKNOWN} " + filepath.Join(root, "foreign-home")
			case "literal-project-home":
				facts.Literal = "literal ${UNKNOWN} " + f.base.Project
			case "literal-unknown":
				facts.Literal = "literal replaced " + f.base.Home
			}
			var events bytes.Buffer
			methods := []string{"initialize", "tools/list", "tools/call"}
			if kind == "out-of-order" {
				methods = []string{"tools/list", "initialize", "tools/call"}
			}
			if kind == "call-missing" {
				methods = methods[:2]
			}
			if kind == "duplicate-call" {
				methods = append(methods, "tools/call")
			}
			for _, method := range methods {
				session := facts.Session
				if kind == "event-session" {
					session = "wrong"
				}
				var value any
				if method == "tools/call" {
					value = facts
					if kind == "extra-event-field" {
						var object map[string]any
						_ = json.Unmarshal(copilotProjectContractBody(t, facts), &object)
						object["unknown"] = true
						value = object
					}
				}
				_ = json.NewEncoder(&events).Encode(map[string]any{"method": method, "pid": facts.PID, "session": session, "nonce": facts.Nonce, "timestamp": time.Now().UTC().Format(time.RFC3339Nano), "facts": value})
			}
			projectNativeRuntimeWrite(t, f.base.Events, events.Bytes(), 0600)
			switch kind {
			case "nonce":
				facts.Nonce = "wrong"
			case "revision":
				facts.Revision = "A-two"
			case "root":
				facts.Root = filepath.Join(root, "foreign")
			case "data":
				facts.Data = filepath.Join(root, "foreign")
			case "cwd":
				facts.CWD = root
			case "session":
				facts.Session = "foreign"
			}
			body := copilotProjectContractBody(t, facts)
			if kind == "field-case" {
				var object map[string]any
				_ = json.Unmarshal(body, &object)
				object["Nonce"] = object["nonce"]
				delete(object, "nonce")
				body = copilotProjectContractBody(t, object)
			}
			err := f.validateFacts(string(body))
			if (err == nil) != (kind == "valid") {
				t.Fatalf("facts admission %s: %v", kind, err)
			}
		})
	}
}

func TestCopilotProjectNativeHeldReadsRejectAliasesAndHardlinks(t *testing.T) {
	root := t.TempDir()
	leaf := filepath.Join(root, "ordinary")
	projectNativeRuntimeWrite(t, leaf, []byte("safe"), 0700)
	if got, err := projectNativeRuntimeDigest(leaf); err != nil || len(got) != 64 {
		t.Fatalf("ordinary digest: %v", err)
	}
	if err := os.Symlink(leaf, filepath.Join(root, "alias")); err != nil {
		t.Fatal(err)
	}
	if _, err := projectNativeRuntimeRead(filepath.Join(root, "alias"), 16); err == nil {
		t.Fatal("leaf symlink accepted")
	}
	if err := os.Link(leaf, filepath.Join(root, "hardlink")); err != nil {
		t.Fatal(err)
	}
	if _, err := projectNativeRuntimeDigest(leaf); err == nil {
		t.Fatal("immutable file with multiple links accepted")
	}
}
func copilotProjectContractBody(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestCopilotProjectNativeJSONClosedContract(t *testing.T) {
	for _, body := range [][]byte{[]byte(`{"a":1,"a":2}`), []byte(`{"a":{"x":1,"x":2}}`), []byte(`{"a":NaN}`), []byte(`{} {}`), {0xff}, bytes.Repeat([]byte(" "), copilotProjectBodyLimit+1), []byte(strings.Repeat("[", 34) + strings.Repeat("]", 34))} {
		if _, err := projectNativeRuntimeJSON(body); err == nil {
			t.Fatalf("unsafe JSON admitted: %.80s", body)
		}
	}
	if _, err := copilotProjectRequest(copilotProjectContractBody(t, copilotProjectContractRequest()), "uapcopilota-inspect_runtime"); err != nil {
		t.Fatal(err)
	}
}
func TestCopilotProjectNativeProviderRejectsForeignMissingAndMalformedDeclarations(t *testing.T) {
	for _, kind := range []string{"foreign", "missing", "unknown-envelope", "unknown-message", "duplicate-tools", "schema-extra", "wrong-type"} {
		t.Run(kind, func(t *testing.T) {
			m := copilotProjectContractRequest()
			function := m["tools"].([]any)[0].(map[string]any)["function"].(map[string]any)
			switch kind {
			case "foreign":
				function["name"] = "uapcopilotb-inspect_runtime"
			case "missing":
				delete(m, "tools")
			case "unknown-envelope":
				m["extra"] = true
			case "unknown-message":
				m["messages"].([]any)[0].(map[string]any)["images"] = []any{}
			case "duplicate-tools":
				m["tools"] = append(m["tools"].([]any), m["tools"].([]any)[0])
			case "schema-extra":
				function["parameters"].(map[string]any)["additionalProperties"] = true
			case "wrong-type":
				m["stream"] = 1
			}
			if _, err := copilotProjectRequest(copilotProjectContractBody(t, m), "uapcopilota-inspect_runtime"); err == nil {
				t.Fatal("malformed native request accepted")
			}
		})
	}
}
func TestCopilotProjectNativeProviderRequiresReturnedCallAndFactsBeforeSentinel(t *testing.T) {
	for _, kind := range []string{"valid", "wrong-id", "wrong-facts", "wrong-route", "wrong-host", "credentials"} {
		t.Run(kind, func(t *testing.T) {
			validated := 0
			p := newCopilotProjectProvider("uapcopilota-inspect_runtime", "nonce", func(body string) error {
				validated++
				if body != "facts" {
					return fmt.Errorf("facts mismatch")
				}
				return nil
			})
			p.server = &httptest.Server{URL: "http://127.0.0.1:12345"}
			send := func(m any, route string) *httptest.ResponseRecorder {
				target := p.server.URL + route
				r, err := http.NewRequestWithContext(context.Background(), "POST", target, bytes.NewReader(copilotProjectContractBody(t, m)))
				if err != nil {
					t.Fatal(err)
				}
				r.RequestURI = target
				r.Header.Set("Content-Type", "application/json")
				if kind == "wrong-host" {
					r.Host = "other.invalid"
				}
				if kind == "credentials" {
					r.Header.Set("Authorization", "Bearer forbidden")
				}
				w := httptest.NewRecorder()
				p.serve(w, r)
				return w
			}
			route := "/v1/chat/completions"
			if kind == "wrong-route" {
				route += "?unexpected=1"
			}
			first := send(copilotProjectContractRequest(), route)
			if kind == "wrong-route" || kind == "wrong-host" || kind == "credentials" {
				if first.Code != 400 || p.returned || validated != 0 {
					t.Fatal("unadmitted provider request ran")
				}
				return
			}
			if first.Code != 200 || strings.Contains(first.Body.String(), p.sentinel) {
				t.Fatal("sentinel before returned tool result")
			}
			m := copilotProjectContractRequest()
			id, facts := p.id, "facts"
			if kind == "wrong-id" {
				id = "foreign"
			}
			if kind == "wrong-facts" {
				facts = "wrong"
			}
			args := `{"nonce":"nonce","operation":"write","marker":"retained-nonce"}`
			m["messages"] = []any{map[string]any{"role": "assistant", "content": nil, "tool_calls": []any{map[string]any{"id": p.id, "type": "function", "function": map[string]any{"name": p.tool, "arguments": args}}}}, map[string]any{"role": "tool", "tool_call_id": id, "content": facts}}
			second := send(m, route)
			if kind == "valid" {
				if second.Code != 200 || !p.returned || validated != 1 || !strings.Contains(second.Body.String(), p.sentinel) {
					t.Fatal("valid inert exchange failed")
				}
			} else if second.Code != 400 || p.returned || strings.Contains(second.Body.String(), p.sentinel) {
				t.Fatal("bad returned result passed")
			}
		})
	}
}
