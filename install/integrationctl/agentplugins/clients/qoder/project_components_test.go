package qoder

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func qoderWrite(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestQoderProjectMixedSkillsStdioHTTPNativeLifecycle(t *testing.T) {
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	project, user := filepath.Join(base, "A"), filepath.Join(base, "home", ".qoder")
	active := filepath.Join(project, ".uap", "managed", "clients", "qoder", "demo")
	qoderWrite(t, filepath.Join(user, "keep"), "user-untouched")
	qoderWrite(t, filepath.Join(active, "skills", "docs", "SKILL.md"), "---\nname: docs\ndescription: native fixture\n---\n")
	qoderWrite(t, filepath.Join(active, ".mcp.json"), `{"mcpServers":{"stdio":{"command":"echo","args":["ok"]},"http":{"type":"http","url":"https://example.test/mcp"}}}`)
	path := filepath.Join(project, ".mcp.json")
	qoderWrite(t, path, `{"mcpServers":{"foreign":{"command":"foreign"}},"other":"keep"}`)
	plan := domain.DeliveryPlan{TargetContext: domain.TargetContext{ProjectRoot: project, UserConfigRoot: user}, Components: []domain.ComponentDecision{
		{Kind: domain.ComponentSkill, Name: "docs", Support: domain.SupportNative},
		{Kind: domain.ComponentMCPServer, Name: "stdio", Support: domain.SupportNative},
		{Kind: domain.ComponentMCPServer, Name: "http", Support: domain.SupportNative},
	}}
	objects, err := qoderProjectObjects(active, plan)
	if err != nil || len(objects) != 3 {
		t.Fatalf("staged objects: %+v %v", objects, err)
	}
	layout := qoderComponents(project)
	servers, err := qoderDesiredServers(active, objects)
	if err != nil || servers["stdio"].Type != "stdio" || servers["http"].Type != "remote" {
		t.Fatalf("native servers: %+v %v", servers, err)
	}
	mutation := shared.ProjectComponentMutation{Layout: layout, ActivePath: active, Desired: objects, Servers: servers, Kernel: nativeconfig.New()}
	if err := mutation.Apply(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := layout.Verify(objects, false, mutation.Kernel); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(content), `"type":"http"`) || !strings.Contains(string(content), `"other":"keep"`) {
		t.Fatalf("project MCP bytes: %s %v", content, err)
	}
	if _, err := os.Stat(filepath.Join(project, ".qoder", "skills", "docs", "SKILL.md")); err != nil {
		t.Fatal(err)
	}
	mutation.Previous = append([]domain.NativeObjectOwnership(nil), objects...)
	if err := mutation.Apply(context.Background()); err != nil {
		t.Fatal(err)
	}
	// User/local policy may change after installation. It must affect future
	// selection, never the permission to remove our exact owned project bytes.
	qoderWrite(t, filepath.Join(user, "skills", "docs", "SKILL.md"), "foreign user shadow")
	local := filepath.Join(project, ".qoder", "settings.local.json")
	qoderWrite(t, local, `{"mcpServers":{"stdio":{"command":"foreign-local"}}}`)
	if err := qoderHigherPriorityConflicts(plan); err == nil {
		t.Fatal("shadowed installation was not detected before activation")
	}
	removed, err := deactivateQoderNative(context.Background(), clients.Env{NativeConfig: mutation.Kernel},
		domain.DeactivationRequest{
			TargetContext: domain.TargetContext{Scope: domain.ScopeProject, ProjectRoot: project, UserConfigRoot: user},
			NativeObjects: objects, Confirmed: true,
		})
	if err != nil || !removed.ExternalRemovalComplete {
		t.Fatalf("owned removal was blocked by later foreign priority: %+v %v", removed, err)
	}
	if data, err := os.ReadFile(filepath.Join(user, "skills", "docs", "SKILL.md")); err != nil || string(data) != "foreign user shadow" {
		t.Fatalf("remove changed foreign user Skill: %q %v", data, err)
	}
	if data, err := os.ReadFile(local); err != nil || string(data) != `{"mcpServers":{"stdio":{"command":"foreign-local"}}}` {
		t.Fatalf("remove changed foreign local MCP: %q %v", data, err)
	}
	if _, err := os.Lstat(filepath.Join(project, ".qoder", "skills", "docs")); !os.IsNotExist(err) {
		t.Fatalf("owned skill survives removal: %v", err)
	}
	content, err = os.ReadFile(path)
	if err != nil || strings.Contains(string(content), `"stdio"`) || strings.Contains(string(content), `"http"`) || !strings.Contains(string(content), `"foreign"`) {
		t.Fatalf("owned MCP removal changed foreign project state: %s %v", content, err)
	}
	data, err := os.ReadFile(filepath.Join(user, "keep"))
	if err != nil || string(data) != "user-untouched" {
		t.Fatal("user qoder profile changed", err)
	}
}

func TestQoderProjectPriorityAndTransportNegative(t *testing.T) {
	base, _ := filepath.EvalSymlinks(t.TempDir())
	project, user := filepath.Join(base, "project"), filepath.Join(base, "user", ".qoder")
	qoderWrite(t, filepath.Join(user, "keep"), "foreign")
	plan := domain.DeliveryPlan{TargetContext: domain.TargetContext{ProjectRoot: project, UserConfigRoot: user},
		Components: []domain.ComponentDecision{{Kind: domain.ComponentSkill, Name: "docs", Support: domain.SupportNative}, {Kind: domain.ComponentMCPServer, Name: "api", Support: domain.SupportNative}}}
	qoderWrite(t, filepath.Join(project, "keep"), "original")
	if err := qoderHigherPriorityConflicts(plan); err != nil {
		t.Fatal(err)
	}
	shadow := filepath.Join(user, "skills", "docs", "SKILL.md")
	qoderWrite(t, shadow, "foreign")
	if err := qoderHigherPriorityConflicts(plan); err == nil {
		t.Fatal("user Skill priority was ignored")
	}
	if err := os.RemoveAll(filepath.Dir(shadow)); err != nil {
		t.Fatal(err)
	}
	qoderWrite(t, filepath.Join(project, ".qoder", "settings.local.json"), `{"mcpServers":{"api":{"url":"https://foreign.test"}}}`)
	if err := qoderHigherPriorityConflicts(plan); err == nil {
		t.Fatal("local qoder MCP priority was ignored")
	}
	// Skills-only project operations cannot require a valid MCP settings file.
	qoderWrite(t, filepath.Join(project, ".qoder", "settings.local.json"), `{"mcpServers":`)
	skillsOnly := plan
	skillsOnly.Components = plan.Components[:1]
	if err := qoderHigherPriorityConflicts(skillsOnly); err != nil {
		t.Fatalf("skills-only delivery consulted unrelated MCP configuration: %v", err)
	}
	mcpOnly := plan
	mcpOnly.Components = plan.Components[1:]
	if err := qoderHigherPriorityConflicts(mcpOnly); err == nil {
		t.Fatal("MCP delivery ignored ambiguous higher-priority local settings")
	}
	for _, value := range []string{
		`{"mcpServers":{"api":{"type":"sse","url":"https://invalid.test"}}}`,
		`{"mcpServers":{"api":{"type":"http","url":"https://valid.test","trust":true}}}`,
		`{"mcpServers":{"api":{"command":"echo","command":"other"}}}`,
	} {
		qoderWrite(t, filepath.Join(project, "staging", ".mcp.json"), value)
		if _, err := qoderProjectServer(filepath.Join(project, "staging"), "api"); err == nil {
			t.Fatalf("unqualified staged server was accepted: %s", value)
		}
	}
}


func TestQoderProjectSingleKindNativeLifecycle(t *testing.T) {
	for _, tc := range []struct {
		name string
		kind domain.ComponentKind
	}{
		{name: "skill-only", kind: domain.ComponentSkill},
		{name: "mcp-only", kind: domain.ComponentMCPServer},
	} {
		t.Run(tc.name, func(t *testing.T) {
			base, err := filepath.EvalSymlinks(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			project, user := filepath.Join(base, "A"), filepath.Join(base, "home", ".qoder")
			active := filepath.Join(project, ".uap", "managed", "clients", "qoder", "demo")
			qoderWrite(t, filepath.Join(user, "keep"), "original user")
			qoderWrite(t, filepath.Join(project, ".mcp.json"), `{"mcpServers":{"foreign":{"command":"foreign"}},"note":"keep"}`)
			switch tc.kind {
			case domain.ComponentSkill:
				qoderWrite(t, filepath.Join(active, "skills", "docs", "SKILL.md"), "---\\nname: docs\\ndescription: only skill\\n---\\n")
			case domain.ComponentMCPServer:
				qoderWrite(t, filepath.Join(active, ".mcp.json"), `{"mcpServers":{"docs":{"type":"http","url":"https://example.test/mcp"}}}`)
			}
			plan := domain.DeliveryPlan{TargetContext: domain.TargetContext{ProjectRoot: project, UserConfigRoot: user}, Components: []domain.ComponentDecision{{Kind: tc.kind, Name: "docs", Support: domain.SupportNative}}}
			objects, err := qoderProjectObjects(active, plan)
			if err != nil || len(objects) != 1 {
				t.Fatalf("single-kind ownership: %+v %v", objects, err)
			}
			servers, err := qoderDesiredServers(active, objects)
			if err != nil {
				t.Fatal(err)
			}
			layout := qoderComponents(project)
			mutation := shared.ProjectComponentMutation{Layout: layout, ActivePath: active, Desired: objects, Servers: servers, Kernel: nativeconfig.New()}
			if err := mutation.Apply(context.Background()); err != nil {
				t.Fatal(err)
			}
			if err := layout.Verify(objects, false, mutation.Kernel); err != nil {
				t.Fatal(err)
			}
			skill := filepath.Join(project, ".qoder", "skills", "docs", "SKILL.md")
			mcp := filepath.Join(project, ".mcp.json")
			if tc.kind == domain.ComponentSkill {
				if _, err := os.Stat(skill); err != nil {
					t.Fatal(err)
				}
				if content, err := os.ReadFile(mcp); err != nil || strings.Contains(string(content), `"docs"`) {
					t.Fatalf("skill-only modified MCP: %s %v", content, err)
				}
			} else {
				if _, err := os.Lstat(skill); !os.IsNotExist(err) {
					t.Fatalf("MCP-only created Skill: %v", err)
				}
				if content, err := os.ReadFile(mcp); err != nil || !strings.Contains(string(content), `"docs"`) {
					t.Fatalf("MCP-only missing native server: %s %v", content, err)
				}
			}
			mutation.Previous, mutation.Desired, mutation.Servers = objects, nil, nil
			if err := mutation.Apply(context.Background()); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Lstat(skill); !os.IsNotExist(err) {
				t.Fatalf("owned single-kind Skill survives removal: %v", err)
			}
			content, err := os.ReadFile(mcp)
			if err != nil || strings.Contains(string(content), `"docs"`) || !strings.Contains(string(content), `"foreign"`) || !strings.Contains(string(content), `"note":"keep"`) {
				t.Fatalf("single-kind removal changed foreign MCP: %s %v", content, err)
			}
			if content, err := os.ReadFile(filepath.Join(user, "keep")); err != nil || string(content) != "original user" {
				t.Fatalf("single-kind lifecycle changed global Qoder: %s %v", content, err)
			}
		})
	}
}

func TestQoderOldPackageOnlyReceiptRemainsLegacy(t *testing.T) {
	old := make([]domain.NativeObjectOwnership, 1, 2)
	old[0] = domain.NativeObjectOwnership{Kind: "managed_package_directory"}
	if !qoderLegacyPrepared(old) || qoderLegacyPrepared(nil) || qoderLegacyPrepared(append(old, domain.NativeObjectOwnership{Kind: qoderProjectSkillKind})) {
		t.Fatal("qoder old and native ownership generations were confused")
	}
}
