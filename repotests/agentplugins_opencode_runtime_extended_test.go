package pluginkitairepo_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

// Real native sessions, with a scripted provider selecting tools deterministically.
// Provider transcripts prove skill BODY delivery, never model comprehension.
func TestAgentpluginsOpenCodeNativeRuntimeExtended(t *testing.T) {
	if os.Getenv("AGENTPLUGINS_OPENCODE_NATIVE_E2E") != "1" {
		t.Skip("opt-in native execution")
	}
	f := newOpenCodeNativeFixture(t)
	// A replacement value itself contains a known token: recursive expansion
	// would corrupt the literal directory name and is detected by argv/env facts.
	oldRoot := f.Root
	newRoot := oldRoot + "-${PLUGIN_DATA}"
	if err := os.Rename(oldRoot, newRoot); err != nil {
		t.Fatal(err)
	}
	for _, path := range []*string{&f.Root, &f.Home, &f.XDGConfig, &f.XDGData, &f.XDGCache, &f.XDGState, &f.Project, &f.PackageRoot, &f.StateHome} {
		*path = strings.Replace(*path, oldRoot, newRoot, 1)
	}
	t.Logf("extended native evidence: %s", f.Root)
	client := openCodeNativeBinary(t, "AGENTPLUGINS_OPENCODE_BIN")
	installer := openCodeNativeBinary(t, "AGENTPLUGINS_INSTALLER_BIN")
	scanner := openCodePrepareNative(t, f, client)
	if got := openCodeVersionString(t, f, client); got != "1.18.29" {
		t.Fatal(got)
	}
	identity, err := nativeSourceIdentity(os.Getenv("AGENTPLUGINS_INSTALLER_COMMIT"), os.Getenv("AGENTPLUGINS_INSTALLER_TREE"), os.Getenv("AGENTPLUGINS_INSTALLER_PATCH_SHA256"))
	if err != nil {
		t.Fatal(err)
	}
	nf := &nativeFixture{Root: f.Root, Package: f.PackageRoot, Probe: openCodeNativeBinary(t, "AGENTPLUGINS_NATIVE_PROBE_BIN"), Events: filepath.Join(f.Root, "events.jsonl"), Nonce: filepath.Base(oldRoot)}
	writePackage := func(revision, version string) {
		nf.writePackage(t, revision, version)
		path := filepath.Join(nf.Package, "mcp.json")
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var doc map[string]any
		if json.Unmarshal(b, &doc) != nil {
			t.Fatal("fixture JSON")
		}
		for _, raw := range doc["mcpServers"].(map[string]any) {
			env := raw.(map[string]any)["env"].(map[string]any)
			env["UAP_TEST_ROOT"] = "${PLUGIN_ROOT}/../../../../.."
			env["UAP_TEST_EVENTS"] = "${PLUGIN_ROOT}/../../../../../events.jsonl"
		}
		nativeJSON(t, path, doc)
	}

	var mu sync.Mutex
	var requests []json.RawMessage
	var step int
	removed := false
	operation := "write"
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(io.LimitReader(r.Body, 4<<20))
		mu.Lock()
		defer mu.Unlock()
		requests = append(requests, append(json.RawMessage(nil), b...))
		w.Header().Set("Content-Type", "text/event-stream")
		emit := func(delta any, finish any) {
			body, _ := json.Marshal(map[string]any{"id": "fixture", "object": "chat.completion.chunk", "created": 1, "model": "fixture", "choices": []any{map[string]any{"index": 0, "delta": delta, "finish_reason": finish}}})
			fmt.Fprintf(w, "data: %s\n\n", body)
		}
		names := []string{"default_inspect_runtime", "explicit_inspect_runtime", "skill"}
		var request struct {
			Tools []struct {
				Function struct {
					Name string `json:"name"`
				} `json:"function"`
			} `json:"tools"`
		}
		mainRequest := false
		if json.Unmarshal(b, &request) == nil {
			for _, tool := range request.Tools {
				if tool.Function.Name == "default_inspect_runtime" {
					mainRequest = true
				}
			}
		}
		if !removed && step < len(names) && mainRequest {
			var args any = map[string]string{"nonce": nf.Nonce, "operation": operation, "marker": nf.Nonce}
			if step == 2 {
				args = map[string]string{"name": "native-proof"}
			}
			a, _ := json.Marshal(args)
			emit(map[string]any{"role": "assistant", "tool_calls": []any{map[string]any{"index": 0, "id": fmt.Sprintf("call-%d", step), "type": "function", "function": map[string]any{"name": names[step], "arguments": string(a)}}}}, nil)
			emit(map[string]any{}, "tool_calls")
			step++
		} else {
			emit(map[string]any{"role": "assistant", "content": "Fixture complete"}, nil)
			emit(map[string]any{}, "stop")
		}
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer provider.Close()
	nativeJSON(t, filepath.Join(f.XDGConfig, "opencode", "opencode.json"), map[string]any{"provider": map[string]any{"uap-fixture": map[string]any{"npm": "@ai-sdk/openai-compatible", "env": []string{}, "models": map[string]any{"fixture": map[string]any{"name": "Fixture", "tool_call": true, "limit": map[string]any{"context": 32000, "output": 1024}}}, "options": map[string]any{"apiKey": "fixture", "baseURL": provider.URL + "/v1"}}}, "model": "uap-fixture/fixture", "small_model": "uap-fixture/fixture", "permission": "allow"})
	stages := map[string]any{}
	evidence := map[string]any{"status": "failed", "source_identity": identity, "client_sha256": openCodeSHA256(t, client), "installer_sha256": openCodeSHA256(t, installer), "probe_sha256": openCodeSHA256(t, nf.Probe), "scanner": scanner, "stages": stages, "provider": "scripted_loopback_no_real_model", "transcript_sha256": f.transcriptSHA256}
	defer func() { nativeJSON(t, filepath.Join(f.Root, "extended-runtime-evidence.json"), evidence) }()
	var root, data string
	session := func(label, revision string) {
		t.Helper()
		mu.Lock()
		step = 0
		requests = nil
		mu.Unlock()
		nativeWrite(t, nf.Events, nil, 0600)
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, client, "run", "--model", "uap-fixture/fixture", "--format", "json", "Inspect the disposable runtime and load the native-proof skill, then finish.")
		cmd.Dir = f.Project
		cmd.Env = f.env(filepath.Dir(client))
		out, err := cmd.CombinedOutput()
		f.record(t, label, out)
		mu.Lock()
		captured := append([]json.RawMessage(nil), requests...)
		mu.Unlock()
		nativeJSON(t, filepath.Join(f.Root, label+"-provider.json"), captured)
		providerBytes, marshalErr := json.Marshal(captured)
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		f.record(t, label+"-provider", providerBytes)
		if err != nil {
			t.Fatalf("session %s: %v\n%s", label, err, out)
		}
		events, err := os.ReadFile(nf.Events)
		if err != nil {
			t.Fatal(err)
		}
		f.record(t, label+"-events", events)
		var facts []nativeFacts
		for _, line := range strings.Split(strings.TrimSpace(string(events)), "\n") {
			var e struct {
				Method string      `json:"method"`
				Facts  nativeFacts `json:"facts"`
			}
			if strings.TrimSpace(line) == "" {
				continue
			}
			if err := json.Unmarshal([]byte(line), &e); err != nil {
				t.Fatal(err)
			}
			if e.Method == "tools/call" {
				facts = append(facts, e.Facts)
			}
		}
		if removed {
			if len(strings.TrimSpace(string(events))) != 0 {
				t.Fatal("removed runtime dispatched")
			}
			if len(captured) == 0 {
				t.Fatal("no removed session provider request")
			}
			mainSeen := false
			for _, request := range captured {
				var req struct {
					Messages []struct {
						Role    string          `json:"role"`
						Content json.RawMessage `json:"content"`
					} `json:"messages"`
					Tools json.RawMessage `json:"tools"`
				}
				if json.Unmarshal(request, &req) != nil {
					t.Fatal("bad provider capture")
				}
				if len(req.Tools) > 2 {
					mainSeen = true
				}
				for _, message := range req.Messages {
					if message.Role == "system" && (strings.Contains(string(message.Content), "native-proof") || strings.Contains(string(message.Content), "UAP_SKILL_BODY_")) {
						t.Fatal("removed skill metadata still in outgoing system context")
					}
				}
				for _, forbidden := range []string{"default_inspect_runtime", "explicit_inspect_runtime", "native-proof", "UAP_SKILL_BODY_"} {
					if strings.Contains(string(req.Tools), forbidden) {
						t.Fatalf("removed runtime still advertised: %s", forbidden)
					}
				}
			}

			if !mainSeen {
				t.Fatal("removed session had no main request")
			}
			stages[label] = map[string]any{"status": "passed"}
			return
		}
		if len(facts) != 2 {
			t.Fatalf("want two native calls, got %d\n%s", len(facts), out)
		}
		for i, v := range facts {
			if root == "" {
				root = v.Root
				data = v.Data
			}
			wantCWD := root
			if i == 1 {
				wantCWD = filepath.Join(root, "skills")
			}
			if v.Root != root || v.Data != data || v.CWD != wantCWD || v.Revision != revision || v.Marker != nf.Nonce || v.Literal != "literal ${UNKNOWN} $HOME" || v.Once != root+"/${UNKNOWN}" || !reflect.DeepEqual(v.Argv, []string{"argument with spaces", root + "/argument", "${UNKNOWN}"}) {
				t.Fatalf("bad runtime facts: %+v", v)
			}
		}
		bodyNonce := "UAP_SKILL_BODY_" + revision + "_" + nf.Nonce
		bodySeen := false
		returned := map[string]nativeFacts{}
		for _, request := range captured {
			var req struct {
				Tools    json.RawMessage `json:"tools"`
				Messages []struct {
					Role    string          `json:"role"`
					Content json.RawMessage `json:"content"`
				} `json:"messages"`
			}
			if json.Unmarshal(request, &req) != nil {
				t.Fatal("bad provider capture")
			}
			if strings.Contains(string(req.Tools), bodyNonce) {
				t.Fatal("BODY leaked into tools metadata")
			}
			for _, message := range req.Messages {
				if strings.Contains(string(message.Content), bodyNonce) {
					if message.Role != "tool" {
						t.Fatal("BODY leaked before native skill tool")
					}
					bodySeen = true
				}
				if message.Role == "tool" {
					var content string
					if json.Unmarshal(message.Content, &content) == nil {
						var got nativeFacts
						if json.Unmarshal([]byte(content), &got) == nil && got.Nonce == nf.Nonce {
							returned[got.Session] = got
						}
					}
				}
			}
		}
		if !bodySeen || strings.Contains(string(captured[0]), bodyNonce) {
			t.Fatal("skill BODY not proven in outgoing tool context")
		}
		for _, v := range facts {
			nativeAssertEvent(t, nf, v)
			if !reflect.DeepEqual(returned[v.Session], v) {
				t.Fatal("native protocol facts did not return to provider")
			}
		}

		stages[label] = map[string]any{"status": "passed", "facts": facts, "skill_body_outgoing": true}

	}
	writePackage("A", "1.0.0")
	added := openCodeRunInstaller(t, f, installer, filepath.Dir(client), "add", "add", f.PackageRoot, "--target", "opencode")
	openCodeAssertCompleted(t, "add", added)
	session("install", "A")
	operation = "read"
	for _, u := range []struct{ label, revision, version string }{{"update", "B", "2.0.0"}, {"refresh", "C", "2.0.0"}} {
		writePackage(u.revision, u.version)
		result := openCodeRunInstaller(t, f, installer, filepath.Dir(client), u.label, "update", f.PackageRoot, "--target", "opencode")
		openCodeAssertCompleted(t, u.label, result)
		openCodeAssertMutated(t, result, true)
		session(u.label, u.revision)
	}
	if err := os.RemoveAll(root); err != nil {
		t.Fatal(err)
	}
	repaired := openCodeRunInstaller(t, f, installer, filepath.Dir(client), "repair", "repair", "native-proof", "--target", "opencode")
	openCodeAssertCompleted(t, "repair", repaired)
	session("repaired", "C")
	result := openCodeRunInstaller(t, f, installer, filepath.Dir(client), "remove", "remove", "native-proof", "--target", "opencode")
	d, _ := result["data"].(map[string]any)
	if result["result"] != "success" || d["status"] != "data_retained" {
		t.Fatal(result)
	}
	marker, err := os.ReadFile(filepath.Join(data, "native-marker.txt"))
	if err != nil || string(marker) != nf.Nonce {
		t.Fatal("persistent data lost", err)
	}
	removed = true
	session("removed", "")
	evidence["status"] = "passed"
}
