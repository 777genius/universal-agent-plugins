package providers

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/managedstdio"
)

func TestClaudeStdioDistinctCWDAndOpaqueArguments(t *testing.T) {
	envelope := stagingEnvelope(t)
	if err := os.Mkdir(filepath.Join(envelope.SnapshotRoot, "work"), 0700); err != nil {
		t.Fatal(err)
	}
	envelope.MCP.Servers = map[string]domain.MCPServer{}
	plan := stagingPlan(t, domain.ClientClaude, domain.PackageProjection)
	plan.Components = nil
	data := filepath.Join(t.TempDir(), "data-${PLUGIN_ROOT}")
	if err := os.Mkdir(data, 0700); err != nil {
		t.Fatal(err)
	}
	opaque := []any{"", "--", "$(touch escaped)", "a b", "${UNKNOWN}", "${PLUGIN_DATA}/cache"}
	for name, cwd := range map[string]string{"default": "", "explicit": "./work", "data": "${PLUGIN_DATA}"} {
		envelope.MCP.Servers[name] = domain.MCPServer{Type: "stdio", Decoded: map[string]any{"command": "sh", "cwd": cwd, "args": opaque, "env": map[string]any{"VALUE": "${PLUGIN_DATA}|${UNKNOWN}"}}}
		plan.Components = append(plan.Components, domain.ComponentDecision{Kind: domain.ComponentMCPServer, Name: name, Support: domain.SupportProjected})
	}
	delivery, err := windsurfFixtureStager(t).StageWithPluginData(context.Background(), envelope, plan, "opaque", domain.CompatibilityHints{}, data)
	if err != nil {
		t.Fatal(err)
	}
	document := readObject(t, filepath.Join(delivery.StagingPath, ".mcp.json"))
	runtime := filepath.Join(plan.ActivePath, claudeRuntimeDirectory)
	for name, wantCWD := range map[string]string{"default": runtime, "explicit": filepath.Join(runtime, "work"), "data": data} {
		server := document[name].(map[string]any)
		args := server["args"].([]any)
		if server["command"] != filepath.Join(runtime, filepath.FromSlash(managedstdio.RelativeDirectory), managedstdio.ExecutableName) || server["cwd"] != nil || args[3] != wantCWD || args[6] != "sh" {
			t.Fatalf("%s: %+v", name, server)
		}
		anchor := "plugin"
		if name == "data" {
			anchor = "data"
		}
		if args[4] != anchor {
			t.Fatalf("anchor: %v", args)
		}
		want := []any{"", "--", "$(touch escaped)", "a b", "${UNKNOWN}", data + "/cache"}
		if !reflect.DeepEqual(args[7:], want) {
			t.Fatalf("opaque argv changed: %v", args)
		}
		env := server["env"].(map[string]any)
		if env["VALUE"] != data+"|${UNKNOWN}" || env["PLUGIN_ROOT"] != runtime || env["PLUGIN_DATA"] != data {
			t.Fatalf("env expanded again: %v", env)
		}
	}
	if !reflect.DeepEqual(envelope.MCP.Servers["default"].Decoded["args"], opaque) {
		t.Fatal("source args changed")
	}
	if _, err := os.Stat(filepath.Join(envelope.SnapshotRoot, filepath.FromSlash(managedstdio.RelativeDirectory))); !os.IsNotExist(err) {
		t.Fatal("source gained helper", err)
	}
}
