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
	installation := kimiSDKIntactInstallation(t, a)
	mcp := filepath.Join(a.project, ".kimi-code", "mcp.json")
	skill := filepath.Join(a.project, ".kimi-code", "skills", "docs", "SKILL.md")
	beforeMCP, beforeSkill := kimiSDKRead(t, mcp), kimiSDKRead(t, skill)
	beforeB, beforeUser := qoderSDKTree(t, b.project), qoderSDKTree(t, a.user)
	result := a.apply(t, installer.OpRepair, "repair-intact")
	kimiSDKIntactSelection(t, result.Delivery)
	if kimiSDKRead(t, mcp) != beforeMCP || kimiSDKRead(t, skill) != beforeSkill {
		t.Fatal("intact SDK repair changed native skill or MCP bytes")
	}
	repaired := a.binding(t)
	after := kimiSDKIntactInstallation(t, a)
	if after.InstallationID != installation.InstallationID || !reflect.DeepEqual(after.Source, installation.Source) || !reflect.DeepEqual(after.Package, installation.Package) {
		t.Fatal("intact SDK repair changed installation or pinned source identity")
	}
	if repaired.Activation != domain.ActivationPrepared || repaired.Verification != domain.VerificationInstalled || repaired.ClientBindingID != binding.ClientBindingID || repaired.PhysicalArtifact != binding.PhysicalArtifact || repaired.DataReceiptID != binding.DataReceiptID || !reflect.DeepEqual(repaired.NativeObjects, binding.NativeObjects) {
		t.Fatal("intact SDK repair changed held binding, data or native ownership")
	}
	if !reflect.DeepEqual(beforeB, qoderSDKTree(t, b.project)) || !reflect.DeepEqual(beforeUser, qoderSDKTree(t, a.user)) {
		t.Fatal("intact SDK repair changed original user or B trees")
	}
	if kimiSDKRead(t, filepath.Join(a.data, "keep.txt")) != "retained data" {
		t.Fatal("intact SDK repair changed retained data")
	}
	kimiSDKCheckStdio(t, a)
	kimiSDKCheckStdio(t, b)
}

func kimiSDKIntactInstallation(t *testing.T, f *kimiSDKFixture) domain.Installation {
	t.Helper()
	var state domain.StateFileV2
	if err := json.Unmarshal([]byte(kimiSDKRead(t, filepath.Join(f.project, ".uap", "state-v2.json"))), &state); err != nil || len(state.Installations) != 1 {
		t.Fatal("SDK did not persist one selected installation", err)
	}
	return state.Installations[0]
}

func kimiSDKIntactSelection(t *testing.T, plan *installer.DeliveryPlan) {
	t.Helper()
	if plan == nil {
		t.Fatal("intact SDK repair omitted its actual delivery plan")
	}
	for _, component := range plan.Components {
		if component.Kind == string(domain.ComponentMCPServer) && component.Name == "docs" && component.Support == string(domain.SupportNative) && component.Reason == "" {
			return
		}
	}
	t.Fatal("intact SDK repair did not retain native MCP support")
}

const kimiSDKIntactForeign = `{"mcpServers":{"original-user":{"transport":"http","url":"https://original.test/mcp"}},"preference":"keep"}`
