package mcpruntime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/project"
	processadapter "github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/process"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func writeProject(t *testing.T, server map[string]any, extras map[string]string) (string, project.Service, project.Result) {
	t.Helper()
	root, scratch := t.TempDir(), t.TempDir()
	write := func(name, body string) {
		t.Helper()
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	write("plugin.json", `{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"runtime-fixture"}`)
	mcp, err := json.Marshal(map[string]any{"$schema": "https://agent-plugins.org/schemas/1.0.0/mcp.schema.json", "mcpServers": map[string]any{"selected": server}})
	if err != nil {
		t.Fatal(err)
	}
	write("mcp.json", string(mcp))
	for name, body := range extras {
		write(name, body)
	}
	service := project.Service{Scratch: scratch}
	p, err := service.Read(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	return root, service, p
}

func TestStdioHandshakeToolAndPrivateEnvironment(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("Node is not installed")
	}
	containmentErr := (processadapter.OS{}).DuplexCapability()
	source := `import readline from 'node:readline';
const r=readline.createInterface({input:process.stdin});
for await(const line of r){const q=JSON.parse(line);if(q.method==='notifications/initialized')continue;let result;
if(q.method==='initialize')result={protocolVersion:'2025-06-18',capabilities:{tools:{}},serverInfo:{name:'x',version:'1'}};
if(q.method==='tools/list')result={tools:[{name:'echo',inputSchema:{type:'object'}}]};
if(q.method==='tools/call')result=process.env.PLUGIN_DATA&&process.env.PLUGIN_ROOT&&process.env.HOME.startsWith(process.env.PLUGIN_DATA)&&!process.env.GITHUB_TOKEN?{content:[]}:{isError:true};
process.stdout.write(JSON.stringify({jsonrpc:'2.0',id:q.id,result})+'\n');}`
	root, service, p := writeProject(t, map[string]any{"type": "stdio", "command": "node", "args": []any{"${PLUGIN_ROOT}/server.mjs"}}, map[string]string{"server.mjs": source, "fixture.json": `{"value":"not-reported"}`})
	t.Setenv("GITHUB_TOKEN", "must-not-be-inherited")
	ev, err := Run(context.Background(), Options{SourceRoot: root, Scratch: service.Scratch, Project: p, Server: "selected", Tool: "echo", Fixture: "fixture.json", Deadline: 5 * time.Second, Projects: service})
	if err != nil {
		if got := code(err); got != "runtime_stdio_containment_unavailable" && (containmentErr == nil || got != "runtime_process_containment_unavailable") || !ev.Cleanup {
			t.Fatalf("unavailable containment evidence=%+v err=%v capability=%v", ev, err, containmentErr)
		}
		entries, readErr := os.ReadDir(service.Scratch)
		if readErr != nil || len(entries) != 0 {
			t.Fatalf("private runtime root retained: %v %v", entries, readErr)
		}
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	if ev.Transport != "stdio" || !ev.Initialize || !ev.ListTools || !ev.ToolCall || !ev.Cleanup || ev.ToolCount != 1 {
		t.Fatalf("unexpected evidence: %+v", ev)
	}
	entries, err := os.ReadDir(service.Scratch)
	if err != nil || len(entries) != 0 {
		t.Fatalf("private runtime root retained: %v %v", entries, err)
	}
}

func TestStreamableHTTPRejectsReservedHeaders(t *testing.T) {
	for _, name := range []string{"Accept", "content-type", "MCP-SESSION-ID", "mcp-protocol-version"} {
		root, service, p := writeProject(t, map[string]any{"type": "streamable-http", "url": "https://example.invalid/mcp", "headers": map[string]any{name: "authored"}}, nil)
		_, err := Run(context.Background(), Options{SourceRoot: root, Scratch: service.Scratch, Project: p, Server: "selected", AllowNetwork: true, Deadline: time.Second, Projects: service})
		if code(err) != "runtime_http_config_invalid" {
			t.Fatalf("reserved header %q: %v", name, err)
		}
	}
}

func TestStreamableHTTPRejectsNonLoopbackCleartext(t *testing.T) {
	for _, rawURL := range []string{"http://example.com/mcp", "ftp://example.com/mcp", "https://example.com/mcp#fragment"} {
		err := runHTTP(context.Background(), domain.MCPServer{Decoded: map[string]any{"url": rawURL}}, "", nil, &Evidence{})
		if code(err) != "runtime_http_config_invalid" {
			t.Fatalf("URL %q: %v", rawURL, err)
		}
	}
}

func TestReadHTTPPayloadSelectsMatchingMultilineSSEEvent(t *testing.T) {
	body := "event: message\ndata: {\"jsonrpc\":\"2.0\",\"method\":\"notifications/progress\"}\n\n" +
		"event: message\ndata: {\"jsonrpc\":\"2.0\",\ndata: \"id\":2,\"result\":{}}\n\n"
	resp := &http.Response{Header: http.Header{"Content-Type": []string{"text/event-stream"}}, Body: io.NopCloser(strings.NewReader(body))}
	payload, err := readHTTPPayload(resp, 2)
	if err != nil || string(payload) != "{\"jsonrpc\":\"2.0\",\n\"id\":2,\"result\":{}}" {
		t.Fatalf("payload=%q err=%v", payload, err)
	}
}

func TestReadHTTPPayloadReturnsBeforeLongLivedSSEEOF(t *testing.T) {
	reader, writer := io.Pipe()
	defer reader.Close()
	release := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = io.WriteString(writer, "event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":7,\"result\":{}}\n\n")
		<-release
		_ = writer.Close()
	}()
	resp := &http.Response{Header: http.Header{"Content-Type": []string{"text/event-stream"}}, Body: reader}
	type result struct {
		payload []byte
		err     error
	}
	returned := make(chan result, 1)
	go func() {
		payload, err := readHTTPPayload(resp, 7)
		returned <- result{payload, err}
	}()
	select {
	case got := <-returned:
		if got.err != nil || !strings.Contains(string(got.payload), `"id":7`) {
			t.Errorf("payload=%q err=%v", got.payload, got.err)
		}
	case <-time.After(time.Second):
		t.Error("matching SSE event waited for response EOF")
	}
	close(release)
	<-done
}

func TestInitializeRequiresNegotiatedProtocolVersion(t *testing.T) {
	for _, version := range []string{"", "2024-11-05", "future"} {
		body := json.RawMessage(fmt.Sprintf(`{"protocolVersion":%q,"capabilities":{},"serverInfo":{"name":"x","version":"1"}}`, version))
		if validInitialize(body) {
			t.Fatalf("accepted protocol version %q", version)
		}
	}
}

func TestInitializeNegotiatesToolsCapability(t *testing.T) {
	without := json.RawMessage(`{"protocolVersion":"2025-06-18","capabilities":{},"serverInfo":{"name":"x","version":"1"}}`)
	if tools, valid := initializeCapabilities(without); tools || !valid {
		t.Fatalf("capabilities without tools = tools %v valid %v", tools, valid)
	}
	with := json.RawMessage(`{"protocolVersion":"2025-06-18","capabilities":{"tools":{"listChanged":true}},"serverInfo":{"name":"x","version":"1"}}`)
	if tools, valid := initializeCapabilities(with); !tools || !valid {
		t.Fatalf("tools capability = tools %v valid %v", tools, valid)
	}
	for _, malformed := range []string{"null", "true", `[]`} {
		body := json.RawMessage(`{"protocolVersion":"2025-06-18","capabilities":{"tools":` + malformed + `},"serverInfo":{"name":"x","version":"1"}}`)
		if _, valid := initializeCapabilities(body); valid {
			t.Fatalf("accepted malformed tools capability %s", malformed)
		}
	}
	for _, body := range []string{
		`{"protocolVersion":"2025-06-18","protocolVersion":"2025-06-18","capabilities":{},"serverInfo":{"name":"x","version":"1"}}`,
		`{"protocolVersion":"2025-06-18","capabilities":{"tools":{"listChanged":"true"}},"serverInfo":{"name":"x","version":"1"}}`,
		`{"protocolVersion":"2025-06-18","capabilities":{"tools":{"listChanged":true,"listChanged":false}},"serverInfo":{"name":"x","version":"1"}}`,
		`{"protocolVersion":"2025-06-18","capabilities":{},"serverInfo":null}`,
		`{"protocolVersion":"2025-06-18","capabilities":{},"serverInfo":{"name":7,"version":"1"}}`,
	} {
		if _, valid := initializeCapabilities(json.RawMessage(body)); valid {
			t.Fatalf("accepted malformed initialize result %s", body)
		}
	}
	extensions := json.RawMessage(`{"protocolVersion":"2025-06-18","capabilities":{"tools":{"listChanged":false,"vendor.example":{"enabled":true}},"vendor.capability":{"mode":"test"}},"serverInfo":{"name":"x","version":"1","vendor.info":7},"vendor.top":[]}`)
	if tools, valid := initializeCapabilities(extensions); !tools || !valid {
		t.Fatal("permitted initialize extensions were rejected")
	}
}

func TestToolListRejectsMissingNullAndMalformedArrays(t *testing.T) {
	for _, body := range []string{
		`{}`, `{"tools":null}`, `{"tools":{}}`, `{"tools":7}`, `{"tools":[null]}`, `{"tools":[{}]}`,
		`{"tools":[{"name":"echo"}]}`,
		`{"tools":[{"name":"echo","inputSchema":null}]}`,
		`{"tools":[{"name":"echo","inputSchema":[]}]}`,
		`{"tools":[{"name":7,"inputSchema":{}}]}`,
		`{"tools":[{"name":"echo","inputSchema":{}},{"name":"echo","inputSchema":{}}]}`,
		`{"tools":[{"name":"echo","name":"other","inputSchema":{}}]}`,
		`{"tools":[{"name":"echo","inputSchema":{"type":"object","type":"array"}}]}`,
		`{"tools":[],"nextCursor":null}`,
		`{"tools":[],"nextCursor":7}`,
	} {
		if _, err := decodeToolList(json.RawMessage(body)); code(err) != "runtime_protocol_invalid" {
			t.Fatalf("tool list %s: %v", body, err)
		}
	}
	tools, err := decodeToolList(json.RawMessage(`{"tools":[{"name":"echo","description":"kept","inputSchema":{},"vendor.extension":true}],"nextCursor":""}`))
	if err != nil || len(tools) != 1 || tools[0].Name != "echo" {
		t.Fatalf("valid tool list rejected: %#v %v", tools, err)
	}
	if _, err := decodeToolList(json.RawMessage(`{"tools":[],"nextCursor":"more"}`)); code(err) != "runtime_tools_pagination_unsupported" {
		t.Fatalf("non-empty pagination cursor was not explicit: %v", err)
	}
}

func TestStreamableHTTPRequiresOptInAndRunsBoundedStages(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		var q struct {
			ID     int    `json:"id"`
			Method string `json:"method"`
		}
		if err := json.NewDecoder(r.Body).Decode(&q); err != nil {
			t.Error(err)
			return
		}
		if q.Method == "initialize" {
			if value := r.Header.Get("Mcp-Protocol-Version"); value != "" {
				t.Errorf("initialize unexpectedly carried protocol header %q", value)
			}
		} else if value := r.Header.Get("Mcp-Protocol-Version"); value != protocolVersion {
			t.Errorf("%s protocol header=%q", q.Method, value)
		}
		if q.ID == 0 {
			w.WriteHeader(http.StatusAccepted)
			return
		}
		var result any
		switch q.Method {
		case "initialize":
			result = map[string]any{"protocolVersion": "2025-06-18", "capabilities": map[string]any{}, "serverInfo": map[string]any{"name": "x", "version": "1"}}
		default:
			t.Errorf("unexpected method %s", q.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%d,"result":%s}`, q.ID, mustJSON(t, result))
	}))
	defer server.Close()
	root, service, p := writeProject(t, map[string]any{"type": "streamable-http", "url": server.URL}, nil)
	base := Options{SourceRoot: root, Scratch: service.Scratch, Project: p, Server: "selected", Deadline: 5 * time.Second, Projects: service}
	if _, err := Run(context.Background(), base); code(err) != "runtime_network_opt_in_required" {
		t.Fatalf("network was not denied: %v", err)
	}
	base.AllowNetwork = true
	ev, err := Run(context.Background(), base)
	if err != nil || ev.Transport != "streamable_http" || !ev.Initialize || ev.ListTools || !ev.Cleanup {
		t.Fatalf("HTTP evidence=%+v err=%v", ev, err)
	}
}

func TestStreamableHTTPListsOnlyAdvertisedToolsAndRejectsNullList(t *testing.T) {
	var methods []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		var q struct {
			ID     int    `json:"id"`
			Method string `json:"method"`
		}
		_ = json.NewDecoder(r.Body).Decode(&q)
		methods = append(methods, q.Method)
		if q.ID == 0 {
			w.WriteHeader(http.StatusAccepted)
			return
		}
		result := `{"tools":null}`
		if q.Method == "initialize" {
			result = `{"protocolVersion":"2025-06-18","capabilities":{"tools":{}},"serverInfo":{"name":"x","version":"1"}}`
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"jsonrpc":"2.0","id":%d,"result":%s}`, q.ID, result)
	}))
	defer server.Close()
	root, service, p := writeProject(t, map[string]any{"type": "streamable-http", "url": server.URL}, nil)
	ev, err := Run(context.Background(), Options{SourceRoot: root, Scratch: service.Scratch, Project: p, Server: "selected", AllowNetwork: true, Deadline: 5 * time.Second, Projects: service})
	if code(err) != "runtime_protocol_invalid" || !ev.Initialize || ev.ListTools || !ev.Cleanup {
		t.Fatalf("evidence=%+v err=%v", ev, err)
	}
	if got := strings.Join(methods, ","); got != "initialize,notifications/initialized,tools/list" {
		t.Fatalf("methods=%s", got)
	}
}

