package installer_test

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/kiro"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

// Public SDK composition with real filesystem/state and inert stdio helper
// bytes. No runner, native client, launcher or MCP process is executed.
func TestProjectKiroPublicSDKStdioDataRepairRestartAndRemove(t *testing.T) {
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	user, pkg, helper := filepath.Join(base, "user", ".kiro"), filepath.Join(base, "package"), filepath.Join(base, "inert-helper")
	kiroSDKWrite(t, helper, "inert helper bytes; never executed")
	if err := os.Chmod(helper, 0700); err != nil {
		t.Fatal(err)
	}
	const original = "---\nname: docs\ndescription: Inert Kiro SDK fixture\n---\nOriginal pinned bytes.\n"
	kiroSDKWrite(t, filepath.Join(pkg, "plugin.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"sdk-kiro","version":"1.0.0"}`)
	kiroSDKWrite(t, filepath.Join(pkg, "skills", "docs", "SKILL.md"), original)
	kiroSDKWrite(t, filepath.Join(pkg, "mcp.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{"docs":{"type":"stdio","command":"node","args":["${PLUGIN_ROOT}/server.js","${PLUGIN_DATA}"],"cwd":"${PLUGIN_DATA}"}}}`)
	kiroSDKWrite(t, filepath.Join(pkg, "server.js"), "// Inert fixture; never executed.\n")
	const foreign = `{"mcpServers":{"foreign":{"url":"https://foreign.test","disabled":true}},"policy":"keep"}`
	kiroSDKWrite(t, filepath.Join(user, "settings", "mcp.json"), foreign)
	a, b := newKiroSDKFixture(t, filepath.Join(base, "A"), user, pkg, helper), newKiroSDKFixture(t, filepath.Join(base, "B"), user, pkg, helper)
	for _, f := range []*kiroSDKFixture{a, b} {
		kiroSDKWrite(t, filepath.Join(f.project, ".kiro", "settings", "mcp.json"), foreign)
		f.apply(t, installer.OpInstall, "install")
		binding := f.binding(t)
		if binding.Activation != domain.ActivationPrepared || binding.Verification != domain.VerificationInstalled || f.data == "" {
			t.Fatalf("Kiro SDK overclaimed/omitted data: %+v", binding)
		}
		kiroSDKWrite(t, filepath.Join(f.data, "keep.txt"), f.project)
		kiroSDKCheckStdio(t, f)
	}
	if a.data == b.data {
		t.Fatal("project stdio data receipts alias")
	}
	beforeB := kiroSDKRead(t, filepath.Join(b.project, ".uap", "state-v2.json"))
	bBinding := b.binding(t)
	kiroSDKRepairUpdateRemove(t, a, pkg, helper, original)
	if beforeB != kiroSDKRead(t, filepath.Join(b.project, ".uap", "state-v2.json")) || !reflect.DeepEqual(bBinding, b.binding(t)) {
		t.Fatal("Kiro SDK changed project B ownership")
	}
	kiroSDKCheckStdio(t, b)
	if kiroSDKRead(t, filepath.Join(user, "settings", "mcp.json")) != foreign {
		t.Fatal("Kiro SDK changed global native configuration")
	}
	for _, f := range []*kiroSDKFixture{a, b} {
		if kiroSDKRead(t, filepath.Join(f.data, "keep.txt")) != f.project {
			t.Fatal("SDK removed scoped persistent data")
		}
	}
}

func kiroSDKCheckStdio(t *testing.T, f *kiroSDKFixture) {
	t.Helper()
	binding := f.binding(t)
	servers, _, _, _, err := kiro.ReadMCPConfig(filepath.Join(f.project, ".kiro", "settings", "mcp.json"))
	if err != nil {
		t.Fatal(err)
	}
	entry := servers["docs"].(map[string]any)
	env := entry["env"].(map[string]any)
	args := entry["args"].([]any)
	if env["PLUGIN_ROOT"] != binding.TargetLocator || env["PLUGIN_DATA"] != f.data || len(args) < 7 || args[1] != binding.TargetLocator || args[2] != f.data || args[3] != f.data || args[4] != "data" || entry["cwd"] != nil || entry["type"] != nil {
		t.Fatalf("SDK stdio binding = %+v", entry)
	}
	if kiroSDKRead(t, entry["command"].(string)) != "inert helper bytes; never executed" {
		t.Fatal("SDK launcher did not come from injected trusted helper")
	}
}

func kiroSDKRepairUpdateRemove(t *testing.T, f *kiroSDKFixture, pkg, helper, original string) {
	t.Helper()
	binding := f.binding(t)
	nativeSkill := filepath.Join(f.project, ".kiro", "skills", "docs", "SKILL.md")
	if got := f.apply(t, installer.OpUpdate, "unchanged"); got.Mutated {
		t.Fatalf("unchanged=%+v", got)
	}
	pinned, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"plugin.json", "mcp.json", "server.js"} {
		kiroSDKWrite(t, filepath.Join(pinned, name), kiroSDKRead(t, filepath.Join(pkg, name)))
	}
	kiroSDKWrite(t, filepath.Join(pinned, "skills", "docs", "SKILL.md"), original)
	kiroSDKWrite(t, filepath.Join(pkg, "skills", "docs", "SKILL.md"), "---\nname: docs\ndescription: Advanced Kiro SDK fixture\n---\nUpdated bytes.\n")
	if err := os.RemoveAll(filepath.Dir(nativeSkill)); err != nil {
		t.Fatal(err)
	}
	f.request.PackageRoot, f.request.SourceRoot = pinned, pkg
	f.apply(t, installer.OpRepair, "repair-installed-pin")
	if kiroSDKRead(t, nativeSkill) != original {
		t.Fatal("SDK repair followed moving source")
	}
	fresh := newKiroSDKFixture(t, f.project, f.user, pkg, helper)
	fresh.data = f.data
	*f = *fresh
	f.apply(t, installer.OpUpdate, "update-after-restart")
	if kiroSDKRead(t, nativeSkill) == original {
		t.Fatal("SDK update failed to advance selected project")
	}
	kiroSDKCheckStdio(t, f)
	view, err := f.engine.Inspect(t.Context())
	if err != nil || view.Recovery.Required {
		t.Fatalf("Kiro SDK status=%+v,%v", view, err)
	}
	f.apply(t, installer.OpRemove, "remove")
	if _, err := os.Stat(binding.TargetLocator); !os.IsNotExist(err) {
		t.Fatalf("SDK managed artifact survived: %v", err)
	}
	if _, err := os.Stat(nativeSkill); !os.IsNotExist(err) {
		t.Fatalf("SDK native skill survived: %v", err)
	}
	servers, _, _, _, err := kiro.ReadMCPConfig(filepath.Join(f.project, ".kiro", "settings", "mcp.json"))
	if err != nil || servers["docs"] != nil || servers["foreign"] == nil {
		t.Fatalf("SDK remove ownership=%+v,%v", servers, err)
	}
}
