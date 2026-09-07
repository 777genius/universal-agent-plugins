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
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

// Scripted provider exercises the native client's tool dispatch, not model intelligence.
// The supported gateway seam is https://code.claude.com/docs/en/llm-gateway.
type claudeGateway struct {
	mu             sync.Mutex
	requests       []json.RawMessage
	skillRequested bool
	skillBodySeen  bool
	results        map[string]json.RawMessage
	calls          map[string]string
}

func claudeRuntimeSession(t *testing.T, f *nativeFixture, cf *claudeNativeFixture, client, label, revision, operation string) string {
	t.Helper()
	g := &claudeGateway{results: map[string]json.RawMessage{}, calls: map[string]string{}}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/messages/count_tokens" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"input_tokens":100}`)
			return
		}
		if r.URL.Path != "/v1/messages" {
			http.Error(w, "fixture endpoint only", 404)
			return
		}
		b, err := io.ReadAll(io.LimitReader(r.Body, 8<<20))
		if err != nil {
			http.Error(w, "read", 400)
			return
		}
		var req struct {
			Tools []struct {
				Name string `json:"name"`
			} `json:"tools"`
			Messages []struct {
				Content json.RawMessage `json:"content"`
			} `json:"messages"`
		}
		if json.Unmarshal(b, &req) != nil {
			http.Error(w, "json", 400)
			return
		}
		g.mu.Lock()
		defer g.mu.Unlock()
		g.requests = append(g.requests, append(json.RawMessage(nil), b...))
		if strings.Contains(string(b), "UAP_SKILL_BODY_"+revision+"_"+f.Nonce) {
			g.skillBodySeen = true
		}
		for _, m := range req.Messages {
			var blocks []struct {
				Type    string          `json:"type"`
				ID      string          `json:"tool_use_id"`
				Content json.RawMessage `json:"content"`
				IsError bool            `json:"is_error"`
			}
			if json.Unmarshal(m.Content, &blocks) != nil {
				continue
			}
			for _, block := range blocks {
				if block.Type == "tool_result" && !block.IsError && g.calls[block.ID] != "" {
					g.results[g.calls[block.ID]] = block.Content
				}
			}
		}
		var content []map[string]any
		for _, tool := range req.Tools {
			if tool.Name == "Skill" && !g.skillRequested && strings.Contains(string(b), "native-proof:native-proof") {
				g.skillRequested = true
				content = append(content, map[string]any{"type": "tool_use", "id": "uap_skill", "name": "Skill", "input": map[string]string{"skill": "native-proof:native-proof"}})
			}
		}
		for _, tool := range req.Tools {
			if !strings.Contains(tool.Name, "native-proof") || !strings.HasSuffix(tool.Name, "inspect_runtime") {
				continue
			}
			if _, ok := g.results[tool.Name]; ok {
				continue
			}
			id := fmt.Sprintf("uap_tool_%d", len(g.calls)+1)
			g.calls[id] = tool.Name
			content = append(content, map[string]any{"type": "tool_use", "id": id, "name": tool.Name, "input": map[string]string{"nonce": f.Nonce, "operation": operation, "marker": "persist-" + f.Nonce}})
		}
		stop := "tool_use"
		if len(content) == 0 {
			content = []map[string]any{{"type": "text", "text": "fixture complete"}}
			stop = "end_turn"
		}
		w.Header().Set("Content-Type", "text/event-stream")
		emit := func(kind string, v any) {
			data, _ := json.Marshal(v)
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", kind, data)
		}
		emit("message_start", map[string]any{"type": "message_start", "message": map[string]any{"id": "msg_uap", "type": "message", "role": "assistant", "model": "claude-sonnet-4-5", "content": []any{}, "stop_reason": nil, "stop_sequence": nil, "usage": map[string]int{"input_tokens": 100, "output_tokens": 0}}})
		for i, c := range content {
			var delta map[string]any
			if c["type"] == "tool_use" {
				input, _ := json.Marshal(c["input"])
				c["input"] = map[string]any{}
				delta = map[string]any{"type": "input_json_delta", "partial_json": string(input)}
			} else {
				delta = map[string]any{"type": "text_delta", "text": c["text"]}
				c["text"] = ""
			}
			emit("content_block_start", map[string]any{"type": "content_block_start", "index": i, "content_block": c})
			emit("content_block_delta", map[string]any{"type": "content_block_delta", "index": i, "delta": delta})
			emit("content_block_stop", map[string]any{"type": "content_block_stop", "index": i})
		}
		emit("message_delta", map[string]any{"type": "message_delta", "delta": map[string]any{"stop_reason": stop, "stop_sequence": nil}, "usage": map[string]int{"output_tokens": 10}})
		emit("message_stop", map[string]any{"type": "message_stop"})
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, client, "-p", "Use the installed native-proof runtime inspection tools.", "--output-format", "stream-json", "--verbose", "--max-turns", "4", "--permission-mode", "bypassPermissions", "--model", "claude-sonnet-4-5")
	cmd.Env = append(cf.env(filepath.Dir(client)), "ANTHROPIC_BASE_URL="+server.URL, "ANTHROPIC_AUTH_TOKEN=uap-disposable-fixture-token", "CLAUDE_CODE_DISABLE_1M_CONTEXT=1")
	cmd.Dir = f.Project
	out, err := cmd.CombinedOutput()
	cf.record(t, label, out)
	g.mu.Lock()
	defer g.mu.Unlock()
	gatewayPath := filepath.Join(f.Root, label+"-gateway.json")
	nativeJSON(t, gatewayPath, g.requests)
	cf.transcriptSHA256[label+"-gateway.json"] = nativeSHA(t, gatewayPath)
	if err != nil {
		t.Fatalf("Claude runtime %s: %v\n%s", label, err, out)
	}
	if !g.skillBodySeen {
		t.Fatal("installed selected skill body was not observed in outgoing provider context")
	}
	if len(g.results) != 3 {
		t.Fatalf("expected installed stdio and HTTP tools returned through native client, got %d; see gateway evidence", len(g.results))
	}
	seen := map[string]bool{}
	var markerPath string
	for name, raw := range g.results {
		var blocks []struct {
			Text string `json:"text"`
		}
		if json.Unmarshal(raw, &blocks) != nil {
			t.Fatalf("tool result content: %s", raw)
		}
		if len(blocks) != 1 {
			t.Fatalf("tool result blocks: %s", raw)
		}
		var facts nativeFacts
		if json.Unmarshal([]byte(blocks[0].Text), &facts) != nil {
			t.Fatalf("tool facts: %s", raw)
		}
		expectedRevision := revision
		if strings.Contains(name, "http") {
			expectedRevision = "HTTP"
		}
		if facts.Nonce != f.Nonce || facts.Revision != expectedRevision || facts.Marker != "persist-"+f.Nonce || facts.Session == "" {
			t.Fatalf("uncorrelated facts: %+v", facts)
		}
		nativeAssertEvent(t, f, facts)
		for _, p := range []string{facts.Root, facts.Data, facts.CWD} {
			nativeRequireContained(t, f.Root, p)
		}
		if strings.Contains(name, "http") {
			seen["http"] = true
			continue
		}
		markerPath = filepath.Join(facts.Data, "native-marker.txt")
		expected := facts.Root
		if strings.Contains(name, "explicit") {
			expected = filepath.Join(expected, "skills")
			seen["explicit"] = true
		} else if strings.Contains(name, "default") {
			seen["default"] = true
		} else {
			t.Fatalf("unexpected fixture tool: %s", name)
		}
		if facts.CWD != expected || facts.Literal != "literal ${UNKNOWN} $HOME" || facts.Once != facts.Root+"/${UNKNOWN}" || len(facts.Argv) != 3 || facts.Argv[1] != facts.Root+"/argument" {
			t.Fatalf("runtime semantics: %+v", facts)
		}
	}
	if !seen["default"] || !seen["explicit"] || !seen["http"] {
		t.Fatal("missing default/explicit stdio tools")
	}
	return markerPath
}