func TestToolFixturePairAndContainment(t *testing.T) {
	root, service, p := writeProject(t, map[string]any{"type": "stdio", "command": "node"}, nil)
	base := Options{SourceRoot: root, Scratch: service.Scratch, Project: p, Server: "selected", Deadline: time.Second, Projects: service}
	base.Tool = "x"
	if _, err := Run(context.Background(), base); code(err) != "runtime_tool_fixture_pair_required" {
		t.Fatalf("unpaired tool: %v", err)
	}
	base.Fixture = "../outside.json"
	if _, err := Run(context.Background(), base); code(err) != "runtime_fixture_outside_package" {
		t.Fatalf("fixture escaped: %v", err)
	}
}

func TestRestrictedEnvironmentCannotOverridePrivateRoots(t *testing.T) {
	for _, name := range []string{"HOME", "Path", "TMPDIR", "PLUGIN_DATA", "XDG_CONFIG_HOME"} {
		if _, err := restrictedEnv("/package", "/data", "/bin/node", map[string]any{name: "/outside"}); err == nil {
			t.Fatalf("reserved environment %q was accepted", name)
		}
	}
	env, err := restrictedEnv("/package", "/data", "/bin/node", map[string]any{"RUNTIME_MODE": "test"})
	if err != nil || len(env) != 8 || env[len(env)-1] != "RUNTIME_MODE=test" {
		t.Fatalf("safe authored environment rejected: %v %v", env, err)
	}
}

