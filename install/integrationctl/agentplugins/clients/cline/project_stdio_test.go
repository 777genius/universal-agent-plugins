package cline

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/managedstdio"
)

func TestProjectClineManagedStdioPreservesAuthoredCommandAndCWD(t *testing.T) {
	root, data := t.TempDir(), t.TempDir()
	active := filepath.Join(root, "active")
	for _, path := range []string{filepath.Join(root, "runtime"), filepath.Join(root, "bin")} {
		if err := os.MkdirAll(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "bin", "probe"), []byte("inert"), 0700); err != nil {
		t.Fatal(err)
	}
	plan := domain.DeliveryPlan{Scope: domain.ScopeProject, ActivePath: active}
	for _, command := range []string{"go", "./bin/probe"} {
		for _, cwd := range []string{"", "./runtime", "${PLUGIN_DATA}"} {
			t.Run(command+cwd, func(t *testing.T) {
				args := []any{"", "argument with spaces", "${PLUGIN_ROOT}/file", "${PLUGIN_DATA}", "${UNKNOWN}", "$HOME", "./relative"}
				source := domain.MCPServer{Type: "stdio", Decoded: map[string]any{"command": command, "cwd": cwd, "args": args, "env": map[string]any{"MARKER": "${PLUGIN_DATA}/${UNKNOWN}"}}}
				original, _ := json.Marshal(source.Decoded)
				server, err := projectClineServer(root, source, plan, data)
				if err != nil {
					t.Fatal(err)
				}
				expectedCWD, anchor := active, "plugin"
				if cwd == "./runtime" {
					expectedCWD = filepath.Join(active, "runtime")
				}
				if cwd == "${PLUGIN_DATA}" {
					expectedCWD, anchor = data, "data"
				}
				expectedArgs := []string{managedstdio.Mode, active, data, expectedCWD, anchor, "--", command, "", "argument with spaces", active + "/file", data, "${UNKNOWN}", "$HOME", "./relative"}
				helper := filepath.Join(active, filepath.FromSlash(managedstdio.RelativeDirectory), managedstdio.ExecutableName)
				if server.Command != helper || server.CWD != expectedCWD || !reflect.DeepEqual(server.Args, expectedArgs) {
					t.Fatalf("managed binding: %+v want %q", server, expectedArgs)
				}
				if server.Env["PLUGIN_ROOT"] != active || server.Env["PLUGIN_DATA"] != data || server.Env["MARKER"] != data+"/${UNKNOWN}" {
					t.Fatal(server.Env)
				}
				decoded, err := managedstdio.DecodeCommandArguments(server.Args[5], server.Args[6:])
				if err != nil || decoded[0] != command || !reflect.DeepEqual(decoded[1:], expectedArgs[7:]) {
					t.Fatal(decoded, err)
				}
				after, _ := json.Marshal(source.Decoded)
				if string(after) != string(original) {
					t.Fatal("projection mutated authored envelope")
				}
			})
		}
	}
}

func TestProjectClineManagedStdioLeavesUserProjectionUnchanged(t *testing.T) {
	root, data := t.TempDir(), t.TempDir()
	source := domain.MCPServer{Type: "stdio", Decoded: map[string]any{"command": "go", "args": []any{"${PLUGIN_ROOT}", "${UNKNOWN}"}}}
	plan := domain.DeliveryPlan{Scope: domain.ScopeUser, ActivePath: root}
	server, err := projectClineServer(root, source, plan, data)
	if err != nil {
		t.Fatal(err)
	}
	if server.Command != "go" || server.CWD != root || !reflect.DeepEqual(server.Args, []string{root, "${UNKNOWN}"}) {
		t.Fatal(server)
	}
	if server.Env["PLUGIN_ROOT"] != root || server.Env["PLUGIN_DATA"] != data {
		t.Fatal(server.Env)
	}
}

func TestProjectClineManagedStdioActiveReadbackRequiresExactBinding(t *testing.T) {
	root, data := t.TempDir(), t.TempDir()
	source := domain.MCPServer{Type: "stdio", Decoded: map[string]any{"command": "go", "args": []any{"run", "${PLUGIN_ROOT}/main.go"}}}
	plan := domain.DeliveryPlan{Scope: domain.ScopeProject, ActivePath: root, Components: []domain.ComponentDecision{{Kind: domain.ComponentMCPServer, Name: "docs", Support: domain.SupportPrepared}}}
	envelope := domain.PackageEnvelope{MCP: domain.MCPComponent{Servers: map[string]domain.MCPServer{"docs": source}}}
	if err := ProjectClineNative(root, envelope, plan, data); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, ClineProjectionFile)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := validateClineProjectStdio(root, envelope, plan, data); err != nil {
		t.Fatal(err)
	}
	for _, selected := range []string{"", "relative", t.TempDir()} {
		if err := validateClineProjectStdio(root, envelope, plan, selected); !errors.Is(err, domain.ErrProjectionRefreshRequired) {
			t.Fatal("foreign data admitted", selected, err)
		}
	}
	for _, mutation := range []string{"unwrapped", "cwd", "args", "env"} {
		t.Run(mutation, func(t *testing.T) {
			var projection ClineProjection
			if err := json.Unmarshal(before, &projection); err != nil {
				t.Fatal(err)
			}
			server := projection.Servers["docs"]
			switch mutation {
			case "unwrapped":
				server.Command = "go"
				server.Args = []string{"run", root + "/main.go"}
			case "cwd":
				server.CWD = data
			case "args":
				server.Args = append(server.Args, "foreign")
			case "env":
				server.Env["PLUGIN_DATA"] = "foreign"
			}
			projection.Servers["docs"] = server
			if err := shared.WriteJSON(path, projection); err != nil {
				t.Fatal(err)
			}
			stale, _ := os.ReadFile(path)
			if err := validateClineProjectStdio(root, envelope, plan, data); !errors.Is(err, domain.ErrProjectionRefreshRequired) {
				t.Fatal("stale projection admitted", err)
			}
			after, _ := os.ReadFile(path)
			if string(after) != string(stale) {
				t.Fatal("active validation rewrote immutable projection")
			}
		})
	}
}

