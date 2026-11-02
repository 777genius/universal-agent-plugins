package pluginkitairepo_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

const codexForeignProjectTool = "mcp__foreign_fixture__uap_ping"
const codexForeignProjectCallID = "call_uap_codex_project_foreign"

// Only the model is scripted. A real pinned Codex app-server must discover the
// project MCP child, execute its tools/call, and return the result to Responses.
type codexProjectModel struct {
	server  *httptest.Server
	mu      sync.Mutex
	current *codexProjectModelStage
}

type codexProjectModelStage struct {
	wantCall   bool
	requests   int
	advertised int
	issued     int
	results    int
	invalid    []string
	done       chan struct{}
	once       sync.Once
}

func newCodexProjectModel(t *testing.T) *codexProjectModel {
	t.Helper()
	m := &codexProjectModel{}
	m.server = httptest.NewServer(http.HandlerFunc(m.respond))
	t.Cleanup(m.server.Close)
	return m
}

func (m *codexProjectModel) begin(wantCall bool) *codexProjectModelStage {
	m.mu.Lock()
	defer m.mu.Unlock()
	stage := &codexProjectModelStage{wantCall: wantCall, done: make(chan struct{})}
	m.current = stage
	return stage
}

func (m *codexProjectModel) respond(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || r.URL.Path != "/v1/responses" {
		http.NotFound(w, r)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, (1<<20)+1))
	if err != nil || len(body) > 1<<20 {
		http.Error(w, "invalid bounded model request", http.StatusRequestEntityTooLarge)
		return
	}
	var input struct {
		Tools []struct {
			Name string `json:"name"`
		} `json:"tools"`
		Input []json.RawMessage `json:"input"`
	}
	if json.Unmarshal(body, &input) != nil || len(input.Tools) > 128 || len(input.Input) > 128 {
		http.Error(w, "invalid model request shape", http.StatusBadRequest)
		return
	}
	advertised := false
	for _, tool := range input.Tools {
		if tool.Name == codexForeignProjectTool {
			advertised = true
		}
	}
	resultSeen, wrongResult := false, false
	for _, raw := range input.Input {
		var item struct {
			Type   string          `json:"type"`
			CallID string          `json:"call_id"`
			Output json.RawMessage `json:"output"`
		}
		if json.Unmarshal(raw, &item) != nil {
			http.Error(w, "invalid input item", http.StatusBadRequest)
			return
		}
		if item.Type == "function_call_output" {
			if item.CallID == codexForeignProjectCallID && bytes.Contains(item.Output, []byte("uap-pong")) {
				resultSeen = true
			} else {
				wrongResult = true
			}
		}
	}

	m.mu.Lock()
	stage := m.current
	if stage == nil {
		m.mu.Unlock()
		http.Error(w, "no active model stage", http.StatusConflict)
		return
	}
	stage.requests++
	if advertised {
		stage.advertised++
	}
	if wrongResult {
		stage.invalid = append(stage.invalid, "wrong function_call_output")
	}
	sendCall := false
	if stage.requests > 8 {
		stage.invalid = append(stage.invalid, "model request budget exceeded")
	} else if stage.wantCall {
		switch {
		case resultSeen:
			stage.results++
		case advertised && stage.issued == 0:
			stage.issued++
			sendCall = true
		case !advertised:
			stage.invalid = append(stage.invalid, "native Codex did not advertise foreign project tool")
		default:
			stage.invalid = append(stage.invalid, "model requested again without MCP result")
		}
	} else if advertised || resultSeen {
		stage.invalid = append(stage.invalid, "project B received project A tool or result")
	}
	m.mu.Unlock()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	emit := func(event string, value any) {
		data, _ := json.Marshal(value)
		_, _ = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, data)
	}
	emit("response.created", map[string]any{"type": "response.created", "response": map[string]string{"id": "resp_uap_codex_project"}})
	if sendCall {
		emit("response.output_item.done", map[string]any{"type": "response.output_item.done", "item": map[string]string{
			"type": "function_call", "call_id": codexForeignProjectCallID,
			"name": codexForeignProjectTool, "arguments": "{}",
		}})
	} else {
		emit("response.output_item.done", map[string]any{"type": "response.output_item.done", "item": map[string]any{
			"type": "message", "id": "msg_uap_codex_project", "role": "assistant",
			"content": []any{map[string]string{"type": "output_text", "text": "Project MCP check completed."}},
		}})
	}
	emit("response.completed", map[string]any{"type": "response.completed", "response": map[string]any{
		"id": "resp_uap_codex_project",
		"usage": map[string]int{"input_tokens": 1, "output_tokens": 1, "total_tokens": 2},
	}})
	if flush, ok := w.(http.Flusher); ok {
		flush.Flush()
	}
	if !sendCall {
		stage.once.Do(func() { close(stage.done) })
	}
}