func TestAgentpluginsClaudeNativeRuntimeLifecycle(t *testing.T) {
	if os.Getenv("AGENTPLUGINS_CLAUDE_NATIVE_E2E") != "1" {
		t.Skip("opt-in native client execution")
	}
	nativeRequireDisposable(t)
	client := claudeNativeBinary(t, "AGENTPLUGINS_CLAUDE_BIN")
	installer := claudeNativeBinary(t, "AGENTPLUGINS_INSTALLER_BIN")
	if expected := os.Getenv("AGENTPLUGINS_CLAUDE_SHA256"); len(expected) != 64 || claudeSHA256(t, client) != expected {
		t.Fatal("Claude binary pin mismatch")
	}
	f := newNativeFixture(t, true)
	cf := &claudeNativeFixture{Root: f.Root, Home: f.Home, ConfigDir: filepath.Join(f.Root, "claude-config"), PackageRoot: f.Package, StateHome: filepath.Join(f.Root, "installer-state"), transcriptSHA256: map[string]string{}}
	nativeWrite(t, filepath.Join(cf.ConfigDir, "settings.json"), []byte(`{"enableAllProjectMcpServers":true}`), 0600)
	stages := map[string]string{"install": "not_evaluated", "runtime_A": "not_evaluated", "runtime_B": "not_evaluated", "same_version_refresh": "not_evaluated", "repair": "not_evaluated", "remove": "not_evaluated"}
	identity, identityErr := nativeSourceIdentity(os.Getenv("AGENTPLUGINS_INSTALLER_COMMIT"), os.Getenv("AGENTPLUGINS_INSTALLER_TREE"), os.Getenv("AGENTPLUGINS_INSTALLER_PATCH_SHA256"))
	if identityErr != nil {
		t.Fatal(identityErr)
	}
	evidence := map[string]any{"source_identity": identity, "os": runtime.GOOS, "arch": runtime.GOARCH, "profile_root": cf.ConfigDir, "stages": stages, "provider": "scripted_loopback", "real_model": "not_evaluated", "oauth": "not_evaluated", "client_sha256": claudeSHA256(t, client), "installer_sha256": claudeSHA256(t, installer)}
	defer func() {
		evidence["finished_utc"] = time.Now().UTC().Format(time.RFC3339Nano)
		evidence["transcript_sha256"] = cf.transcriptSHA256
		nativeJSON(t, filepath.Join(f.Root, "evidence.json"), evidence)
	}()
	evidence["scanner"] = nativeProvisionScanner(t, f)
	version := claudeVersionString(t, cf, client)
	evidence["client_version"] = version
	if want := os.Getenv("AGENTPLUGINS_CLAUDE_VERSION"); want == "" || version != want {
		t.Fatalf("Claude version %q does not match pin %q", version, want)
	}
	run := func(label string, args ...string) {
		t.Helper()
		r := claudeRunInstaller(t, cf, installer, filepath.Dir(client), label, args...)
		d, _ := r["data"].(map[string]any)
		if label == "add" && nativeFindString(r, "evidence_source") != "local_scan" {
			t.Fatal("first add did not use local scanner")
		}
		if label == "remove" {
			if err := claudeRetainedRemovalResult(r); err != nil {
				t.Fatal(err)
			}
			return
		}
		if d["status"] != "completed" {
			t.Fatalf("%s: %+v", label, r)
		}
	}
	nativeHTTP(t, f)
	f.RedirectURL = "" // Redirect credential policy is a separate native evidence row.
	f.writePackage(t, "A", "1.0.0")
	stages["install"] = "failed"
	run("add", "add", f.Package, "--target", "claude")
	stages["install"] = "passed"
	entries := claudePluginList(t, cf, client, "installed")
	if len(entries) != 1 || entries[0].ID != "native-proof@skills-dir" {
		t.Fatalf("installed attribution: %+v", entries)
	}
	stages["runtime_A"] = "failed"
	claudeRuntimeSession(t, f, cf, client, "runtime-A", "A", "write")
	stages["runtime_A"] = "passed"
	stages["runtime_B"] = "failed"
	f.writePackage(t, "B", "2.0.0")
	run("update", "update", "native-proof", "--target", "claude")
	claudeRuntimeSession(t, f, cf, client, "runtime-B", "B", "read")
	stages["runtime_B"] = "passed"
	stages["same_version_refresh"] = "failed"
	f.writePackage(t, "C", "2.0.0")
	run("same-version-update", "update", "native-proof", "--target", "claude")
	claudeRuntimeSession(t, f, cf, client, "runtime-C", "C", "read")
	stages["same_version_refresh"] = "passed"
	stages["repair"] = "failed"
	nativeRequireContained(t, f.Root, entries[0].InstallPath)
	if err := os.RemoveAll(entries[0].InstallPath); err != nil {
		t.Fatal(err)
	}
	run("repair", "repair", "native-proof", "--target", "claude")
	markerPath := claudeRuntimeSession(t, f, cf, client, "runtime-repair", "C", "read")
	markerDigest := nativeSHA(t, markerPath)
	stages["repair"] = "passed"
	stages["remove"] = "failed"
	run("remove", "remove", "native-proof", "--target", "claude")
	if got := claudePluginList(t, cf, client, "removed"); len(got) != 0 {
		t.Fatalf("plugin survived removal: %+v", got)
	}
	if _, err := os.Stat(entries[0].InstallPath); !os.IsNotExist(err) {
		t.Fatalf("managed artifact survived remove: %v", err)
	}
	if nativeSHA(t, markerPath) != markerDigest {
		t.Fatal("safe remove changed retained marker")
	}
	stages["remove"] = "passed"
}

