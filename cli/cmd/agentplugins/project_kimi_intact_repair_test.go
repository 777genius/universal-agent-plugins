package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// Exercise the public App with an intact recorded artifact. The existing
// damaged-pin repair regression deliberately takes a different selection path.
func TestProjectKimiPublicIntactRepairPreservesMCPSelection(t *testing.T) {
	transport := newProjectJourneyTransport(t, "zip")
	journey := newProjectJourney(t, transport)
	user := filepath.Join(journey.home, ".kimi-code")
	journey.app = configureProjectKimiJourney(journey.app, user, filepath.Join(journey.home, "kimi"))
	for _, project := range []string{journey.a, journey.b} {
		if err := os.Mkdir(filepath.Join(project, ".git"), 0700); err != nil {
			t.Fatal(err)
		}
		projectJourneyWrite(t, filepath.Join(project, ".kimi-code", "mcp.json"), kimiIntactForeign)
		journey.run(t, project, "add", transport.repo, "--target", "kimi", "--no-save")
	}
	projectJourneyWrite(t, filepath.Join(user, "mcp.json"), kimiIntactForeign)
	installation, binding := projectKimiVersionOwned(t, journey, projectJourneySkill, false)
	projectJourneyDataMarker(t, journey, journey.a, installation)
	beforeUser, beforeB := securityFixtureTree(t, user), securityFixtureTree(t, journey.b)
	mcp := filepath.Join(journey.a, ".kimi-code", "mcp.json")
	skill := filepath.Join(journey.a, ".kimi-code", "skills", "docs", "SKILL.md")
	beforeMCP, beforeSkill := projectJourneyRead(t, mcp), projectJourneyRead(t, skill)
	output := journey.run(t, journey.a, "repair", installation.InstallationID, "--target", "kimi")
	if projectJourneyRead(t, mcp) != beforeMCP || projectJourneyRead(t, skill) != beforeSkill {
		t.Fatal("intact public repair changed native skill or MCP bytes")
	}
	if strings.Contains(output, "not_in_recorded_delivery") || strings.Contains(output, "managed_component_removal_planned") {
		t.Fatal("intact public repair discarded recorded project MCP selection")
	}
	after, repaired := projectKimiVersionOwned(t, journey, projectJourneySkill, false)
	if after.InstallationID != installation.InstallationID || !reflect.DeepEqual(after.Source, installation.Source) || repaired.ClientBindingID != binding.ClientBindingID || repaired.DataReceiptID != binding.DataReceiptID || !reflect.DeepEqual(repaired.NativeObjects, binding.NativeObjects) {
		t.Fatal("intact repair changed held installation, binding, data or native ownership")
	}
	if !reflect.DeepEqual(beforeUser, securityFixtureTree(t, user)) || !reflect.DeepEqual(beforeB, securityFixtureTree(t, journey.b)) {
		t.Fatal("intact repair changed original user or B")
	}
	for _, receipt := range installation.DataReceipts {
		if projectJourneyRead(t, filepath.Join(receipt.Locator, "keep.txt")) != "retained environment data\n" {
			t.Fatal("intact repair changed retained data")
		}
	}
}

const kimiIntactForeign = `{"mcpServers":{"foreign":{"transport":"http","url":"https://foreign.test/mcp","enabled":false}},"policy":"keep"}`
