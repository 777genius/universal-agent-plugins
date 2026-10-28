package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
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
	projectKimiIntactRepairSelection(t, output, installation.InstallationID)
	if projectJourneyRead(t, mcp) != beforeMCP || projectJourneyRead(t, skill) != beforeSkill {
		t.Fatal("intact public repair changed native skill or MCP bytes")
	}
	if strings.Contains(output, "not_in_recorded_delivery") || strings.Contains(output, "managed_component_removal_planned") {
		t.Fatal("intact public repair discarded recorded project MCP selection")
	}
	after, repaired := projectKimiVersionOwned(t, journey, projectJourneySkill, false)
	if after.InstallationID != installation.InstallationID || !reflect.DeepEqual(after.Source, installation.Source) || !reflect.DeepEqual(after.Package, installation.Package) || repaired.ClientBindingID != binding.ClientBindingID || repaired.PhysicalArtifact != binding.PhysicalArtifact || repaired.DataReceiptID != binding.DataReceiptID || !reflect.DeepEqual(repaired.NativeObjects, binding.NativeObjects) {
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

func projectKimiIntactRepairSelection(t *testing.T, output, installationID string) {
	t.Helper()
	var envelope struct {
		Data struct {
			Targets []struct {
				Target string `json:"target"`
				Output struct {
					Result struct {
						InstallationID string              `json:"installation_id"`
						Plan           domain.DeliveryPlan `json:"plan"`
					} `json:"result"`
				} `json:"output"`
			} `json:"targets"`
		} `json:"data"`
	}
	// The journey helper appends diagnostic stderr after the JSON stdout.
	if err := json.NewDecoder(strings.NewReader(output)).Decode(&envelope); err != nil || len(envelope.Data.Targets) != 1 {
		t.Fatal("public repair did not return one target result", err)
	}
	target := envelope.Data.Targets[0]
	plan := target.Output.Result.Plan
	if target.Target != "kimi" || target.Output.Result.InstallationID != installationID || plan.Scope != domain.ScopeProject || plan.ClientID != domain.ClientKimi {
		t.Fatal("public repair changed explicit selected identity")
	}
	for _, component := range plan.Components {
		if component.Kind == domain.ComponentMCPServer && component.Name == "docs" && component.Support == domain.SupportNative && component.Reason == "" {
			return
		}
	}
	t.Fatal("public repair did not preserve the recorded native MCP component")
}

const kimiIntactForeign = `{"mcpServers":{"foreign":{"transport":"http","url":"https://foreign.test/mcp","enabled":false}},"policy":"keep"}`
