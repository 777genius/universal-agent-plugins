package windsurf

import (
	"encoding/json"
	"os"

	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/managedstdio"
)

func TestWindsurfSSEUsesLegacyURLKey(t *testing.T) {
	t.Parallel()
	configPath := filepath.Join(t.TempDir(), ".codeium", "windsurf-insiders", "mcp_config.json")
	server, err := windsurfServer(domain.MCPServer{
		Type:    "sse",
		Decoded: map[string]any{"type": "sse", "url": "https://events.test/sse"},
	}, "/test/package", "/test/data")
	if err != nil {
		t.Fatal(err)
	}
	if server.RemoteTransport != "sse" {
		t.Fatalf("remote transport = %q", server.RemoteTransport)
	}
	if _, err := nativeconfig.New().Apply(nativeconfig.Request{
		Paths: nativeconfig.Paths{JSON: configPath}, Codec: nativeconfig.CodecWindsurf,
		Action: nativeconfig.ActionAdd, Name: "events", Server: server,
	}); err != nil {
		t.Fatal(err)
	}
	entry := readObject(t, configPath)["mcpServers"].(map[string]any)["events"].(map[string]any)
	if entry["url"] != "https://events.test/sse" || entry["serverUrl"] != nil {
		t.Fatalf("SSE native shape = %+v", entry)
	}
}

func TestWindsurfExpandsOnlyPortableStdioValues(t *testing.T) {
	t.Parallel()
	packageRoot := filepath.Join(t.TempDir(), "plugin", "${PLUGIN_DATA}")
	dataRoot := filepath.Join(t.TempDir(), "data")
	stdio, err := windsurfServer(domain.MCPServer{
		Type: "stdio",
		Decoded: map[string]any{
			"type":    "stdio",
			"command": "${PLUGIN_ROOT}",
			"args":    []any{"${PLUGIN_ROOT}/run.js", "${PLUGIN_CACHE}/literal"},
			"env":     map[string]any{"DATA": "${PLUGIN_DATA}/state", "UNKNOWN": "${HOME}"},
		},
	}, packageRoot, dataRoot)
	if err != nil {
		t.Fatal(err)
	}
	if stdio.Args[6] != "${PLUGIN_ROOT}" || stdio.Args[7] != filepath.Join(packageRoot, "run.js") || stdio.Args[8] != "${PLUGIN_CACHE}/literal" || stdio.Env["DATA"] != filepath.Join(dataRoot, "state") || stdio.Env["UNKNOWN"] != "${HOME}" {
		t.Fatalf("Windsurf stdio placeholder projection = %+v", stdio)
	}
	if strings.Contains(stdio.Args[7], dataRoot) {
		t.Fatalf("Windsurf recursively expanded replacement text: %+v", stdio.Args)
	}

	remote, err := windsurfServer(domain.MCPServer{
		Type: "streamable-http",
		Decoded: map[string]any{
			"type":    "streamable-http",
			"url":     "https://example.test/${PLUGIN_ROOT}",
			"headers": map[string]any{"X-Path": "${PLUGIN_DATA}"},
		},
	}, packageRoot, dataRoot)
	if err != nil {
		t.Fatal(err)
	}
	if remote.URL != "https://example.test/${PLUGIN_ROOT}" || remote.Headers["X-Path"] != "${PLUGIN_DATA}" {
		t.Fatalf("Windsurf remote literals were expanded: %+v", remote)
	}
}

func TestWindsurfStandardProjectionNeverDropsCWD(t *testing.T) {
	t.Parallel()
	_, err := standardWindsurfServer(nativeconfig.Server{Type: "stdio", Command: "node", CWD: "/workspace"}, "stdio")
	if err == nil || !strings.Contains(err.Error(), "does not support cwd") {
		t.Fatalf("Windsurf standard projection accepted cwd: %v", err)
	}
}

