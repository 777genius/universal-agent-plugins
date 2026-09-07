package usecase

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/loader"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/specregistry"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/managedstdio"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
)

func TestPortableDotPathsInstallAndExactRepair(t *testing.T) {
	for _, cwd := range []string{"./", "./data/..", "${PLUGIN_ROOT}"} {
		t.Run(cwd, func(t *testing.T) {
			service, store, _ := serviceFixture(t)
			service.NativeObserver = providers.NativeIdentityObserver{Stager: service.Stager}
			client := domain.DetectedClient{ClientID: domain.ClientOpenCode, Status: domain.DetectionDetected, ConfigRoot: filepath.Join(t.TempDir(), "opencode")}
			input := addInput(t, client, "https://example.test/portable-dot-paths")
			root := input.Envelope.SnapshotRoot
			for _, dir := range []string{"bin", "data"} {
				if err := os.MkdirAll(filepath.Join(root, dir), 0700); err != nil {
					t.Fatal(err)
				}
			}
			// Executable permission is observed; neither loader nor this lifecycle launches it.
			if err := os.WriteFile(filepath.Join(root, "bin/server"), []byte("inert test executable; never run\n"), 0755); err != nil {
				t.Fatal(err)
			}
			mcp := map[string]any{"$schema": domain.MCPSchemaV1, "mcpServers": map[string]any{
				"local":  map[string]any{"type": "stdio", "command": "./bin/../bin/server", "cwd": cwd},
				"remote": map[string]any{"type": "streamable-http", "url": "https://example.test/mcp"},
			}}
			body, err := json.Marshal(mcp)
			if err != nil {
				t.Fatal(err)
			}
			if err = os.WriteFile(filepath.Join(root, "mcp.json"), body, 0600); err != nil {
				t.Fatal(err)
			}
			registry, err := specregistry.New()
			if err != nil {
				t.Fatal(err)
			}
			envelope, err := (loader.Loader{Registry: registry}).Load(context.Background(), domain.LoadInput{SnapshotRoot: root, Source: input.Envelope.Source, TreeDigest: input.Envelope.TreeDigest, ExecutableFiles: []string{"bin/server"}})
			if err != nil {
				t.Fatal(err)
			}
			if len(envelope.MCP.Servers) != 2 {
				t.Fatalf("loader diagnostics: %+v", envelope.Diagnostics)
			}
			input.Envelope = envelope
			added, err := service.AddGroup(context.Background(), GroupInput{Targets: []AddInput{input}, OperationGroupID: "dot-path-add", Confirmed: true})
			if err != nil {
				t.Fatal(err)
			}
			state, err := store.Load()
			if err != nil {
				t.Fatal(err)
			}
			active := onlyBinding(state.Installations[0]).TargetLocator
			verify := func() {
				t.Helper()
				config := readUsecaseObject(t, filepath.Join(client.ConfigRoot, "opencode.json"))
				server := config["mcp"].(map[string]any)["local"].(map[string]any)
				argv := server["command"].([]any)
				if argv[0] != filepath.Join(active, "bin/server") || server["cwd"] != active {
					t.Fatalf("projection: %+v", server)
				}
			}
			verify()
			if err := os.WriteFile(filepath.Join(client.ConfigRoot, "opencode.json"), []byte(`{"mcp":{}}`), 0600); err != nil {
				t.Fatal(err)
			}
			input.InstallationID = added.InstallationID
			repaired, err := service.RepairGroup(context.Background(), GroupInput{Targets: []AddInput{input}, OperationGroupID: "dot-path-repair", Confirmed: true, Repair: true})
			if err != nil {
				t.Fatal(err)
			}
			if !repaired.Mutated {
				t.Fatal("repair did not restore missing native artifacts")
			}
			verify()
			restored, err := os.ReadFile(filepath.Join(active, "bin/server"))
			if err != nil || string(restored) != "inert test executable; never run\n" {
				t.Fatalf("restored executable = %q, %v", restored, err)
			}
		})
	}
}

func TestClaudeBundledDotPathsInstallAndRepair(t *testing.T) {
	service, _, _ := serviceFixture(t)
	helper := filepath.Join(t.TempDir(), "fixture-helper")
	if err := os.WriteFile(helper, []byte("fixture helper; never executed"), 0700); err != nil {
		t.Fatal(err)
	}
	source, err := managedstdio.NewSource(helper, "fixture")
	if err != nil {
		t.Fatal(err)
	}
	service.Stager = providers.Stager{LauncherSource: source}
	client := domain.DetectedClient{ClientID: domain.ClientClaude, Status: domain.DetectionDetected, ConfigRoot: filepath.Join(t.TempDir(), "claude"), ExecutablePath: "/test/bin/claude"}
	runner := &fakeClaudeLifecycleRunner{configRoot: client.ConfigRoot}
	service.Activator = providers.Activator{Runner: runner}
	service.NativeObserver = providers.NativeIdentityObserver{Runner: runner, Stager: service.Stager}
	input := addInput(t, client, "https://example.test/claude-bundled-paths")
	input.BackendExecutable = client.ExecutablePath
	root := input.Envelope.SnapshotRoot
	for _, dir := range []string{"bin", "work"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "bin/server"), []byte("inert fixture; never executed\n"), 0755); err != nil {
		t.Fatal(err)
	}
	body := []byte(`{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{"local":{"type":"stdio","command":"./bin/../bin/server","cwd":"./work","args":["${PLUGIN_ROOT}/config"]}}}`)
	if err := os.WriteFile(filepath.Join(root, "mcp.json"), body, 0600); err != nil {
		t.Fatal(err)
	}
	registry, err := specregistry.New()
	if err != nil {
		t.Fatal(err)
	}
	input.Envelope, err = (loader.Loader{Registry: registry}).Load(context.Background(), domain.LoadInput{SnapshotRoot: root, Source: input.Envelope.Source, TreeDigest: input.Envelope.TreeDigest, ExecutableFiles: []string{"bin/server"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(input.Envelope.MCP.Servers) != 1 {
		t.Fatalf("loader diagnostics: %+v", input.Envelope.Diagnostics)
	}
	input.Confirmed = true
	installed, err := service.Add(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	verify := func() {
		t.Helper()
		runtime := filepath.Join(installed.Plan.ActivePath, ".agentplugins-runtime")
		server := readUsecaseObject(t, filepath.Join(installed.Plan.ActivePath, ".mcp.json"))["local"].(map[string]any)
		if server["cwd"] != nil || server["args"].([]any)[3] != filepath.Join(runtime, "work") || server["args"].([]any)[6] != "./bin/../bin/server" {
			t.Fatalf("Claude final paths: %+v", server)
		}
		if server["args"].([]any)[7] != filepath.Join(runtime, "config") {
			t.Fatalf("Claude PLUGIN_ROOT expansion: %+v", server)
		}
		if _, err := os.Stat(filepath.Join(runtime, "bin/server")); err != nil {
			t.Fatal(err)
		}
	}
	verify()
	if err := os.RemoveAll(installed.Plan.ActivePath); err != nil {
		t.Fatal(err)
	}
	input.OperationID = "claude-dot-repair"
	repaired, err := service.Repair(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if !repaired.Mutated {
		t.Fatal("Claude missing managed package was not repaired")
	}
	verify()
}