func TestSafeLinkRequiresContainedRelativeTarget(t *testing.T) {
	for _, tc := range []struct {
		name, target string
		want         bool
	}{
		{"node_modules/.bin/server", "../server/index.mjs", true},
		{"bin/server", "../../outside", false},
		{"bin/server", "/outside", false},
		{"bin/server", `..\outside`, false},
	} {
		if got := safeLink(tc.name, tc.target); got != tc.want {
			t.Fatalf("safeLink(%q, %q)=%v, want %v", tc.name, tc.target, got, tc.want)
		}
	}
}

func TestHTTPDeadlineCleansPrivateRoots(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-time.After(200 * time.Millisecond) }))
	defer server.Close()
	root, service, p := writeProject(t, map[string]any{"type": "streamable-http", "url": server.URL}, nil)
	_, err := Run(context.Background(), Options{SourceRoot: root, Scratch: service.Scratch, Project: p, Server: "selected", AllowNetwork: true, Deadline: 25 * time.Millisecond, Projects: service})
	if !hasErrorCode(err, "runtime_deadline_exceeded") {
		t.Fatalf("deadline result: %v", err)
	}
	entries, readErr := os.ReadDir(service.Scratch)
	if readErr != nil || len(entries) != 0 {
		t.Fatalf("deadline retained private roots: %v %v", entries, readErr)
	}
}

