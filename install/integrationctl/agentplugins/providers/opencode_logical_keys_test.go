package providers

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/loader"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/specregistry"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers/nativeconfig"
)

// These are configuration identities, not filesystem names. Native tool-name
// normalization and blank-key support are separate from this lifecycle contract.
var openCodeLogicalKeys = []string{"api/server", "api server", "con", `api"server`, `api\server`, "сервер", "api-server"}

func TestOpenCodeLogicalKeysLifecycle(t *testing.T) {
	for _, ext := range []string{"json", "jsonc"} {
		t.Run(ext, func(t *testing.T) {
			root := t.TempDir()
			configRoot, active := filepath.Join(root, "config"), filepath.Join(root, "managed", "demo")
			configPath := filepath.Join(configRoot, "opencode."+ext)
			original := `{"theme":"night","mcp":{"foreign":{"type":"remote","url":"https://foreign.test"}}}`
			if ext == "jsonc" {
				original = "// retained comment\n" + original
			}
			writeOpenCodeTestFile(t, configPath, original)
			build := func(version string) []domain.NativeObjectOwnership {
				envelope, plan := openCodeTestPackage(t, active, configRoot, version)
				envelope.MCP.Servers = map[string]domain.MCPServer{}
				plan.Components = plan.Components[:1]
				for _, name := range openCodeLogicalKeys {
					envelope.MCP.Servers[name] = domain.MCPServer{Name: name, Type: "streamable-http", Decoded: map[string]any{"url": "https://example.test/" + version}}
					plan.Components = append(plan.Components, domain.ComponentDecision{Kind: domain.ComponentMCPServer, Name: name, Support: domain.SupportPrepared})
				}
				if err := os.Remove(filepath.Join(active, openCodeProjectionFile)); err != nil {
					t.Fatal(err)
				}
				if err := projectOpenCodeNative(active, envelope, plan, filepath.Join(root, "data")); err != nil {
					t.Fatal(err)
				}
				objects, err := buildOpenCodeNativeObjects(active, envelope, plan)
				if err != nil {
					t.Fatal(err)
				}
				// Persisted ownership retains exact historical IDs, without new encoding.
				raw, err := json.Marshal(objects)
				if err != nil {
					t.Fatal(err)
				}
				var restored []domain.NativeObjectOwnership
				if err := json.Unmarshal(raw, &restored); err != nil {
					t.Fatal(err)
				}
				seen := map[string]bool{}
				for _, obj := range restored {
					if obj.Kind != openCodeMCPObjectKind {
						continue
					}
					if obj.ObjectID != "opencode-mcp:"+obj.LogicalName || obj.Path != configPath {
						t.Fatalf("changed identity: %+v", obj)
					}
					seen[obj.LogicalName] = true
				}
				for _, name := range openCodeLogicalKeys {
					if !seen[name] {
						t.Fatalf("lost key %q", name)
					}
				}
				return restored
			}
			first := build("v1")
			if err := applyOpenCodeNative(configRoot, active, nil, first); err != nil {
				t.Fatal(err)
			}
			if err := verifyOpenCodeNativeObjects(configRoot, active, first); err != nil {
				t.Fatal(err)
			}
			second := build("v2")
			if err := applyOpenCodeNative(configRoot, active, first, second); err != nil {
				t.Fatal(err)
			}
			if err := verifyOpenCodeNativeObjects(configRoot, active, second); err != nil {
				t.Fatal(err)
			}
			// Exact repair recreates absent entries while preserving the foreign entry.
			writeOpenCodeTestFile(t, configPath, original)
			if err := applyOpenCodeNative(configRoot, active, second, second); err != nil {
				t.Fatal(err)
			}
			if err := verifyOpenCodeNativeObjects(configRoot, active, second); err != nil {
				t.Fatal(err)
			}
			if err := applyOpenCodeNative(configRoot, "", second, nil); err != nil {
				t.Fatal(err)
			}
			body := readOpenCodeTestFile(t, configPath)
			if !strings.Contains(body, "foreign.test") || !strings.Contains(body, "night") || (ext == "jsonc" && !strings.Contains(body, "retained comment")) {
				t.Fatalf("foreign content lost: %s", body)
			}
			for _, obj := range second {
				if obj.Kind != openCodeMCPObjectKind {
					continue
				}
				present, _, err := nativeconfig.New().Inspect(nativeconfig.Paths{JSON: filepath.Join(configRoot, "opencode.json"), JSONC: filepath.Join(configRoot, "opencode.jsonc")}, nativeconfig.CodecOpenCode, obj.LogicalName, nil)
				if err != nil || present {
					t.Fatalf("key survived remove: %q %v", obj.LogicalName, err)
				}
			}
			if _, err := os.Stat(filepath.Join(configRoot, "skills", "docs")); !os.IsNotExist(err) {
				t.Fatalf("skill survived remove: %v", err)
			}
		})
	}
}

