package providers

import (
	"encoding/json"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers/nativeconfig"
	"os"
	"path/filepath"
	"testing"
)

func TestOpenCodeDeclaredTransportDoesNotSilentlyConvertSSE(t *testing.T) {
	server := domain.MCPServer{Type: "sse", Decoded: map[string]any{"url": "https://example.test/sse"}}
	if _, err := neutralOpenCodeServer(server); err == nil {
		t.Fatal("SSE silently mapped to HTTP-first remote")
	}
	server.Type = "streamable-http"
	server.Decoded["url"] = "https://example.test/${PLUGIN_ROOT}"
	server.Decoded["headers"] = map[string]any{"Authorization": "Bearer ${PLUGIN_DATA}"}
	neutral, err := neutralOpenCodeServer(server)
	if err != nil {
		t.Fatal(err)
	}
	if neutral.URL != server.Decoded["url"] || neutral.Headers["Authorization"] != "Bearer ${PLUGIN_DATA}" || neutral.RemoteTransport != "" {
		t.Fatalf("remote values changed: %+v", neutral)
	}
	p := filepath.Join(t.TempDir(), "opencode.json")
	if _, err := nativeconfig.New().Apply(nativeconfig.Request{Paths: nativeconfig.Paths{JSON: p}, Codec: nativeconfig.CodecOpenCode, Action: nativeconfig.ActionAdd, Name: "http", Server: neutral}); err != nil {
		t.Fatal(err)
	}
	neutral.RemoteTransport = "sse"
	if _, err := nativeconfig.DesiredReceipt(p, nativeconfig.CodecOpenCode, "sse", neutral, nativeconfig.Placeholders{}); err == nil {
		t.Fatal("invented native transport selector accepted")
	}
}

func TestOpenCodeResolvedSymlinkCWDRoundTripIsNotExpandedAgain(t *testing.T) {
	root := t.TempDir()
	configRoot := filepath.Join(root, "opencode")
	active := filepath.Join(root, "managed", "demo")
	data := filepath.Join(root, "data")
	envelope, plan := openCodeTestPackage(t, active, configRoot, "owned")
	target := filepath.Join(data, "${PLUGIN_ROOT}")
	if err := os.MkdirAll(target, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("${PLUGIN_ROOT}", filepath.Join(data, "link")); err != nil {
		t.Fatal(err)
	}
	envelope.MCP.Servers["docs"] = domain.MCPServer{Name: "docs", Type: "stdio", Decoded: map[string]any{"command": "node", "cwd": "${PLUGIN_DATA}/link", "args": []any{"${PLUGIN_ROOT}/server.js", "${UNKNOWN}"}, "env": map[string]any{"DATA": "${PLUGIN_DATA}", "OTHER": "${HOME}"}}}
	if err := os.Remove(filepath.Join(active, openCodeProjectionFile)); err != nil {
		t.Fatal(err)
	}
	if err := projectOpenCodeNative(active, envelope, plan, data); err != nil {
		t.Fatal(err)
	}
	projection, err := readOpenCodeProjection(active)
	if err != nil {
		t.Fatal(err)
	}
	canonical, err := filepath.EvalSymlinks(target)
	if err != nil {
		t.Fatal(err)
	}
	server := projection.MCPServers["docs"]
	observed, err := filepath.EvalSymlinks(server.CWD)
	if err != nil {
		t.Fatal(err)
	}
	if observed != canonical || server.CWD != target || !server.CWDResolved || server.StdioValuesResolved {
		t.Fatalf("wrong resolved boundary: %+v", server)
	}
	objects, err := buildOpenCodeNativeObjects(active, envelope, plan)
	if err != nil {
		t.Fatal(err)
	}
	if err := applyOpenCodeNative(configRoot, active, nil, objects); err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(readOpenCodeTestFile(t, filepath.Join(configRoot, "opencode.json"))), &doc); err != nil {
		t.Fatal(err)
	}
	entry := doc["mcp"].(map[string]any)["docs"].(map[string]any)
	if entry["cwd"] != server.CWD || entry["command"].([]any)[1] != filepath.Join(active, "server.js") || entry["command"].([]any)[2] != "${UNKNOWN}" || entry["environment"].(map[string]any)["DATA"] != data || entry["environment"].(map[string]any)["OTHER"] != "${HOME}" {
		t.Fatalf("second expansion or lost raw args/env: %#v", entry)
	}
	if _, ok := entry["resolved_cwd"]; ok {
		t.Fatal("private projection metadata leaked to native config")
	}
}

func TestOpenCodeHistoricalRawCWDProjectionStillExpandsOnce(t *testing.T) {
	root := t.TempDir()
	config := filepath.Join(root, "opencode.json")
	projection := openCodeProjection{Version: 1, ConfigPath: config, ConfigJSON: config, ConfigJSONC: filepath.Join(root, "opencode.jsonc"), PackageRoot: root, MCPServers: map[string]nativeconfig.Server{"local": {Type: "stdio", Command: "node", CWD: "${PLUGIN_ROOT}/work"}}}
	body, err := json.Marshal(projection)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, openCodeProjectionFile), body, 0600); err != nil {
		t.Fatal(err)
	}
	loaded, err := readOpenCodeProjection(root)
	if err != nil {
		t.Fatal(err)
	}
	server := loaded.MCPServers["local"]
	if server.CWDResolved {
		t.Fatal("historical raw cwd marked resolved")
	}
	if _, err := nativeconfig.New().Apply(nativeconfig.Request{Paths: nativeconfig.Paths{JSON: config}, Codec: nativeconfig.CodecOpenCode, Action: nativeconfig.ActionAdd, Name: "local", Server: server, Placeholders: nativeconfig.Placeholders{PackageRoot: root}}); err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(readOpenCodeTestFile(t, config)), &doc); err != nil {
		t.Fatal(err)
	}
	if doc["mcp"].(map[string]any)["local"].(map[string]any)["cwd"] != filepath.Join(root, "work") {
		t.Fatalf("historical cwd changed: %#v", doc)
	}
}
