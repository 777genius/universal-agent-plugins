package grok

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/managedstdio"
)

func TestProjectStdioEncodedCWDAndOpaqueArguments(t *testing.T) {
	root := projectTestRoot(t)
	plugin, data := filepath.Join(root, "plugin"), filepath.Join(root, "data")
	projectTestWrite(t, filepath.Join(root, "bin", "probe"), "inert")
	if err := os.MkdirAll(filepath.Join(root, "runtime"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(data, 0700); err != nil {
		t.Fatal(err)
	}
	for _, command := range []string{"go", "./bin/probe"} {
		for _, cwd := range []string{"", "./runtime", "${PLUGIN_DATA}"} {
			t.Run(command+"-"+cwd, func(t *testing.T) {
				source := domain.MCPServer{Type: "stdio", Decoded: map[string]any{"command": command, "cwd": cwd, "args": []any{"", ".", "./relative", "~/relative", "${UNKNOWN}", "雪", "\r\n"}}}
				entry := shared.CloneObject(source.Decoded)
				delete(entry, "type")
				if err := shared.ApplyStdioDataContract(entry, plugin, data, root); err != nil {
					t.Fatal(err)
				}
				expected := plugin
				anchor := "plugin"
				if cwd == "./runtime" {
					expected = filepath.Join(plugin, "runtime")
				}
				if cwd == "${PLUGIN_DATA}" {
					expected = data
					anchor = "data"
				}
				if err := bindGrokStdio(entry, source, plugin, data); err != nil {
					t.Fatal(err)
				}
				wire, err := grokStdioArguments(entry)
				if err != nil || len(wire) != 14 || wire[1] != plugin || wire[2] != data || wire[3] != expected || wire[4] != anchor {
					t.Fatal("selected cwd authority changed", wire, err)
				}
				decoded, err := managedstdio.DecodeCommandArguments(wire[5], wire[6:])
				want := []string{command, "", ".", "./relative", "~/relative", "${UNKNOWN}", "雪", "\r\n"}
				if err != nil || !reflect.DeepEqual(decoded, want) {
					t.Fatal("opaque command vector changed", decoded, err)
				}
				if _, present := entry["cwd"]; present || entry["command"] != filepath.Join(plugin, filepath.FromSlash(managedstdio.RelativeDirectory), managedstdio.ExecutableName) {
					t.Fatal("native cwd bypassed launcher", entry)
				}
			})
		}
	}
}

func TestProjectStdioReadbackRejectsLegacyAndWrongAuthority(t *testing.T) {
	for _, fault := range []string{"none", "legacy", "plugin", "data", "cwd", "anchor", "arguments", "helper", "metadata"} {
		t.Run(fault, func(t *testing.T) {
			root := projectTestRoot(t)
			active, data := filepath.Join(root, "active"), filepath.Join(root, "data")
			for _, path := range []string{active, data} {
				if err := os.Mkdir(path, 0700); err != nil {
					t.Fatal(err)
				}
			}
			executable, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			launcher, err := managedstdio.NewSource(executable, "fixture")
			if err != nil {
				t.Fatal(err)
			}
			if err := launcher.Deliver(active); err != nil {
				t.Fatal(err)
			}
			source := domain.MCPServer{Type: "stdio", Decoded: map[string]any{"command": "./bin/probe", "args": []any{"./opaque"}}}
			envelope := domain.PackageEnvelope{MCP: domain.MCPComponent{Servers: map[string]domain.MCPServer{"probe": source}}}
			plan := domain.DeliveryPlan{ActivePath: active, Components: []domain.ComponentDecision{{Kind: domain.ComponentMCPServer, Name: "probe", Support: domain.SupportNative}}}
			projectTestWrite(t, filepath.Join(active, "bin", "probe"), "inert")
			entry := shared.CloneObject(source.Decoded)
			if err := shared.ApplyStdioDataContract(entry, active, data, active); err != nil {
				t.Fatal(err)
			}
			legacy := shared.CloneObject(entry)
			if err := bindGrokStdio(entry, source, active, data); err != nil {
				t.Fatal(err)
			}
			wire, _ := grokStdioArguments(entry)
			switch fault {
			case "legacy":
				entry = legacy
			case "plugin":
				wire[1] = data
			case "data":
				wire[2] = active
			case "cwd":
				wire[3] = data
			case "anchor":
				wire[4] = "data"
			case "arguments":
				wire[7] = "Pw=="
			case "helper":
				if err := os.WriteFile(filepath.Join(active, filepath.FromSlash(managedstdio.RelativeDirectory), managedstdio.ExecutableName), []byte("different"), 0700); err != nil {
					t.Fatal(err)
				}
			case "metadata":
				if err := os.WriteFile(filepath.Join(active, filepath.FromSlash(managedstdio.RelativeDirectory), "metadata.json"), []byte(`{"protocol":1,"sha256":"wrong","cliVersion":"fixture"}`), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if fault != "legacy" {
				entry["args"] = wire
			}
			body, err := json.Marshal(map[string]any{"mcpServers": map[string]any{"probe": entry}})
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(active, ".mcp.json")
			if err := os.WriteFile(path, body, 0600); err != nil {
				t.Fatal(err)
			}
			err = validateGrokStdio(active, envelope, plan, data)
			if fault == "none" {
				if err != nil {
					t.Fatal(err)
				}
			} else if !errors.Is(err, domain.ErrProjectionRefreshRequired) {
				t.Fatal("wrong projection accepted", err)
			}
			after, readErr := os.ReadFile(path)
			if readErr != nil || string(after) != string(body) {
				t.Fatal("readback rewrote installed config", readErr)
			}
		})
	}
}

func TestProjectStdioMissingTrustedHelperWritesNothing(t *testing.T) {
	request := projectTestRequest(t)
	request.Plan.Components = []domain.ComponentDecision{{Kind: domain.ComponentMCPServer, Name: "probe", Support: domain.SupportNative}}
	stage := filepath.Join(request.Plan.TargetAnchor, ".agentplugins-staging-fixture")
	if err := os.MkdirAll(stage, 0700); err != nil {
		t.Fatal(err)
	}
	envelope := domain.PackageEnvelope{MCP: domain.MCPComponent{Servers: map[string]domain.MCPServer{"probe": {Type: "stdio", Decoded: map[string]any{"command": "go"}}}}}
	_, err := New().Project(t.Context(), clients.ProjectionInput{StagingPath: stage, Envelope: envelope, Plan: request.Plan, PluginDataPath: filepath.Join(projectTestRoot(t), "data")})
	if err == nil {
		t.Fatal("missing trusted helper accepted")
	}
	entries, readErr := os.ReadDir(stage)
	if readErr != nil || len(entries) != 0 {
		t.Fatal("failed helper admission wrote projection", entries, readErr)
	}
}