func TestOpenCodeLogicalKeysStageKeepsHealthySibling(t *testing.T) {
	registry, err := specregistry.New()
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range openCodeLogicalKeys {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			source := filepath.Join(root, "source")
			manifest, _ := json.Marshal(map[string]any{"$schema": domain.PluginSchemaV1, "name": "probe"})
			writeOpenCodeTestFile(t, filepath.Join(source, "plugin.json"), string(manifest))
			mcp, _ := json.Marshal(map[string]any{"$schema": domain.MCPSchemaV1, "mcpServers": map[string]any{name: map[string]any{"type": "streamable-http", "url": "https://example.test/mcp"}}})
			writeOpenCodeTestFile(t, filepath.Join(source, "mcp.json"), string(mcp))
			writeOpenCodeTestFile(t, filepath.Join(source, "skills", "healthy", "SKILL.md"), "---\nname: healthy\ndescription: Healthy\n---\n")
			envelope, err := (loader.Loader{Registry: registry}).Load(context.Background(), domain.LoadInput{SnapshotRoot: source})
			if err != nil {
				t.Fatal(err)
			}
			anchor := filepath.Join(root, "managed")
			target := filepath.Join(anchor, "clients", "opencode")
			plan := domain.DeliveryPlan{ClientID: domain.ClientOpenCode, Scope: domain.ScopeUser, Status: domain.PlanReady, PackageMode: domain.PackagePrepared, PhysicalArtifactID: "probe", TargetAnchor: anchor, TargetRoot: target, ActivePath: filepath.Join(target, "probe"), NativeRegistryRoot: filepath.Join(root, "native"), Components: []domain.ComponentDecision{{Kind: domain.ComponentSkill, Name: "healthy", Support: domain.SupportPrepared}, {Kind: domain.ComponentMCPServer, Name: name, Support: domain.SupportPrepared}}}
			delivery, err := (Stager{}).Stage(context.Background(), envelope, plan, "logical-key", domain.CompatibilityHints{})
			if err != nil {
				t.Fatal(err)
			}
			defer (Stager{}).Discard(context.Background(), delivery)
			if _, err := os.Stat(filepath.Join(delivery.StagingPath, "skills", "healthy", "SKILL.md")); err != nil {
				t.Fatal(err)
			}
			found := false
			for _, obj := range delivery.NativeObjects {
				if obj.Kind == openCodeMCPObjectKind && obj.LogicalName == name {
					found = true
				}
			}
			if !found {
				t.Fatalf("stage lost exact MCP key %q", name)
			}
		})
	}
}

func TestOpenCodeLogicalKeysForeignCollisionAndDrift(t *testing.T) {
	for _, ext := range []string{"json", "jsonc"} {
		for _, name := range openCodeLogicalKeys {
			t.Run(ext+"/"+name, func(t *testing.T) {
				root := t.TempDir()
				configRoot, active := filepath.Join(root, "config"), filepath.Join(root, "managed")
				path := filepath.Join(configRoot, "opencode."+ext)
				writeOpenCodeTestFile(t, path, `{ "mcp": {} }`)
				envelope, plan := openCodeTestPackage(t, active, configRoot, "owned")
				server := envelope.MCP.Servers["docs"]
				server.Name = name
				envelope.MCP.Servers = map[string]domain.MCPServer{name: server}
				plan.Components[1].Name = name
				if err := os.Remove(filepath.Join(active, openCodeProjectionFile)); err != nil {
					t.Fatal(err)
				}
				if err := projectOpenCodeNative(active, envelope, plan, filepath.Join(root, "data")); err != nil {
					t.Fatal(err)
				}
				objects, err := buildOpenCodeNativeObjects(active, envelope, plan)
				if err != nil {
					t.Fatal(err)
				}
				foreign, _ := json.Marshal(map[string]any{"mcp": map[string]any{name: map[string]any{"type": "local", "command": []string{"foreign"}}}})
				writeOpenCodeTestFile(t, path, string(foreign))
				if err := applyOpenCodeNative(configRoot, active, nil, objects); !errors.Is(err, nativeconfig.ErrCollision) {
					t.Fatalf("collision accepted: %v", err)
				}
				if got := readOpenCodeTestFile(t, path); got != string(foreign) {
					t.Fatal("collision changed foreign bytes")
				}
				if _, err := os.Stat(filepath.Join(configRoot, "skills", "docs")); !os.IsNotExist(err) {
					t.Fatal("collision installed sibling skill")
				}
				writeOpenCodeTestFile(t, path, `{"mcp":{}}`)
				if err := applyOpenCodeNative(configRoot, active, nil, objects); err != nil {
					t.Fatal(err)
				}
				writeOpenCodeTestFile(t, path, string(foreign))
				for _, desired := range [][]domain.NativeObjectOwnership{objects, nil} {
					if err := applyOpenCodeNative(configRoot, active, objects, desired); !errors.Is(err, nativeconfig.ErrNotOwned) {
						t.Fatalf("drift accepted: %v", err)
					}
					if got := readOpenCodeTestFile(t, path); got != string(foreign) {
						t.Fatal("drift changed foreign bytes")
					}
				}
			})
		}
	}
}