func TestLongLivedSSEHonorsDeadlineAndCleansPrivateRoots(t *testing.T) {
	requestCanceled := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		if flush, ok := w.(http.Flusher); ok {
			flush.Flush()
		}
		<-r.Context().Done()
		select {
		case <-requestCanceled:
		default:
			close(requestCanceled)
		}
	}))
	defer server.Close()
	root, service, p := writeProject(t, map[string]any{"type": "streamable-http", "url": server.URL}, nil)
	_, err := Run(context.Background(), Options{SourceRoot: root, Scratch: service.Scratch, Project: p, Server: "selected", AllowNetwork: true, Deadline: 50 * time.Millisecond, Projects: service})
	if !hasErrorCode(err, "runtime_deadline_exceeded") {
		t.Fatalf("deadline result: %v", err)
	}
	select {
	case <-requestCanceled:
	case <-time.After(time.Second):
		t.Fatal("SSE request context was not canceled")
	}
	if entries, readErr := os.ReadDir(service.Scratch); readErr != nil || len(entries) != 0 {
		t.Fatalf("deadline retained private roots: %v %v", entries, readErr)
	}
}

func TestLongLivedSSEHonorsCallerCancellationAndCleansPrivateRoots(t *testing.T) {
	requestStarted := make(chan struct{})
	requestCanceled := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		if flush, ok := w.(http.Flusher); ok {
			flush.Flush()
		}
		close(requestStarted)
		<-r.Context().Done()
		close(requestCanceled)
	}))
	defer server.Close()
	root, service, p := writeProject(t, map[string]any{"type": "streamable-http", "url": server.URL}, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	returned := make(chan error, 1)
	go func() {
		_, err := Run(ctx, Options{SourceRoot: root, Scratch: service.Scratch, Project: p, Server: "selected", AllowNetwork: true, Deadline: 5 * time.Second, Projects: service})
		returned <- err
	}()
	select {
	case <-requestStarted:
	case <-time.After(time.Second):
		t.Fatal("SSE request did not start")
	}
	cancel()
	select {
	case err := <-returned:
		if !hasErrorCode(err, "runtime_canceled") || !errors.Is(err, context.Canceled) {
			t.Fatalf("cancellation result: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("SSE request did not return after cancellation")
	}
	select {
	case <-requestCanceled:
	case <-time.After(time.Second):
		t.Fatal("SSE request context was not canceled")
	}
	if entries, readErr := os.ReadDir(service.Scratch); readErr != nil || len(entries) != 0 {
		t.Fatalf("cancellation retained private roots: %v %v", entries, readErr)
	}
}

func TestPrivateCopyPreservesConstructionAndCleanupFailures(t *testing.T) {
	root, service, p := writeProject(t, map[string]any{"type": "stdio", "command": "node"}, nil)
	p.Input.Inventory[0].Captured = false
	cleanupFailure := errors.New("synthetic cleanup failure")
	_, _, _, err := privateCopy(context.Background(), Options{SourceRoot: root, Scratch: service.Scratch, Project: p, Projects: service, removeAll: func(string) error { return cleanupFailure }})
	if !errors.Is(err, cleanupFailure) || !hasErrorCode(err, "runtime_cleanup_failed") || !hasErrorCode(err, "runtime_package_not_copyable") {
		t.Fatalf("construction/cleanup errors were not both preserved: %v", err)
	}
	if entries, readErr := os.ReadDir(service.Scratch); readErr != nil || len(entries) != 1 {
		t.Fatalf("failed cleanup fixture was not retained as expected: %v %v", entries, readErr)
	}
	entries, _ := os.ReadDir(service.Scratch)
	if len(entries) == 1 {
		_ = os.RemoveAll(filepath.Join(service.Scratch, entries[0].Name()))
	}
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
func code(err error) string {
	if value, ok := err.(*Error); ok {
		return value.Code
	}
	return ""
}

func hasErrorCode(err error, want string) bool {
	if err == nil {
		return false
	}
	if value, ok := err.(*Error); ok && value.Code == want {
		return true
	}
	if many, ok := err.(interface{ Unwrap() []error }); ok {
		for _, child := range many.Unwrap() {
			if hasErrorCode(child, want) {
				return true
			}
		}
	}
	if one, ok := err.(interface{ Unwrap() error }); ok {
		return hasErrorCode(one.Unwrap(), want)
	}
	return false
}
