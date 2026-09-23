package kimi

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}
func fixture(t *testing.T) domain.ActivationRequest {
	t.Helper()
	root := filepath.Join(t.TempDir(), ".kimi-code")
	active := filepath.Join(root, "plugins", "managed", "demo-artifact")
	write(t, filepath.Join(active, ".kimi-plugin", "plugin.json"), `{"name":"demo","skills":"./skills/"}`)
	return domain.ActivationRequest{Client: domain.DetectedClient{ClientID: "kimi", ConfigRoot: root}, Plan: domain.DeliveryPlan{ClientID: "kimi", DeclaredName: "demo", NativeRegistryRoot: root, ActivePath: active, Scope: domain.ScopeUser, InstallIntent: domain.InstallIntentAutomatic, Components: []domain.ComponentDecision{{Kind: domain.ComponentSkill, Name: "docs", Support: domain.SupportNative}}}, Delivery: domain.StagedDelivery{ClientID: "kimi", ActivePath: active}, DeclaredName: "demo"}
}
func TestRegistryPreservesUnknownFieldsOnUpdateAndRemoval(t *testing.T) {
	r := fixture(t)
	other := filepath.Join(t.TempDir(), "other")
	initial := map[string]any{"version": 1, "future": map[string]any{"number": 123}, "plugins": []any{
		map[string]any{"id": "demo", "root": r.Delivery.ActivePath, "source": "local-path", "enabled": true, "installedAt": "original", "capabilities": map[string]any{"mcpServers": map[string]any{"disabled": false}}, "github": map[string]any{"ref": "abc"}, "future": "retained"},
		map[string]any{"id": "other", "root": other, "enabled": false, "source": "github", "github": map[string]any{"ref": "xyz"}, "future": []any{1, "two"}}}}
	if err := shared.WriteJSON(registryPath(r.Client.ConfigRoot), initial); err != nil {
		t.Fatal(err)
	}
	before, err := readRegistry(r.Client.ConfigRoot)
	if err != nil {
		t.Fatal(err)
	}
	r.Replacing = true
	if _, err := New().Activate(context.Background(), clients.Env{}, r); err != nil {
		t.Fatal(err)
	}
	after, err := readRegistry(r.Client.ConfigRoot)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before.document["future"], after.document["future"]) || !reflect.DeepEqual(before.records[1], after.records[1]) {
		t.Fatal("unmanaged metadata changed")
	}
	managed := after.records[0].(map[string]any)
	for _, key := range []string{"installedAt", "capabilities", "github", "future"} {
		if !reflect.DeepEqual(before.records[0].(map[string]any)[key], managed[key]) {
			t.Fatalf("lost managed field %s", key)
		}
	}
	out, err := New().Deactivate(context.Background(), clients.Env{}, domain.DeactivationRequest{Client: r.Client, DeclaredName: r.DeclaredName, ManagedArtifactPath: r.Delivery.ActivePath, Confirmed: true})
	if err != nil || !out.ExternalRemovalComplete {
		t.Fatalf("remove: %+v %v", out, err)
	}
	after, err = readRegistry(r.Client.ConfigRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(after.records) != 1 || !reflect.DeepEqual(before.records[1], after.records[0]) || !reflect.DeepEqual(before.document["future"], after.document["future"]) {
		t.Fatal("removal changed unrelated data")
	}
}
func TestVerifyAndUnconfirmedRemovalNeverWrite(t *testing.T) {
	r := fixture(t)
	if _, err := New().Activate(context.Background(), clients.Env{}, r); err != nil {
		t.Fatal(err)
	}
	path := registryPath(r.Client.ConfigRoot)
	before, _ := os.ReadFile(path)
	info, _ := os.Stat(path)
	r.VerifyOnly = true
	if _, err := New().Activate(context.Background(), clients.Env{}, r); err != nil {
		t.Fatal(err)
	}
	if _, err := New().Deactivate(context.Background(), clients.Env{}, domain.DeactivationRequest{Client: r.Client, DeclaredName: "demo", ManagedArtifactPath: r.Delivery.ActivePath}); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(path)
	next, _ := os.Stat(path)
	if string(before) != string(after) || !info.ModTime().Equal(next.ModTime()) {
		t.Fatal("read-only operation changed registry")
	}
	missing := fixture(t)
	missing.VerifyOnly = true
	if _, err := New().Activate(context.Background(), clients.Env{}, missing); err == nil {
		t.Fatal("missing registration verified")
	}
	if _, err := os.Stat(registryPath(missing.Client.ConfigRoot)); !os.IsNotExist(err) {
		t.Fatal("verification created registry")
	}
	if _, err := os.Stat(registryPath(r.Client.ConfigRoot) + ".agentplugins.lock"); !os.IsNotExist(err) {
		t.Fatal("lock leaked")
	}
}