func (m *codexProjectModel) verify(t *testing.T, stage *codexProjectModelStage) {
	t.Helper()
	select {
	case <-stage.done:
	case <-time.After(45 * time.Second):
		t.Fatal("real Codex model turn did not finish within bounded fixture deadline")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(stage.invalid) != 0 || stage.requests < 1 || stage.requests > 8 {
		t.Fatalf("invalid Codex model requests: total=%d kinds=%v", stage.requests, stage.invalid)
	}
	if stage.wantCall {
		if stage.advertised == 0 || stage.issued != 1 || stage.results != 1 {
			t.Fatalf("native Codex did not call and return foreign MCP tool: advertised=%d calls=%d results=%d", stage.advertised, stage.issued, stage.results)
		}
	} else if stage.advertised != 0 || stage.issued != 0 || stage.results != 0 {
		t.Fatalf("project B leaked foreign MCP: advertised=%d calls=%d results=%d", stage.advertised, stage.issued, stage.results)
	}
}

func codexProjectModelSession(t *testing.T, f *codexProjectNativeFixture, model *codexProjectModel, project, label string, wantCall bool) {
	t.Helper()
	stage := model.begin(wantCall)
	selected := *f.base
	selected.Project = project
	r := startNativeRPC(t, &selected, f.client, label)
	defer r.close()
	r.must(t, "initialize", map[string]any{"clientInfo": map[string]string{"name": "uap_codex_project_model", "version": "1.0.0"}, "capabilities": map[string]any{"experimentalApi": true}})
	if err := json.NewEncoder(r.in).Encode(map[string]any{"method": "initialized"}); err != nil {
		t.Fatal(err)
	}
	var thread struct {
		Thread struct {
			ID string `json:"id"`
		} `json:"thread"`
	}
	if err := json.Unmarshal(r.must(t, "thread/start", map[string]any{
		"cwd": project, "ephemeral": true, "approvalPolicy": "never",
		"sandbox": "read-only", "model": "fixture-model", "modelProvider": "fixture",
	}), &thread); err != nil || thread.Thread.ID == "" {
		t.Fatalf("native Codex project thread startup: %v", err)
	}
	r.must(t, "turn/start", map[string]any{"threadId": thread.Thread.ID,
		"input": []any{map[string]any{"type": "text", "text": "Call uap_ping from foreign_fixture once if available, then finish.", "text_elements": []any{}}},
	})
	model.verify(t, stage)
	r.close()
	f.requireForeign(t)
}

func codexProjectMCPEventCount(t *testing.T, path string) (int, []byte) {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil || len(body) > 4096 {
		t.Fatalf("Codex foreign MCP evidence absent or unbounded: %v", err)
	}
	calls, init, list := 0, 0, 0
	for _, method := range strings.Fields(string(body)) {
		switch method {
		case "initialize":
			init++
		case "tools/list":
			list++
		case "tools/call":
			calls++
		}
	}
	if init < 1 || list < 1 || len(strings.Fields(string(body))) > 36 {
		t.Fatalf("real foreign MCP handshake missing or unbounded: init=%d list=%d", init, list)
	}
	return calls, body
}

func TestAgentpluginsCodexNativeProjectModelForeignMCP(t *testing.T) {
	if os.Getenv("AGENTPLUGINS_CODEX_PROJECT_NATIVE_E2E") != "1" {
		t.Skip("source-only pinned Codex project model tool-call test")
	}
	nativeRequireDisposable(t)
	if _, err := os.Lstat("/etc/codex"); !os.IsNotExist(err) {
		t.Fatal("disposable host must have no system Codex config")
	}
	f := &codexProjectNativeFixture{base: newNativeFixture(t, true),
		client: nativeBinary(t, "AGENTPLUGINS_CODEX_BIN"),
		installer: nativeBinary(t, "AGENTPLUGINS_INSTALLER_BIN"),
		unchanged: map[string]string{},
	}
	if want := os.Getenv("AGENTPLUGINS_CODEX_SHA256"); len(want) != 64 || nativeSHA(t, f.client) != want {
		t.Fatal("pinned native Codex binary mismatch")
	}
	f.base.ClientBinDir, f.otherProject = filepath.Dir(f.client), filepath.Join(f.base.Root, "project-B")
	for _, project := range []string{f.base.Project, f.otherProject} {
		if err := os.MkdirAll(filepath.Join(project, ".git"), 0700); err != nil {
			t.Fatal(err)
		}
	}
	model := newCodexProjectModel(t)
	config := fmt.Sprintf("cli_auth_credentials_store = \"file\"\nmcp_oauth_credentials_store = \"file\"\nmodel_provider = \"fixture\"\nmodel = \"fixture-model\"\n[model_providers.fixture]\nname = \"Fixture\"\nbase_url = %q\nwire_api = \"responses\"\nrequires_openai_auth = false\n[analytics]\nenabled = false\n[feedback]\nenabled = false\n[projects.%q]\ntrust_level = \"trusted\"\n[projects.%q]\ntrust_level = \"trusted\"\n", model.server.URL+"/v1", f.base.Project, f.otherProject)
	f.foreign(t, filepath.Join(f.base.CodexHome, "config.toml"), config)
	f.foreign(t, filepath.Join(f.otherProject, ".codex", "config.toml"), "# project B is foreign\nmodel = 'fixture-model'\n")
	f.foreign(t, filepath.Join(f.base.Project, "foreign-keep.txt"), "keep foreign A bytes\n")
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	events := filepath.Join(f.base.Root, "codex-foreign-mcp-events.log")
	foreign := fmt.Sprintf("# foreign MCP belongs to the user, not UAP\n[mcp_servers.foreign_fixture]\ncommand = %q\nargs = [\"-test.run=^TestOpenCodeV2MCPStdioResponder$\"]\n[mcp_servers.foreign_fixture.env]\nUAP_OPENCODE_V2_MCP_HELPER = \"1\"\nUAP_OPENCODE_V2_MCP_EVENTS = %q\n", executable, events)
	projectConfig := filepath.Join(f.base.Project, ".codex", "config.toml")
	nativeWrite(t, projectConfig, []byte(foreign), 0600)
	foreignHash := nativeSHA(t, projectConfig)
	f.base.writePackage(t, "A", "1.0.0")

	stages := map[string]nativeStage{
		"install_preserved_foreign": {Status: "not_evaluated", Reason: "not reached"},
		"model_native_tool_before": {Status: "not_evaluated", Reason: "not reached"},
		"project_B_no_leak": {Status: "not_evaluated", Reason: "not reached"},
		"remove_foreign_retention": {Status: "not_evaluated", Reason: "not reached"},
		"model_native_tool_after": {Status: "not_evaluated", Reason: "not reached"},
	}
	evidence := f.evidence(t, stages)
	evidence["project_contract"] = "codex_trusted_foreign_project_model_mcp_v1"
	evidence["native_surface"] = "real pinned Codex model-driven foreign stdio MCP; scripted local Responses model, no vendor authentication"
	defer func() {
		f.writeEvidence(t, evidence)
		if err := nativeStageFailures(stages); err != nil {
			t.Error(err)
		}
	}()
	added := f.run(t, "codex-model-foreign-add", "add", f.base.Package, "--no-save")
	codexNativeProjectPrepared(t, added)
	codexProjectMCPBinding(t, f)
	current, err := os.ReadFile(projectConfig)
	if err != nil || !bytes.Contains(current, []byte("[mcp_servers.foreign_fixture]")) || !bytes.Contains(current, []byte(executable)) {
		t.Fatal("UAP add lost original foreign project MCP")
	}
	stages["install_preserved_foreign"] = nativeStage{Status: "passed", Reason: "project add retained foreign project MCP alongside UAP-managed MCP, with user/B isolation"}

	codexProjectModelSession(t, f, model, f.base.Project, "codex-model-before-remove-A", true)
	beforeCalls, beforeB := codexProjectMCPEventCount(t, events)
	if beforeCalls != 1 {
		t.Fatalf("real Codex before-remove MCP tools/call count=%d", beforeCalls)
	}
	stages["model_native_tool_before"] = nativeStage{Status: "passed", Reason: "native model turn chose exact foreign MCP tool; real child called and uap-pong returned to local Responses provider"}
	codexProjectModelSession(t, f, model, f.otherProject, "codex-model-control-B", false)
	afterB, err := os.ReadFile(events)
	if err != nil || !bytes.Equal(afterB, beforeB) {
		t.Fatal("control project B connected or executed project A foreign MCP")
	}
	stages["project_B_no_leak"] = nativeStage{Status: "passed", Reason: "trusted B model turn did not discover or execute foreign project A MCP"}
	f.run(t, "codex-model-foreign-remove", "remove", "native-proof")
	if got := nativeSHA(t, projectConfig); got != foreignHash {
		t.Fatal("UAP removal did not restore exact foreign project MCP TOML bytes")
	}
	f.requireForeign(t)
	stages["remove_foreign_retention"] = nativeStage{Status: "passed", Reason: "remove restored byte-identical foreign MCP config without modifying user or project B"}

	codexProjectModelSession(t, f, model, f.base.Project, "codex-model-after-remove-A", true)
	afterCalls, _ := codexProjectMCPEventCount(t, events)
	if afterCalls != 2 {
		t.Fatalf("foreign MCP stopped working after UAP removal: tools/call=%d", afterCalls)
	}
	stages["model_native_tool_after"] = nativeStage{Status: "passed", Reason: "fresh native Codex model turn called untouched foreign MCP after UAP removal and returned uap-pong"}
}
