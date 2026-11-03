package pluginkitairepo_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pelletier/go-toml/v2"
)

const codexForeignProjectNamespace = "mcp__foreign_fixture"
const codexForeignProjectTool = "uap_ping"
const codexForeignProjectCallID = "call_uap_codex_project_foreign"
const codexForeignProjectSearchID = "search_uap_codex_project_foreign"

// Model-visible MCP tools are namespaced, including after deferred search.
type codexProjectModelTool struct {
	Name  string `json:"name"`
	Type  string `json:"type"`
	Tools []struct {
		Name string `json:"name"`
	} `json:"tools"`
}

func codexProjectModelHasForeign(tools []codexProjectModelTool) bool {
	for _, tool := range tools {
		if tool.Type != "namespace" || tool.Name != codexForeignProjectNamespace {
			continue
		}
		for _, child := range tool.Tools {
			if child.Name == codexForeignProjectTool {
				return true
			}
		}
	}
	return false
}

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
	searches   int
	searchHits int
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
	reject := func(reason string, status int) {
		m.mu.Lock()
		if stage := m.current; stage != nil {
			stage.invalid = append(stage.invalid, reason)
			stage.once.Do(func() { close(stage.done) })
		}
		m.mu.Unlock()
		http.Error(w, reason, status)
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, (1<<20)+1))
	if err != nil || len(body) > 1<<20 {
		reject("invalid bounded model request", http.StatusRequestEntityTooLarge)
		return
	}
	var input struct {
		Tools []codexProjectModelTool `json:"tools"`
		Input []json.RawMessage       `json:"input"`
	}
	if json.Unmarshal(body, &input) != nil || len(input.Tools) > 128 || len(input.Input) > 128 {
		reject("invalid model request shape", http.StatusBadRequest)
		return
	}
	advertised := codexProjectModelHasForeign(input.Tools)
	searchAvailable := false
	for _, tool := range input.Tools {
		if tool.Type == "tool_search" {
			searchAvailable = true
		}
	}
	resultSeen, searchFound, wrongResult, wrongSearch := false, false, false, false
	var wrongResultDetail string
	for _, raw := range input.Input {
		var item struct {
			Type   string                  `json:"type"`
			CallID string                  `json:"call_id"`
			Output json.RawMessage         `json:"output"`
			Tools  []codexProjectModelTool `json:"tools"`
		}
		if json.Unmarshal(raw, &item) != nil {
			reject("invalid input item", http.StatusBadRequest)
			return
		}
		switch item.Type {
		case "function_call_output":
			if item.CallID == codexForeignProjectCallID && bytes.Contains(item.Output, []byte("uap-pong")) {
				resultSeen = true
			} else {
				wrongResult = true
				// Only in the disposable, credential-free failure fixture: retain
				// a bounded error hint to distinguish policy refusals from routing bugs.
				wrongResultDetail = string(item.Output)
				if len(wrongResultDetail) > 400 {
					wrongResultDetail = wrongResultDetail[:400] + "...(truncated)"
				}
			}
		case "tool_search_output":
			if item.CallID == codexForeignProjectSearchID && codexProjectModelHasForeign(item.Tools) {
				searchFound = true
			} else {
				wrongSearch = true
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
	if searchFound {
		stage.searchHits++
	}
	if wrongResult {
		stage.invalid = append(stage.invalid, "wrong function_call_output: "+wrongResultDetail)
	}
	if wrongSearch {
		stage.invalid = append(stage.invalid, "wrong tool_search_output")
	}
	sendSearch, sendCall := false, false
	switch {
	case stage.requests > 8:
		stage.invalid = append(stage.invalid, "model request budget exceeded")
	case !stage.wantCall:
		if advertised || searchFound || resultSeen {
			stage.invalid = append(stage.invalid, "project B received project A tool or result")
		}
	case resultSeen:
		if stage.issued != 1 {
			stage.invalid = append(stage.invalid, "result received without exactly one tool call")
		} else {
			stage.results++
		}
	case stage.issued > 0:
		stage.invalid = append(stage.invalid, "model requested again without MCP result")
	case advertised || (searchFound && stage.searches == 1):
		stage.issued++
		sendCall = true
	case stage.searches == 0 && searchAvailable:
		stage.searches++
		sendSearch = true
	default:
		stage.invalid = append(stage.invalid, "foreign project MCP not available from tool_search")
	}
	m.mu.Unlock()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	emit := func(event string, value any) {
		data, _ := json.Marshal(value)
		_, _ = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, data)
	}
	emit("response.created", map[string]any{"type": "response.created", "response": map[string]string{"id": "resp_uap_codex_project"}})
	if sendSearch {
		emit("response.output_item.done", map[string]any{"type": "response.output_item.done", "item": map[string]any{
			"type": "tool_search_call", "call_id": codexForeignProjectSearchID,
			"execution": "client",
			"arguments": map[string]any{"query": "foreign_fixture uap_ping project MCP", "limit": 8},
		}})
	} else if sendCall {
		emit("response.output_item.done", map[string]any{"type": "response.output_item.done", "item": map[string]string{
			"type": "function_call", "call_id": codexForeignProjectCallID,
			"namespace": codexForeignProjectNamespace, "name": codexForeignProjectTool, "arguments": "{}",
		}})
	} else {
		emit("response.output_item.done", map[string]any{"type": "response.output_item.done", "item": map[string]any{
			"type": "message", "id": "msg_uap_codex_project", "role": "assistant",
			"content": []any{map[string]string{"type": "output_text", "text": "Project MCP check completed."}},
		}})
	}
	emit("response.completed", map[string]any{"type": "response.completed", "response": map[string]any{
		"id":    "resp_uap_codex_project",
		"usage": map[string]int{"input_tokens": 1, "output_tokens": 1, "total_tokens": 2},
	}})
	if flush, ok := w.(http.Flusher); ok {
		flush.Flush()
	}
	if !sendCall && !sendSearch {
		stage.once.Do(func() { close(stage.done) })
	}
}

func TestCodexProjectModelDeferredMCPToolSearch(t *testing.T) {
	m := &codexProjectModel{}
	stage := m.begin(true)
	send := func(body string) string {
		t.Helper()
		response := httptest.NewRecorder()
		m.respond(response, httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/responses", strings.NewReader(body)))
		if response.Code != http.StatusOK {
			t.Fatalf("fixture rejected valid Responses request: %d", response.Code)
		}
		return response.Body.String()
	}
	first := send(`{"tools":[{"type":"tool_search","name":"tool_search"}],"input":[]}`)
	if !strings.Contains(first, `"tool_search_call"`) || !strings.Contains(first, codexForeignProjectSearchID) {
		t.Fatal("first Responses turn did not request real Codex deferred MCP search")
	}
	second := send(`{"tools":[{"type":"tool_search","name":"tool_search"}],"input":[{"type":"tool_search_output","call_id":"search_uap_codex_project_foreign","tools":[{"type":"namespace","name":"mcp__foreign_fixture","tools":[{"type":"function","name":"uap_ping"}]}]}]}`)
	if !strings.Contains(second, `"function_call"`) || !strings.Contains(second, `"namespace":"mcp__foreign_fixture"`) {
		t.Fatal("pinned namespaced MCP function call missing after tool_search result")
	}
	third := send(`{"tools":[{"type":"tool_search","name":"tool_search"}],"input":[{"type":"function_call_output","call_id":"call_uap_codex_project_foreign","output":"uap-pong"}]}`)
	if !strings.Contains(third, `"message"`) {
		t.Fatal("function call result did not complete model turn")
	}
	m.verify(t, stage)
}

func TestCodexProjectModelRejectsMalformedRequests(t *testing.T) {
	for _, test := range []struct {
		name, body, reason string
		status             int
	}{
		{"too large", strings.Repeat("a", (1<<20)+1), "invalid bounded model request", http.StatusRequestEntityTooLarge},
		{"invalid JSON", "{", "invalid model request shape", http.StatusBadRequest},
		{"invalid input", "{\"input\":[42]}", "invalid input item", http.StatusBadRequest},
	} {
		t.Run(test.name, func(t *testing.T) {
			m := &codexProjectModel{}
			stage := m.begin(true)
			request := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/v1/responses", strings.NewReader(test.body))
			response := httptest.NewRecorder()
			m.respond(response, request)
			if response.Code != test.status || len(stage.invalid) != 1 || stage.invalid[0] != test.reason {
				t.Fatalf("scripted model rejection not recorded: status=%d reasons=%v", response.Code, stage.invalid)
			}
			select {
			case <-stage.done:
			default:
				t.Fatal("scripted model request rejection left E2E waiting")
			}
		})
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
		if (stage.advertised == 0 && stage.searchHits == 0) || stage.issued != 1 || stage.results != 1 || stage.searches > 1 {
			t.Fatalf("native Codex did not call and return foreign MCP tool: advertised=%d searches=%d hits=%d calls=%d results=%d", stage.advertised, stage.searches, stage.searchHits, stage.issued, stage.results)
		}
	} else if stage.advertised != 0 || stage.issued != 0 || stage.results != 0 {
		t.Fatalf("project B leaked foreign MCP: advertised=%d calls=%d results=%d", stage.advertised, stage.issued, stage.results)
	}
}

// Native MCP startup is asynchronous; qualify only connected servers with an
// exact declared tool before a model turn, never an inferred configuration.
func codexProjectAwaitModelMCP(t *testing.T, r *nativeRPC, threadID string, wantForeign, wantManaged bool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(r.ctx, 30*time.Second)
	defer cancel()
	expected := map[string]string{}
	if wantForeign {
		expected["foreign_fixture"] = "uap_ping"
		if wantManaged {
			expected["default"] = "inspect_runtime"
			expected["explicit"] = "inspect_runtime"
		}
	}
	for {
		raw, err := r.requestContext(ctx, "mcpServerStatus/list", map[string]any{"threadId": threadID, "limit": 100})
		if err != nil {
			t.Fatal(err)
		}
		var inventory codexProjectMCPInventory
		if err := json.Unmarshal(raw, &inventory); err != nil || inventory.NextCursor != nil {
			t.Fatalf("incomplete native Codex MCP inventory: %s (%v)", raw, err)
		}
		if len(expected) == 0 {
			if len(inventory.Data) != 0 {
				t.Fatalf("control project discovered foreign MCP: %+v", inventory)
			}
			return
		}
		ready := len(inventory.Data) == len(expected)
		seen := map[string]bool{}
		for _, server := range inventory.Data {
			tool, valid := expected[server.Name]
			if !valid || server.PluginID != "" || seen[server.Name] {
				t.Fatalf("unexpected/duplicate project MCP server: %+v", server)
			}
			seen[server.Name] = true
			switch server.RuntimeStatus {
			case "starting":
				ready = false
			case "connected":
				if len(server.Tools) != 1 || server.Tools[tool] == nil {
					t.Fatalf("connected MCP lacks exact native tool: %+v", server)
				}
			default:
				t.Fatalf("native project MCP did not connect: %+v", server)
			}
		}
		if ready {
			return
		}
		timer := time.NewTimer(60 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			t.Fatalf("native MCP did not reach connected state: %v", ctx.Err())
		case <-timer.C:
		}
	}
}

func codexProjectModelSession(t *testing.T, f *codexProjectNativeFixture, model *codexProjectModel, project, label string, wantCall, wantManaged bool) {
	t.Helper()
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
	codexProjectAwaitModelMCP(t, r, thread.Thread.ID, wantCall, wantManaged)
	stage := model.begin(wantCall)
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

// Only UAP-owned MCP entries may change. The native TOML adapter preserves
// foreign values and comments, but deliberately does not promise that editing
// and later removing sections reconstructs every whitespace byte of the file.
func codexAssertForeignProjectMCP(t *testing.T, path, original string, managed bool) {
	t.Helper()
	current, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	const foreignComment = "# foreign MCP belongs to the user, not UAP"
	if !bytes.Contains(current, []byte(foreignComment)) {
		t.Fatal("Codex project configuration lost foreign MCP ownership comment")
	}
	var before, after map[string]any
	if err := toml.Unmarshal([]byte(original), &before); err != nil {
		t.Fatal(err)
	}
	if err := toml.Unmarshal(current, &after); err != nil {
		t.Fatal(err)
	}
	servers, ok := after["mcp_servers"].(map[string]any)
	if !ok {
		t.Fatal("Codex project configuration lost MCP servers table")
	}
	for _, name := range []string{"default", "explicit"} {
		_, present := servers[name]
		if present != managed {
			t.Fatalf("UAP-owned Codex MCP entry %q present=%t, want=%t", name, present, managed)
		}
		delete(servers, name)
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatal("Codex project configuration changed foreign TOML values")
	}
}