func TestPreflightAcceptsPlanBeforeDeliveryExists(t *testing.T) {
	r := fixture(t)
	r.DeclaredName = ""
	r.Delivery = domain.StagedDelivery{}
	if err := New().PreflightActivation(clients.Env{}, r); err != nil {
		t.Fatalf("preflight before staging: %v", err)
	}
}
func TestRegistryCollisionAndMalformedInputFailWithoutMutation(t *testing.T) {
	for _, kind := range []string{"foreign-root", "duplicate", "bad-version", "symlink", "same-root-other-id"} {
		t.Run(kind, func(t *testing.T) {
			r := fixture(t)
			path := registryPath(r.Client.ConfigRoot)
			rec := map[string]any{"id": "demo", "root": r.Delivery.ActivePath, "enabled": true}
			doc := map[string]any{"version": 1, "plugins": []any{rec}}
			switch kind {
			case "foreign-root":
				rec["root"] = t.TempDir()
			case "duplicate":
				doc["plugins"] = []any{rec, rec}
			case "bad-version":
				doc["version"] = 2
			case "same-root-other-id":
				rec["id"] = "other"
			}
			body, _ := json.Marshal(doc)
			if kind == "symlink" {
				other := filepath.Join(t.TempDir(), "registry")
				write(t, other, string(body))
				if err := os.Symlink(other, path); err != nil {
					t.Fatal(err)
				}
			} else {
				write(t, path, string(body))
			}
			r.Replacing = true
			if _, err := New().Activate(context.Background(), clients.Env{}, r); err == nil {
				t.Fatal("unsafe registry accepted")
			}
			after, _ := os.ReadFile(path)
			if string(after) != string(body) {
				t.Fatal("registry changed on failure")
			}
		})
	}
}
func TestProjectionUsesKimiRelativeStdioPaths(t *testing.T) {
	for _, command := range []string{"node", "./bin/server"} {
		t.Run(command, func(t *testing.T) {
			stage := t.TempDir()
			active := filepath.Join(t.TempDir(), "active")
			data := t.TempDir()
			write(t, filepath.Join(stage, "bin", "server"), "binary fixture")
			write(t, filepath.Join(stage, "mcp.json"), "{}")
			in := clients.ProjectionInput{StagingPath: stage, PluginDataPath: data, Plan: domain.DeliveryPlan{ActivePath: active, Components: []domain.ComponentDecision{{Kind: domain.ComponentMCPServer, Name: "test", Support: domain.SupportNative}}}, Envelope: domain.PackageEnvelope{MCP: domain.MCPComponent{Servers: map[string]domain.MCPServer{"test": {Type: "stdio", Decoded: map[string]any{"command": command, "args": []any{"${PLUGIN_ROOT}/x"}, "cwd": "./bin"}}}}}}
			if _, err := New().Project(context.Background(), in); err != nil {
				t.Fatal(err)
			}
			body, err := os.ReadFile(filepath.Join(stage, ".kimi-plugin", "plugin.json"))
			if err != nil {
				t.Fatal(err)
			}
			doc, err := shared.DecodeStrictJSONObject(body)
			if err != nil {
				t.Fatal(err)
			}
			server := doc["mcpServers"].(map[string]any)["test"].(map[string]any)
			if server["command"] != command || server["cwd"] != "./bin" {
				t.Fatalf("invalid Kimi paths: %+v", server)
			}
			if server["env"].(map[string]any)["PLUGIN_DATA"] != data {
				t.Fatal("data environment missing")
			}
			if _, err := os.Stat(filepath.Join(stage, "mcp.json")); !os.IsNotExist(err) {
				t.Fatal("portable MCP still present")
			}
			if in.Envelope.MCP.Servers["test"].Decoded["cwd"] != "./bin" {
				t.Fatal("envelope mutated")
			}
		})
	}
}
func TestProjectionRejectsOutsideCWDAndPriorityManifest(t *testing.T) {
	for _, kind := range []string{"data-cwd", "priority-manifest", "implicit-agents"} {
		t.Run(kind, func(t *testing.T) {
			stage := t.TempDir()
			data := t.TempDir()
			in := clients.ProjectionInput{StagingPath: stage, PluginDataPath: data, Plan: domain.DeliveryPlan{ActivePath: filepath.Join(t.TempDir(), "active"), Components: []domain.ComponentDecision{{Kind: domain.ComponentMCPServer, Name: "test", Support: domain.SupportNative}}}, Envelope: domain.PackageEnvelope{MCP: domain.MCPComponent{Servers: map[string]domain.MCPServer{"test": {Type: "stdio", Decoded: map[string]any{"command": "node", "cwd": "${PLUGIN_DATA}"}}}}}}
			if kind == "implicit-agents" {
				write(t, filepath.Join(stage, "agents", "reviewer.md"), "unsupported agent")
			}
			if kind == "priority-manifest" {
				in.Envelope.MCP.Servers["test"].Decoded["cwd"] = "./"
				write(t, filepath.Join(stage, "kimi.plugin.json"), `{"name":"foreign"}`)
			}
			if _, err := New().Project(context.Background(), in); err == nil {
				t.Fatal("unsafe projection accepted")
			}
		})
	}
}
func TestRegistryRefusesUnownedEntryAndPreservesDisabledState(t *testing.T) {
	r := fixture(t)
	if err := mutateRegistry(r.Client.ConfigRoot, "demo", r.Delivery.ActivePath, false, false, time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := New().Activate(context.Background(), clients.Env{}, r); err == nil {
		t.Fatal("unowned entry adopted")
	}
	reg, _ := readRegistry(r.Client.ConfigRoot)
	reg.records[0].(map[string]any)["enabled"] = false
	if err := shared.WriteJSON(registryPath(r.Client.ConfigRoot), reg.document); err != nil {
		t.Fatal(err)
	}
	r.Replacing = true
	if _, err := New().Activate(context.Background(), clients.Env{}, r); err == nil {
		t.Fatal("disabled plugin reported active")
	}
	reg, _ = readRegistry(r.Client.ConfigRoot)
	if reg.records[0].(map[string]any)["enabled"] != false {
		t.Fatal("user disabled state overwritten")
	}
}
