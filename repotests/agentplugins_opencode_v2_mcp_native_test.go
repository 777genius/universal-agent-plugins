package pluginkitairepo_test

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The helper is the real stdio MCP server child started by the pinned OpenCode
// executable. No mocked OpenCode output and no vendor/provider credentials.
func TestOpenCodeV2MCPStdioResponder(t *testing.T) {
	if os.Getenv("UAP_OPENCODE_V2_MCP_HELPER") != "1" {
		return
	}
	events := os.Getenv("UAP_OPENCODE_V2_MCP_EVENTS")
	if events == "" || !filepath.IsAbs(events) {
		os.Exit(2)
	}
	reader := bufio.NewScanner(os.Stdin)
	reader.Buffer(make([]byte, 4096), 1<<20)
	writer := bufio.NewWriter(os.Stdout)
	count := 0
	for reader.Scan() {
		count++
		if count > 24 {
			os.Exit(2)
		}
		var request struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
		}
		if json.Unmarshal(reader.Bytes(), &request) != nil || request.Method == "" {
			os.Exit(2)
		}
		if err := appendOpenCodeV2MCPEvent(events, request.Method); err != nil {
			os.Exit(2)
		}
		if len(request.ID) == 0 {
			if request.Method != "notifications/initialized" {
				os.Exit(2)
			}
			continue
		}
		var result any
		switch request.Method {
		case "initialize":
			result = map[string]any{
				"protocolVersion": "2025-11-25",
				"capabilities": map[string]any{"tools": map[string]any{}},
				"serverInfo": map[string]string{"name": "uap-native-mcp-fixture", "version": "1.0.0"},
			}
		case "tools/list":
			result = map[string]any{"tools": []any{
				map[string]any{"name": "uap_ping", "description": "Inert project MCP check",
					"inputSchema": map[string]any{"type": "object", "properties": map[string]any{}}},
			}}
		case "ping":
			result = map[string]any{}
		case "tools/call":
			// This endpoint exists for later model-driven qualification. The
			// current test must not misreport an unobserved tools/call as PASS.
			result = map[string]any{"content": []any{map[string]string{"type": "text", "text": "uap-pong"}}}
		default:
			os.Exit(2)
		}
		reply, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": request.ID, "result": result})
		if err != nil {
			os.Exit(2)
		}
		if _, err := writer.Write(append(reply, '\n')); err != nil {
			os.Exit(2)
		}
		if writer.Flush() != nil {
			os.Exit(2)
		}
	}
	if reader.Err() != nil {
		os.Exit(2)
	}
	os.Exit(0)
}

func appendOpenCodeV2MCPEvent(path, method string) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	_, writeErr := fmt.Fprintln(f, method)
	closeErr := f.Close()
	if writeErr != nil {
		return writeErr
	}
	return closeErr
}

func TestAgentpluginsOpenCodeV2NativeProjectMCPHandshake(t *testing.T) {
	if os.Getenv("AGENTPLUGINS_OPENCODE_PROJECT_NATIVE_E2E") != "1" ||
		os.Getenv("AGENTPLUGINS_OPENCODE_VERSION") != "2.2.5" {
		t.Skip("requires explicit pinned V2 native project selection")
	}
	nativeRequireDisposable(t)
	client := os.Getenv("AGENTPLUGINS_OPENCODE_BIN")
	if client == "" || nativeSHA(t, client) != os.Getenv("AGENTPLUGINS_OPENCODE_SHA256") {
		t.Fatal("native MCP test requires exact pinned OpenCode binary")
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	home := filepath.Join(root, "home")
	userConfig := filepath.Join(root, "xdg-config", "opencode")
	projectA := filepath.Join(root, "project-A")
	projectB := filepath.Join(root, "project-B")
	for _, dir := range []string{home, userConfig, projectA, projectB, filepath.Join(root, "tmp")} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	events := filepath.Join(root, "observed-mcp-methods.log")
	config := map[string]any{
		"mcp": map[string]any{
			"servers": map[string]any{
				"uap_project_fixture": map[string]any{
					"type": "local",
					"command": []string{executable, "-test.run=^TestOpenCodeV2MCPStdioResponder$"},
					"environment": map[string]string{
						"UAP_OPENCODE_V2_MCP_HELPER": "1",
						"UAP_OPENCODE_V2_MCP_EVENTS": events,
					},
					"disabled": false,
					"protocol": "legacy",
				},
			},
		},
	}
	data, err := json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	configuration := filepath.Join(projectA, "opencode.json")
	if err := os.WriteFile(configuration, data, 0600); err != nil {
		t.Fatal(err)
	}
	baseline := sha256.Sum256(data)
	env := append(nativePlatformEnvironment(root, home, filepath.Dir(client)),
		"UAP_OPENCODE_V2_MCP_HELPER=", "UAP_OPENCODE_V2_MCP_EVENTS=")
	observe := func(project string) string {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 32*time.Second)
		defer cancel()
		result := captureProjectNative(ctx, client, project, env, []string{"mcp", "list", "--standalone"}, 1<<20, 30*time.Second)
		if result.Exit != 0 || result.Error != "" || result.Deadline ||
			result.StdoutLimit || result.StderrLimit {
			t.Fatalf("native V2 MCP list failed (exit=%d error_kind=%s)", result.Exit, result.ErrorKind)
		}
		return strings.TrimSpace(string(result.Stdout))
	}
	if output := observe(projectA); !strings.Contains(output, "uap_project_fixture") ||
		!strings.Contains(output, "connected") {
		t.Fatal("native V2 client failed to initialize project MCP fixture")
	}
	methodsRaw, err := os.ReadFile(events)
	if err != nil || len(methodsRaw) > 2048 {
		t.Fatal("native MCP event inventory absent or unbounded", err)
	}
	methods := strings.Fields(string(methodsRaw))
	counts := map[string]int{}
	for _, method := range methods {
		counts[method]++
		if counts[method] > 8 {
			t.Fatal("native MCP event repeated outside bounded lifecycle")
		}
	}
	if counts["initialize"] == 0 || counts["tools/list"] == 0 ||
		counts["tools/call"] != 0 || len(methods) > 24 {
		t.Fatal("native V2 MCP initialize/tools-list handshake was not exact")
	}
	before := append([]byte(nil), methodsRaw...)
	for _, project := range []string{projectB, projectA} {
		if project == projectA {
			if err := os.Remove(configuration); err != nil {
				t.Fatal(err)
			}
		}
		if output := observe(project); !strings.Contains(output, "No MCP servers configured") {
			t.Fatal("project B or removed project A leaked native MCP server")
		}
		after, err := os.ReadFile(events)
		if err != nil || !bytes.Equal(before, after) {
			t.Fatal("absent project opened original MCP server")
		}
	}
	current, err := os.ReadFile(filepath.Join(root, "xdg-config", "opencode", "opencode.json"))
	if err == nil || !os.IsNotExist(err) || len(current) != 0 {
		t.Fatal("MCP project E2E wrote to original user config")
	}
	t.Log("real pinned V2 project MCP initialize/tools-list confirmed; tools/call and vendor models unqualified")
}
