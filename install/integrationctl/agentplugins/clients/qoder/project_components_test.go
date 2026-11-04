package qoder

import (
    "context"
    "os"
    "path/filepath"
    "strings"
    "testing"

    "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
    "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
    "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func qoderWrite(t *testing.T, path, body string) {
    t.Helper()
    if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil { t.Fatal(err) }
    if err := os.WriteFile(path, []byte(body), 0600); err != nil { t.Fatal(err) }
}

func TestQoderProjectMixedSkillsStdioHTTPNativeLifecycle(t *testing.T) {
    base, err := filepath.EvalSymlinks(t.TempDir())
    if err != nil { t.Fatal(err) }
    project, user := filepath.Join(base, "A"), filepath.Join(base, "home", ".qoder")
    active := filepath.Join(project, ".uap", "managed", "clients", "qoder", "demo")
    qoderWrite(t, filepath.Join(user, "keep"), "user-untouched")
    qoderWrite(t, filepath.Join(active, "skills", "docs", "SKILL.md"), "---\nname: docs\ndescription: native fixture\n---\n")
    qoderWrite(t, filepath.Join(active, ".mcp.json"), `{"mcpServers":{"stdio":{"command":"echo","args":["ok"]},"http":{"type":"http","url":"https://example.test/mcp"}}}`)
    path := filepath.Join(project, ".mcp.json")
    qoderWrite(t, path, `{"mcpServers":{"foreign":{"command":"foreign"}},"other":"keep"}`)
    plan := domain.DeliveryPlan{TargetContext:domain.TargetContext{ProjectRoot:project, UserConfigRoot:user}, Components:[]domain.ComponentDecision{
        {Kind:domain.ComponentSkill, Name:"docs", Support:domain.SupportNative},
        {Kind:domain.ComponentMCPServer, Name:"stdio", Support:domain.SupportNative},
        {Kind:domain.ComponentMCPServer, Name:"http", Support:domain.SupportNative},
    }}
    objects, err := qoderProjectObjects(active, plan)
    if err != nil || len(objects) != 3 { t.Fatalf("staged objects: %+v %v", objects, err) }
    layout := qoderComponents(project)
    servers, err := qoderDesiredServers(active, objects)
    if err != nil || servers["stdio"].Type != "stdio" || servers["http"].Type != "remote" {
        t.Fatalf("native servers: %+v %v", servers, err)
    }
    mutation := shared.ProjectComponentMutation{Layout:layout, ActivePath:active, Desired:objects, Servers:servers, Kernel:nativeconfig.New()}
    if err := mutation.Apply(context.Background()); err != nil { t.Fatal(err) }
    if err := layout.Verify(objects, false, mutation.Kernel); err != nil { t.Fatal(err) }
    content, err := os.ReadFile(path)
    if err != nil || !strings.Contains(string(content), `"type":"http"`) || !strings.Contains(string(content), `"other":"keep"`) {
        t.Fatalf("project MCP bytes: %s %v", content, err)
    }
    if _, err := os.Stat(filepath.Join(project, ".qoder", "skills", "docs", "SKILL.md")); err != nil { t.Fatal(err) }
    mutation.Previous = append([]domain.NativeObjectOwnership(nil), objects...)
    if err := mutation.Apply(context.Background()); err != nil { t.Fatal(err) }
    mutation.Desired, mutation.Servers = nil, nil
    if err := mutation.Apply(context.Background()); err != nil { t.Fatal(err) }
    if _, err := os.Lstat(filepath.Join(project, ".qoder", "skills", "docs")); !os.IsNotExist(err) { t.Fatalf("owned skill survives removal: %v", err) }
    content, err = os.ReadFile(path)
    if err != nil || strings.Contains(string(content), `"stdio"`) || strings.Contains(string(content), `"http"`) || !strings.Contains(string(content), `"foreign"`) {
        t.Fatalf("owned MCP removal changed foreign project state: %s %v", content, err)
    }
    data, err := os.ReadFile(filepath.Join(user, "keep"))
    if err != nil || string(data) != "user-untouched" { t.Fatal("user Qoder profile changed", err) }
}

func TestQoderProjectPriorityAndTransportNegative(t *testing.T) {
    base, _ := filepath.EvalSymlinks(t.TempDir())
    project, user := filepath.Join(base, "project"), filepath.Join(base, "user", ".qoder")
    qoderWrite(t, filepath.Join(user, "keep"), "foreign")
    plan := domain.DeliveryPlan{TargetContext:domain.TargetContext{ProjectRoot:project, UserConfigRoot:user},
        Components:[]domain.ComponentDecision{{Kind:domain.ComponentSkill, Name:"docs", Support:domain.SupportNative}, {Kind:domain.ComponentMCPServer, Name:"api", Support:domain.SupportNative}}}
    qoderWrite(t, filepath.Join(project, "keep"), "original")
    if err := qoderHigherPriorityConflicts(plan); err != nil { t.Fatal(err) }
    shadow := filepath.Join(user, "skills", "docs", "SKILL.md")
    qoderWrite(t, shadow, "foreign")
    if err := qoderHigherPriorityConflicts(plan); err == nil { t.Fatal("user Skill priority was ignored") }
    if err := os.RemoveAll(filepath.Dir(shadow)); err != nil { t.Fatal(err) }
    qoderWrite(t, filepath.Join(project, ".qoder", "settings.local.json"), `{"mcpServers":{"api":{"url":"https://foreign.test"}}}`)
    if err := qoderHigherPriorityConflicts(plan); err == nil { t.Fatal("local Qoder MCP priority was ignored") }
    for _, value := range []string{
        `{"mcpServers":{"api":{"type":"sse","url":"https://invalid.test"}}}`,
        `{"mcpServers":{"api":{"type":"http","url":"https://valid.test","trust":true}}}`,
        `{"mcpServers":{"api":{"command":"echo","command":"other"}}}`,
    } {
        qoderWrite(t, filepath.Join(project, "staging", ".mcp.json"), value)
        if _, err := qoderProjectServer(filepath.Join(project,"staging"), "api"); err == nil { t.Fatalf("unqualified staged server was accepted: %s", value) }
    }
}

func TestQoderOldPackageOnlyReceiptRemainsLegacy(t *testing.T) {
    old := []domain.NativeObjectOwnership{{Kind:"managed_package_directory"}}
    if !qoderLegacyPrepared(old) || qoderLegacyPrepared(nil) || qoderLegacyPrepared(append(old, domain.NativeObjectOwnership{Kind:qoderProjectSkillKind})) {
        t.Fatal("Qoder old and native ownership generations were confused")
    }
}
