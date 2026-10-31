package installer_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

// Public SDK composition with real filesystem/state and inert stdio helper
// bytes. No runner, native client, launcher or MCP process is executed.
func TestProjectKimiPublicSDKStdioDataRepairRestartAndRemove(t *testing.T) {
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	user, pkg, helper := filepath.Join(base, "user", ".kimi-code"), filepath.Join(base, "package"), filepath.Join(base, "inert-helper")
	kimiSDKWrite(t, helper, "inert helper bytes; never executed")
	if err := os.Chmod(helper, 0700); err != nil {
		t.Fatal(err)
	}
	const original = "---\nname: docs\ndescription: Inert Kimi SDK fixture\n---\nOriginal pinned bytes.\n"
	kimiSDKWrite(t, filepath.Join(pkg, "plugin.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"sdk-kimi","version":"1.0.0"}`)
	kimiSDKWrite(t, filepath.Join(pkg, "skills", "docs", "SKILL.md"), original)
	kimiSDKWrite(t, filepath.Join(pkg, "mcp.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{"docs":{"type":"stdio","command":"node","args":["${PLUGIN_ROOT}/server.js","${PLUGIN_DATA}"],"cwd":"${PLUGIN_DATA}"}}}`)
	kimiSDKWrite(t, filepath.Join(pkg, "server.js"), "// Inert fixture; never executed.\n")
	const foreign = `{"mcpServers":{"foreign":{"url":"https://foreign.test","enabled":false}},"policy":"keep"}`
	kimiSDKWrite(t, filepath.Join(user, "mcp.json"), foreign)
	a, b := newKimiSDKFixture(t, filepath.Join(base, "A"), user, pkg, helper), newKimiSDKFixture(t, filepath.Join(base, "B"), user, pkg, helper)
	for _, f := range []*kimiSDKFixture{a, b} {
		kimiSDKWrite(t, filepath.Join(f.project, ".kimi-code", "mcp.json"), foreign)
		f.apply(t, installer.OpInstall, "install")
		binding := f.binding(t)
		if binding.Activation != domain.ActivationPrepared || binding.Verification != domain.VerificationInstalled || f.data == "" {
			t.Fatalf("Kimi SDK overclaimed/omitted data: %+v", binding)
		}
		kimiSDKWrite(t, filepath.Join(f.data, "keep.txt"), f.project)
		kimiSDKCheckStdio(t, f)
	}
	if a.data == b.data {
		t.Fatal("project stdio data receipts alias")
	}
	beforeB := kimiSDKRead(t, filepath.Join(b.project, ".uap", "state-v2.json"))
	bBinding := b.binding(t)
	kimiSDKRepairUpdateRemove(t, a, pkg, helper, original)
	if beforeB != kimiSDKRead(t, filepath.Join(b.project, ".uap", "state-v2.json")) || !reflect.DeepEqual(bBinding, b.binding(t)) {
		t.Fatal("Kimi SDK changed project B ownership")
	}
	kimiSDKCheckStdio(t, b)
	if kimiSDKRead(t, filepath.Join(user, "mcp.json")) != foreign {
		t.Fatal("Kimi SDK changed global native configuration")
	}
	for _, f := range []*kimiSDKFixture{a, b} {
		if kimiSDKRead(t, filepath.Join(f.data, "keep.txt")) != f.project {
			t.Fatal("SDK removed scoped persistent data")
		}
	}
}

func kimiSDKCheckStdio(t *testing.T, f *kimiSDKFixture) {
	t.Helper()
	binding := f.binding(t)
	servers, err := kimiSDKServers(filepath.Join(f.project, ".kimi-code", "mcp.json"))
	if err != nil {
		t.Fatal(err)
	}
	entry := servers["docs"].(map[string]any)
	env := entry["env"].(map[string]any)
	args := entry["args"].([]any)
	if env["PLUGIN_ROOT"] != binding.TargetLocator || env["PLUGIN_DATA"] != f.data || len(args) < 7 || args[1] != binding.TargetLocator || args[2] != f.data || args[3] != f.data || args[4] != "data" || entry["cwd"] != nil || entry["transport"] != "stdio" {
		t.Fatalf("SDK stdio binding = %+v", entry)
	}
	if kimiSDKRead(t, entry["command"].(string)) != "inert helper bytes; never executed" {
		t.Fatal("SDK launcher did not come from injected trusted helper")
	}
}

func kimiSDKRepairUpdateRemove(t *testing.T, f *kimiSDKFixture, pkg, helper, original string) {
	t.Helper()
	binding := f.binding(t)
	nativeSkill := filepath.Join(f.project, ".kimi-code", "skills", "docs", "SKILL.md")
	if got := f.apply(t, installer.OpUpdate, "unchanged"); got.Mutated {
		t.Fatalf("unchanged=%+v", got)
	}
	pinned, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"plugin.json", "mcp.json", "server.js"} {
		kimiSDKWrite(t, filepath.Join(pinned, name), kimiSDKRead(t, filepath.Join(pkg, name)))
	}
	kimiSDKWrite(t, filepath.Join(pinned, "skills", "docs", "SKILL.md"), original)
	kimiSDKWrite(t, filepath.Join(pkg, "skills", "docs", "SKILL.md"), "---\nname: docs\ndescription: Advanced Kimi SDK fixture\n---\nUpdated bytes.\n")
	if err := os.RemoveAll(filepath.Dir(nativeSkill)); err != nil {
		t.Fatal(err)
	}
	f.request.PackageRoot, f.request.SourceRoot = pinned, pkg
	f.apply(t, installer.OpRepair, "repair-installed-pin")
	if kimiSDKRead(t, nativeSkill) != original {
		t.Fatal("SDK repair followed moving source")
	}
	fresh := newKimiSDKFixture(t, f.project, f.user, pkg, helper)
	fresh.data = f.data
	*f = *fresh
	f.apply(t, installer.OpUpdate, "update-after-restart")
	if kimiSDKRead(t, nativeSkill) == original {
		t.Fatal("SDK update failed to advance selected project")
	}
	kimiSDKCheckStdio(t, f)
	view, err := f.engine.Inspect(t.Context())
	if err != nil || view.Recovery.Required {
		t.Fatalf("Kimi SDK status=%+v,%v", view, err)
	}
	f.request.ClientVersion, f.request.ClientExecutable = "", ""
	f.apply(t, installer.OpRemove, "remove")
	if _, err := os.Stat(binding.TargetLocator); !os.IsNotExist(err) {
		t.Fatalf("SDK managed artifact survived: %v", err)
	}
	if _, err := os.Stat(nativeSkill); !os.IsNotExist(err) {
		t.Fatalf("SDK native skill survived: %v", err)
	}
	servers, err := kimiSDKServers(filepath.Join(f.project, ".kimi-code", "mcp.json"))
	if err != nil || servers["docs"] != nil || servers["foreign"] == nil {
		t.Fatalf("SDK remove ownership=%+v,%v", servers, err)
	}
}

func kimiSDKServers(path string) (map[string]any, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var document struct {
		Servers map[string]any `json:"mcpServers"`
	}
	err = json.Unmarshal(body, &document)
	return document.Servers, err
}
