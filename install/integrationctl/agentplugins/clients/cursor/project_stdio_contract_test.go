package cursor

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/managedstdio"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/pathcontract"
)

func TestProjectStdioPreservesOpaqueVectorInEncodedProjection(t *testing.T) {
	active, data := filepath.Join(testBase(t), "package"), filepath.Join(testBase(t), "data")
	args := []any{"", ".", "./relative", "~/relative", "${UNKNOWN}", "雪", "\r\n"}
	entry := map[string]any{"cwd": active, "args": args}
	source := domain.MCPServer{Type: "stdio", Decoded: map[string]any{"command": "./bin/probe", "cwd": "${PLUGIN_ROOT}"}}
	if err := bindCursorStdio(entry, source, active, data); err != nil {
		t.Fatal(err)
	}
	wire, ok := entry["args"].([]string)
	if !ok || len(wire) != 14 || wire[5] != "--argv-base64-v1" || wire[6] != "Li9iaW4vcHJvYmU=" {
		t.Fatal("Cursor did not project the explicit encoded transport")
	}
	decoded, err := managedstdio.DecodeCommandArguments(wire[5], wire[6:])
	want := []string{"./bin/probe", "", ".", "./relative", "~/relative", "${UNKNOWN}", "雪", "\r\n"}
	if err != nil || !reflect.DeepEqual(decoded, want) {
		t.Fatal("Cursor projected child vector changed", err)
	}
	if _, exists := entry["cwd"]; exists || entry["command"] != filepath.Join(active, filepath.FromSlash(managedstdio.RelativeDirectory), managedstdio.ExecutableName) {
		t.Fatal("Cursor bypassed managed cwd/launcher authority")
	}
}

func TestProjectStdioVerificationAcceptsOwnedLegacyAndEncodedForms(t *testing.T) {
	root := testBase(t)
	active, data := filepath.Join(root, "active"), filepath.Join(root, "data")
	for _, path := range []string{active, data} {
		if err := os.Mkdir(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	envelope := domain.PackageEnvelope{MCP: domain.MCPComponent{Servers: map[string]domain.MCPServer{"probe": {Type: "stdio", Decoded: map[string]any{"command": "./bin/probe", "cwd": "${PLUGIN_ROOT}"}}}}}
	plan := domain.DeliveryPlan{ActivePath: active, Components: []domain.ComponentDecision{{Kind: domain.ComponentMCPServer, Name: "probe", Support: domain.SupportNative}}}
	legacy := managedstdio.Arguments(active, data, active, pathcontract.Plugin, "./bin/probe", []string{"./relative", "\n"})
	encoded := managedstdio.EncodedArguments(active, data, active, pathcontract.Plugin, "./bin/probe", []string{"./relative", "\n"})
	for _, fault := range []string{"legacy", "encoded", "mode", "plugin", "data", "cwd", "anchor", "separator", "missing-command", "malformed-command", "malformed-tail", "wire-newline", "decoded-nul"} {
		t.Run(fault, func(t *testing.T) {
			args := append([]string(nil), encoded...)
			switch fault {
			case "legacy":
				args = append([]string(nil), legacy...)
			case "mode":
				args[0] = "--unknown"
			case "plugin":
				args[1] = filepath.Join(root, "foreign")
			case "data":
				args[2] = filepath.Join(root, "foreign")
			case "cwd":
				args[3] = data
			case "anchor":
				args[4] = "data"
			case "separator":
				args[5] = "--unknown"
			case "missing-command":
				args = args[:6]
			case "malformed-command":
				args[6] = "?"
			case "malformed-tail":
				args[len(args)-1] = "?"
			case "wire-newline":
				args[6] += "\n"
			case "decoded-nul":
				args[len(args)-1] = "AA=="
			}
			entry := map[string]any{"command": filepath.Join(active, filepath.FromSlash(managedstdio.RelativeDirectory), managedstdio.ExecutableName), "args": args, "env": map[string]string{"PLUGIN_ROOT": active, "PLUGIN_DATA": data}}
			body, err := json.Marshal(map[string]any{"mcpServers": map[string]any{"probe": entry}})
			if err != nil || os.WriteFile(filepath.Join(active, "mcp.json"), body, 0600) != nil {
				t.Fatal("inert Cursor projection fixture", err)
			}
			err = validateCursorProjectStdio(active, envelope, plan, data)
			if fault == "legacy" || fault == "encoded" {
				if err != nil {
					t.Fatal("owned supported wire was orphaned", err)
				}
			} else if !errors.Is(err, domain.ErrProjectionRefreshRequired) {
				t.Fatal("malformed wire or wrong selected locator was verified", err)
			}
		})
	}
}
