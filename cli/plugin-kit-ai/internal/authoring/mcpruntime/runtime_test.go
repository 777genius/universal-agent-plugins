package mcpruntime

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
	"strings"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/project"
	processadapter "github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/process"
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
	if containmentErr != nil {
		if code(err) != "runtime_process_containment_unavailable" || !ev.Cleanup {
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
		root, service, p := writeProject(t, map[string]any{"type": "streamable-http", "url": rawURL}, nil)
		_, err := Run(context.Background(), Options{SourceRoot: root, Scratch: service.Scratch, Project: p, Server: "selected", AllowNetwork: true, Deadline: time.Second, Projects: service})
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

func TestInitializeRequiresNegotiatedProtocolVersion(t *testing.T) {
	for _, version := range []string{"", "2024-11-05", "future"} {
		body := json.RawMessage(fmt.Sprintf(`{"protocolVersion":%q,"capabilities":{},"serverInfo":{"name":"x","version":"1"}}`, version))
		if validInitialize(body) {
			t.Fatalf("accepted protocol version %q", version)
		}
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
		case "tools/list":
			result = map[string]any{"tools": []any{}}
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
	if err != nil || ev.Transport != "streamable_http" || !ev.Initialize || !ev.ListTools || !ev.Cleanup {
		t.Fatalf("HTTP evidence=%+v err=%v", ev, err)
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
	if code(err) != "runtime_deadline_exceeded" {
		t.Fatalf("deadline result: %v", err)
	}
	entries, readErr := os.ReadDir(service.Scratch)
	if readErr != nil || len(entries) != 0 {
		t.Fatalf("deadline retained private roots: %v %v", entries, readErr)
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
