//go:build linux

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// Shared with the tagged native journey: installation must retain this untrusted
// stdio server and allocate its persistent data without starting the command.
const grokNativePreparedMCP = `{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{"docs":{"type":"stdio","command":"false","env":{"DATA":"${PLUGIN_DATA}"}}}}`

func TestProjectGrokNativeFixtureAddKeepsPreparedData(t *testing.T) {
	transport := newProjectJourneyTransport(t, "zip")
	projectJourneyWrite(t, filepath.Join(transport.repo, "mcp.json"), grokNativePreparedMCP)
	transport.mu.Lock()
	transport.archive = projectJourneyArchive(t, transport.repo)
	transport.mu.Unlock()
	journey := newProjectJourney(t, transport)
	runner := &projectGrokRunner{t: t, executable: filepath.Join(journey.home, "grok-not-executed"), version: "1.0.70"}
	journey.app = configureProjectGrokJourney(journey.app, filepath.Join(journey.home, ".grok"), runner)
	journey.run(t, journey.a, "add", transport.endpoint, "--alias", "docs", "--target", "grok")
	installation, binding := journey.owned(t, journey.a)
	receipt := installation.DataReceipts[binding.DataReceiptID]
	if len(installation.DataReceipts) != 1 || receipt.DataReceiptID == "" || receipt.State != domain.DataReceiptOwned || receipt.Scope != "project" || receipt.PhysicalBackend != binding.PhysicalArtifact || receipt.Locator != filepath.Join(journey.a, ".uap", "plugin-data", binding.PhysicalArtifact) {
		t.Fatalf("native fixture lost exact persistent data authority: %+v", receipt)
	}
	var document struct {
		Servers map[string]struct {
			Env map[string]string `json:"env"`
		} `json:"mcpServers"`
	}
	body, err := os.ReadFile(filepath.Join(binding.TargetLocator, ".mcp.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(body, &document); err != nil || len(document.Servers) != 1 || document.Servers["docs"].Env["DATA"] != receipt.Locator {
		t.Fatalf("native fixture stdio data differs: %s %v", body, err)
	}
	if binding.Activation != domain.ActivationPrepared {
		t.Fatalf("initial fixture add lost Prepared state: %s", binding.Activation)
	}
}