// Removal with owned data deliberately has a distinct status. Validate that exact
// contract instead of treating arbitrary success/data_retained output as completion.
func claudeRetainedRemovalResult(r map[string]any) error {
	d, _ := r["data"].(map[string]any)
	targets, _ := d["targets"].([]any)
	if r["result"] != "success" || d["status"] != "data_retained" || d["plugin_data_preserved"] != true || d["data_retained"] != true || len(targets) != 1 {
		return fmt.Errorf("invalid retained removal: %+v", r)
	}
	target, _ := targets[0].(map[string]any)
	output, _ := target["output"].(map[string]any)
	result, _ := output["result"].(map[string]any)
	deactivation, _ := result["deactivation"].(map[string]any)
	if target["target"] != "claude" || target["status"] != "external_completed" || result["mutated"] != true || result["group_phase"] != "external_completed" || deactivation["artifact_removal_allowed"] != true || deactivation["external_removal_complete"] != true {
		return fmt.Errorf("incomplete retained removal: %+v", r)
	}
	return nil
}

func TestClaudeRetainedRemovalResult(t *testing.T) {
	const fixture = `{"result":"success","data":{"status":"data_retained","plugin_data_preserved":true,"data_retained":true,"targets":[{"target":"claude","status":"external_completed","output":{"result":{"mutated":true,"group_phase":"external_completed","deactivation":{"artifact_removal_allowed":true,"external_removal_complete":true}}}}]}}`
	var result map[string]any
	if err := json.Unmarshal([]byte(fixture), &result); err != nil {
		t.Fatal(err)
	}
	if err := claudeRetainedRemovalResult(result); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"plugin_data_preserved", "data_retained", "mutated", "artifact_removal_allowed", "external_removal_complete"} {
		altered := strings.Replace(fixture, `"`+field+`":true`, `"`+field+`":false`, 1)
		var bad map[string]any
		_ = json.Unmarshal([]byte(altered), &bad)
		if claudeRetainedRemovalResult(bad) == nil {
			t.Fatalf("accepted false %s", field)
		}
	}
}