func TestProjectClineManagedStdioValidatesEveryMemberFromOneProjection(t *testing.T) {
	root, data := t.TempDir(), t.TempDir()
	envelope := domain.PackageEnvelope{MCP: domain.MCPComponent{Servers: map[string]domain.MCPServer{}}}
	plan := domain.DeliveryPlan{Scope: domain.ScopeProject, ActivePath: root}
	for _, name := range []string{"first", "last"} {
		envelope.MCP.Servers[name] = domain.MCPServer{Type: "stdio", Decoded: map[string]any{"command": "go", "args": []any{name}}}
		plan.Components = append(plan.Components, domain.ComponentDecision{Kind: domain.ComponentMCPServer, Name: name, Support: domain.SupportPrepared})
	}
	if err := ProjectClineNative(root, envelope, plan, data); err != nil {
		t.Fatal(err)
	}
	if err := validateClineProjectStdio(root, envelope, plan, data); err != nil {
		t.Fatal(err)
	}
	projection, err := ReadClineProjection(root)
	if err != nil {
		t.Fatal(err)
	}
	late := projection.Servers["last"]
	late.Command = "go"
	projection.Servers["last"] = late
	path := filepath.Join(root, ClineProjectionFile)
	if err := shared.WriteJSON(path, projection); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(path)
	if err := validateClineProjectStdio(root, envelope, plan, data); !errors.Is(err, domain.ErrProjectionRefreshRequired) {
		t.Fatal("late unwrapped member admitted", err)
	}
	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Fatal("multi-member verification changed installed source")
	}
}

func TestProjectClineManagedStdioRemoteOnlyDoesNotReadProjection(t *testing.T) {
	plan := domain.DeliveryPlan{Scope: domain.ScopeProject, Components: []domain.ComponentDecision{{Kind: domain.ComponentMCPServer, Name: "remote", Support: domain.SupportPrepared}}}
	envelope := domain.PackageEnvelope{MCP: domain.MCPComponent{Servers: map[string]domain.MCPServer{"remote": {Type: "streamable-http"}}}}
	if err := validateClineProjectStdio(filepath.Join(t.TempDir(), "absent"), envelope, plan, ""); err != nil {
		t.Fatal(err)
	}
}

func TestReadClineProjectionRejectsAmbiguousOrLinkedJSON(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, ClineProjectionFile)
	invalid := []string{
		`{"servers":{"owned":{"type":"stdio"},"owned":{"type":"sse"}}}`,
		`{"servers":{"owned":{"type":"stdio","type":"sse"}}}`,
		`{"servers":{"owned":{"type":"stdio","TYPE":"sse"}}}`,
		`{"SERVERS":{"owned":{"type":"stdio"}},"servers":{}}`,
		`{"servers":{}} {"servers":{}}`,
	}
	for _, body := range invalid {
		if err := os.WriteFile(path, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := ReadClineProjection(root); err == nil {
			t.Fatalf("ambiguous Cline projection accepted: %s", body)
		}
	}
	if err := os.WriteFile(path, []byte(`{"servers":{}}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadClineProjection(root); err != nil {
		t.Fatalf("unambiguous Cline projection refused: %v", err)
	}
	t.Run("symlink", func(t *testing.T) {
		foreign := filepath.Join(t.TempDir(), "foreign.json")
		if err := os.WriteFile(foreign, []byte(`{"servers":{}}`), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(foreign, path); err != nil {
			if os.IsPermission(err) {
				t.Skipf("symlinks unavailable on this runner: %v", err)
			}
			t.Fatal(err)
		}
		if _, err := ReadClineProjection(root); err == nil {
			t.Fatal("linked Cline projection was admitted")
		}
		body, err := os.ReadFile(foreign)
		if err != nil || string(body) != `{"servers":{}}` {
			t.Fatal("foreign projection was changed", err)
		}
	})
}