func TestWindsurfRejectsReservedStdioEnvWithoutChangingProjection(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	projectionPath := filepath.Join(root, "mcp.json")
	writeTestFile(t, projectionPath, "sentinel\n")
	envelope := windsurfTestEnvelope(t, "collision")
	local := envelope.MCP.Servers["local"]
	authorEnv := map[string]any{"KEEP": "value", "PLUGIN_DATA": "/attacker"}
	local.Decoded["env"] = authorEnv
	envelope.MCP.Servers["local"] = local
	plan := stagingPlan(t, domain.ClientWindsurf, domain.PackagePrepared)
	plan.Components = []domain.ComponentDecision{{Kind: domain.ComponentMCPServer, Name: "local", Support: domain.SupportPrepared}}

	err := ProjectMCP(root, envelope, plan, filepath.Join(t.TempDir(), "data"))
	if err == nil || !strings.Contains(err.Error(), "PLUGIN_DATA is reserved") {
		t.Fatalf("reserved Windsurf env error = %v", err)
	}
	body, readErr := os.ReadFile(projectionPath)
	if readErr != nil || string(body) != "sentinel\n" {
		t.Fatalf("rejected Windsurf env changed projection = %q, %v", body, readErr)
	}
	if len(authorEnv) != 2 || authorEnv["KEEP"] != "value" || authorEnv["PLUGIN_DATA"] != "/attacker" {
		t.Fatalf("rejected Windsurf projection mutated package env: %#v", authorEnv)
	}
}

func TestWindsurfBundledCommandUsesManagedPluginRoot(t *testing.T) {
	t.Parallel()
	activeRoot := t.TempDir()
	commandPath := filepath.Join(activeRoot, "bin", "server")
	writeTestFile(t, commandPath, "#!/bin/sh\nprintf 'managed-root\\n'\n")
	if err := os.Chmod(commandPath, 0o700); err != nil {
		t.Fatal(err)
	}

	envelope := windsurfTestEnvelope(t, "bundled")
	local := envelope.MCP.Servers["local"]
	local.Decoded = shared.CloneObject(local.Decoded)
	local.Decoded["command"] = "./bin/server"
	local.Decoded["args"] = []any{}
	envelope.MCP.Servers["local"] = local
	plan := stagingPlan(t, domain.ClientWindsurf, domain.PackagePrepared)
	plan.ActivePath = activeRoot
	plan.NativeRegistryRoot = filepath.Join(t.TempDir(), ".codeium", "windsurf")
	plan.Components = []domain.ComponentDecision{{Kind: domain.ComponentMCPServer, Name: "local", Support: domain.SupportPrepared}}

	if err := ProjectMCP(activeRoot, envelope, plan, filepath.Join(t.TempDir(), "data")); err != nil {
		t.Fatal(err)
	}
	objects, err := BuildNativeObjects(activeRoot, plan)
	if err != nil {
		t.Fatal(err)
	}
	if err := applyWindsurfNativeMutation(plan.NativeRegistryRoot, activeRoot, nil, objects); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(plan.NativeRegistryRoot, "mcp_config.json")
	entry := readObject(t, configPath)["mcpServers"].(map[string]any)["local"].(map[string]any)
	if got := entry["command"]; got != filepath.Join(activeRoot, filepath.FromSlash(managedstdio.RelativeDirectory), managedstdio.ExecutableName) {
		t.Fatalf("bundled Windsurf command = %v, want %s", got, commandPath)
	}
	if got := envelope.MCP.Servers["local"].Decoded["command"]; got != "./bin/server" {
		t.Fatalf("Windsurf projection mutated source command: %v", got)
	}
}

func windsurfTestEnvelope(t *testing.T, revision string) domain.PackageEnvelope {
	t.Helper()
	localRaw := json.RawMessage(`{"type":"stdio","command":"node","args":["${PLUGIN_ROOT}/runtime/server.js","${PLUGIN_DATA}/cache"]}`)
	remoteRaw := json.RawMessage(`{"type":"streamable-http","url":"https://docs.test/` + revision + `"}`)
	return domain.PackageEnvelope{
		MCP: domain.MCPComponent{Servers: map[string]domain.MCPServer{
			"local": {Name: "local", Type: "stdio", Raw: localRaw, Decoded: map[string]any{"type": "stdio", "command": "node", "args": []any{"${PLUGIN_ROOT}/runtime/server.js", "${PLUGIN_DATA}/cache"}}},
			"docs":  {Name: "docs", Type: "streamable-http", Raw: remoteRaw, Decoded: map[string]any{"type": "streamable-http", "url": "https://docs.test/" + revision}},
		}},
	}
}

func stagingPlan(t *testing.T, client domain.ClientID, mode domain.PackageMode) domain.DeliveryPlan {
	t.Helper()
	root := t.TempDir()
	anchor := filepath.Join(root, "managed")
	target := filepath.Join(anchor, "clients", string(client))
	return domain.DeliveryPlan{
		ClientID: client, Scope: domain.ScopeUser, Status: domain.PlanReady, PackageMode: mode,
		PhysicalArtifactID: "demo-0123456789ab", TargetAnchor: anchor, TargetRoot: target,
		ActivePath: filepath.Join(target, "demo-0123456789ab"),
	}
}
