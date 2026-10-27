package installer_test

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

func kimiSDKIntactPair(t *testing.T) (*kimiSDKFixture, *kimiSDKFixture) {
	t.Helper()
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	user, pkg, helper := filepath.Join(base, "user"), filepath.Join(base, "package"), filepath.Join(base, "inert-helper")
	kimiSDKWrite(t, helper, "inert helper bytes; never executed")
	if err := os.Chmod(helper, 0700); err != nil {
		t.Fatal(err)
	}
	kimiSDKWrite(t, filepath.Join(pkg, "plugin.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"sdk-kimi","version":"1.0.0"}`)
	kimiSDKWrite(t, filepath.Join(pkg, "skills", "docs", "SKILL.md"), "---\nname: docs\ndescription: Intact repair\n---\nHeld skill bytes.\n")
	kimiSDKWrite(t, filepath.Join(pkg, "mcp.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{"docs":{"type":"stdio","command":"node","args":["${PLUGIN_ROOT}/server.js"],"cwd":"${PLUGIN_DATA}"}}}`)
	kimiSDKWrite(t, filepath.Join(pkg, "server.js"), "inert source; never executed\n")
	a, b := newKimiSDKFixture(t, filepath.Join(base, "A"), user, pkg, helper), newKimiSDKFixture(t, filepath.Join(base, "B"), user, pkg, helper)
	for _, f := range []*kimiSDKFixture{a, b} {
		kimiSDKWrite(t, filepath.Join(f.project, ".kimi-code", "mcp.json"), `{"mcpServers":{"foreign":{"transport":"http","url":"https://foreign.test/mcp"}}}`)
		f.apply(t, installer.OpInstall, "install-intact")
		kimiSDKWrite(t, filepath.Join(f.data, "keep.txt"), "retained data")
	}
	kimiSDKWrite(t, filepath.Join(user, "mcp.json"), kimiSDKIntactForeign)
	return a, b
}

func TestProjectKimiSDKIntactRepairPreservesMCPSelection(t *testing.T) {
	a, b := kimiSDKIntactPair(t)
	binding := a.binding(t)
	mcp := filepath.Join(a.project, ".kimi-code", "mcp.json")
	skill := filepath.Join(a.project, ".kimi-code", "skills", "docs", "SKILL.md")
	beforeMCP, beforeSkill := kimiSDKRead(t, mcp), kimiSDKRead(t, skill)
	beforeB := kimiSDKRead(t, filepath.Join(b.project, ".uap", "state-v2.json"))
	result := a.apply(t, installer.OpRepair, "repair-intact")
	if !result.Mutated || result.Client.Activation != string(domain.ActivationPrepared) {
		t.Fatal("intact repair lost its actual prepared mutation outcome")
	}
	if kimiSDKRead(t, mcp) != beforeMCP || kimiSDKRead(t, skill) != beforeSkill {
		t.Fatal("intact SDK repair changed native skill or MCP bytes")
	}
	repaired := a.binding(t)
	if repaired.ClientBindingID != binding.ClientBindingID || repaired.PhysicalArtifact != binding.PhysicalArtifact || repaired.DataReceiptID != binding.DataReceiptID || !reflect.DeepEqual(repaired.NativeObjects, binding.NativeObjects) {
		t.Fatal("intact SDK repair changed held binding, data or native ownership")
	}
	if kimiSDKRead(t, filepath.Join(b.project, ".uap", "state-v2.json")) != beforeB || kimiSDKRead(t, filepath.Join(a.user, "mcp.json")) != kimiSDKIntactForeign || kimiSDKRead(t, filepath.Join(a.data, "keep.txt")) != "retained data" {
		t.Fatal("intact SDK repair changed user, B or retained data")
	}
	kimiSDKCheckStdio(t, a)
	kimiSDKCheckStdio(t, b)
}

const kimiSDKIntactForeign = `{"mcpServers":{"original-user":{"transport":"http","url":"https://original.test/mcp"}},"preference":"keep"}`
