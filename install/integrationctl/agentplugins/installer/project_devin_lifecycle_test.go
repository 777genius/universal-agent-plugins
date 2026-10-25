package installer_test

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

// Public SDK composition with real filesystem/state and inert stdio helper
// bytes. No runner, native client, launcher or MCP process is executed.
func TestProjectDevinPublicSDKStdioDataRepairRestartAndRemove(t *testing.T) {
	base := devinSDKTemp(t)
	user, pkg, helper := filepath.Join(base, "user", ".devin"), filepath.Join(base, "package"), filepath.Join(base, "inert-helper")
	devinSDKWrite(t, helper, "inert helper bytes; never executed")
	if err := os.Chmod(helper, 0700); err != nil {
		t.Fatal(err)
	}
	const original = "---\nname: docs\ndescription: Inert Devin SDK fixture\n---\nOriginal pinned bytes.\n"
	devinSDKWrite(t, filepath.Join(pkg, "plugin.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"sdk-devin","version":"1.0.0"}`)
	devinSDKWrite(t, filepath.Join(pkg, "skills", "docs", "SKILL.md"), original)
	devinSDKWrite(t, filepath.Join(pkg, "mcp.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{"docs":{"type":"stdio","command":"go","args":["${PLUGIN_ROOT}/server.js","${PLUGIN_DATA}"],"cwd":"${PLUGIN_DATA}"}}}`)
	devinSDKWrite(t, filepath.Join(pkg, "server.js"), "// Inert fixture; never executed.\n")
	const foreign = `{"mcpServers":{"foreign":{"url":"https://foreign.test","disabled":true}},"policy":"keep"}`
	devinSDKWrite(t, filepath.Join(user, "mcp_config.json"), foreign)
	a, b := newDevinSDKFixture(t, filepath.Join(base, "A"), user, pkg, helper), newDevinSDKFixture(t, filepath.Join(base, "B"), user, pkg, helper)
	for _, f := range []*devinSDKFixture{a, b} {
		devinSDKWrite(t, filepath.Join(f.project, ".devin", "mcp_config.json"), foreign)
		f.apply(t, installer.OpInstall, "install")
		binding := f.binding(t)
		if binding.Activation != domain.ActivationPrepared || binding.Verification != domain.VerificationInstalled || f.data == "" {
			t.Fatalf("Devin SDK overclaimed/omitted data: %+v", binding)
		}
		devinSDKWrite(t, filepath.Join(f.data, "keep.txt"), f.project)
		devinSDKCheckStdio(t, f)
	}
	if a.data == b.data {
		t.Fatal("project stdio data receipts alias")
	}
	beforeB := devinSDKRead(t, filepath.Join(b.project, ".uap", "state-v2.json"))
	bBinding := b.binding(t)
	devinSDKRepairUpdateRemove(t, a, pkg, helper, original)
	if beforeB != devinSDKRead(t, filepath.Join(b.project, ".uap", "state-v2.json")) || !reflect.DeepEqual(bBinding, b.binding(t)) {
		t.Fatal("Devin SDK changed project B ownership")
	}
	devinSDKCheckStdio(t, b)
	if devinSDKRead(t, filepath.Join(user, "mcp_config.json")) != foreign {
		t.Fatal("Devin SDK changed global native configuration")
	}
	for _, f := range []*devinSDKFixture{a, b} {
		if devinSDKRead(t, filepath.Join(f.data, "keep.txt")) != f.project {
			t.Fatal("SDK removed scoped persistent data")
		}
	}
}

func devinSDKCheckStdio(t *testing.T, f *devinSDKFixture) {
	t.Helper()
	binding := f.binding(t)
	servers, err := devinSDKConfig(filepath.Join(f.project, ".devin", "mcp_config.json"))
	if err != nil {
		t.Fatal(err)
	}
	entry := servers["docs"].(map[string]any)
	env := entry["env"].(map[string]any)
	args := entry["args"].([]any)
	if env["PLUGIN_ROOT"] != binding.TargetLocator || env["PLUGIN_DATA"] != f.data || len(args) < 7 || args[1] != binding.TargetLocator || args[2] != f.data || args[3] != f.data || args[4] != "data" || entry["cwd"] != nil || entry["type"] != nil {
		t.Fatalf("SDK stdio binding = %+v", entry)
	}
	if devinSDKRead(t, entry["command"].(string)) != "inert helper bytes; never executed" {
		t.Fatal("SDK launcher did not come from injected trusted helper")
	}
}

func devinSDKRepairUpdateRemove(t *testing.T, f *devinSDKFixture, pkg, helper, original string) {
	t.Helper()
	binding := f.binding(t)
	nativeSkill := filepath.Join(f.project, ".devin", "skills", "docs", "SKILL.md")
	if got := f.apply(t, installer.OpUpdate, "unchanged"); got.Mutated {
		t.Fatalf("unchanged=%+v", got)
	}
	pinned := t.TempDir()
	for _, name := range []string{"plugin.json", "mcp.json", "server.js"} {
		devinSDKWrite(t, filepath.Join(pinned, name), devinSDKRead(t, filepath.Join(pkg, name)))
	}
	devinSDKWrite(t, filepath.Join(pinned, "skills", "docs", "SKILL.md"), original)
	devinSDKWrite(t, filepath.Join(pkg, "skills", "docs", "SKILL.md"), "---\nname: docs\ndescription: Advanced Devin SDK fixture\n---\nUpdated bytes.\n")
	if err := os.RemoveAll(filepath.Dir(nativeSkill)); err != nil {
		t.Fatal(err)
	}
	f.request.PackageRoot, f.request.SourceRoot = pinned, pkg
	f.apply(t, installer.OpRepair, "repair-installed-pin")
	if devinSDKRead(t, nativeSkill) != original {
		t.Fatal("SDK repair followed moving source")
	}
	fresh := newDevinSDKFixture(t, f.project, f.user, pkg, helper)
	fresh.data = f.data
	*f = *fresh
	f.apply(t, installer.OpUpdate, "update-after-restart")
	if devinSDKRead(t, nativeSkill) == original {
		t.Fatal("SDK update failed to advance selected project")
	}
	devinSDKCheckStdio(t, f)
	view, err := f.engine.Inspect(t.Context())
	if err != nil || view.Recovery.Required {
		t.Fatalf("Devin SDK status=%+v,%v", view, err)
	}
	f.request.ClientVersion, f.request.ClientExecutable = "", ""
	f.apply(t, installer.OpRemove, "remove-without-current-version")
	if _, err := os.Stat(binding.TargetLocator); !os.IsNotExist(err) {
		t.Fatalf("SDK managed artifact survived: %v", err)
	}
	if _, err := os.Stat(nativeSkill); !os.IsNotExist(err) {
		t.Fatalf("SDK native skill survived: %v", err)
	}
	servers, err := devinSDKConfig(filepath.Join(f.project, ".devin", "mcp_config.json"))
	if err != nil || servers["docs"] != nil || servers["foreign"] == nil {
		t.Fatalf("SDK remove ownership=%+v,%v", servers, err)
	}
}
