package main

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// Exercise the real public agentplugins CLI, source loader, project installer,
// receipt store, and OpenCode v1 native project projection in a disposable
// workspace. Only client discovery/version is an inert fixture: this does not
// claim an external model session or authentication.
func TestOpenCodeProjectDoctorProductionReadinessE2E(t *testing.T) {
	transport := newProjectJourneyTransport(t, "git")
	journey := newProjectJourney(t, transport)
	user := filepath.Join(journey.home, "opencode-user")
	journey.app = configureProjectOpenCodeJourney(journey.app, projectOpenCodeDetector{
		root: user, executable: projectOpenCodeProbeFixture(t),
	})
	const foreign = "{ // foreign settings\n\"mcp\":{\"foreign\":{\"type\":\"remote\",\"url\":\"https://foreign.invalid/mcp\",\"enabled\":false}}}\n"
	for _, root := range []string{user, journey.a, journey.b} {
		projectJourneyWrite(t, filepath.Join(root, "opencode.jsonc"), foreign)
		projectJourneyWrite(t, filepath.Join(root, ".opencode", "skills", "foreign", "SKILL.md"), "foreign project skill\n")
	}
	journey.run(t, journey.a, "add", transport.endpoint, "--alias", "docs", "--target", "opencode")
	installation, binding := projectOpenCodeOwned(t, journey, journey.a, projectJourneySkill)
	if binding.Activation != "prepared" {
		t.Fatalf("real project OpenCode delivery did not remain conservative Prepared: %s", binding.Activation)
	}
	snapshots := make(map[string]map[string]string)
	for _, root := range []string{journey.a, journey.b, user, journey.cache} {
		snapshots[root] = securityFixtureTree(t, root)
	}

	var result struct {
		Data struct {
			ReadOnly bool `json:"read_only"`
			ProjectReadiness []struct {
				InstallationID string `json:"installation_id"`
				ClientID string `json:"client_id"`
				Status string `json:"status"`
				NativeEvidence string `json:"native_evidence"`
				NextActions []string `json:"next_actions"`
			} `json:"project_readiness"`
			Findings []struct {
				Code string `json:"code"`
				Status string `json:"status"`
			} `json:"findings"`
		} `json:"data"`
	}
	report := journey.run(t, journey.a, "doctor")
	if err := json.Unmarshal([]byte(report), &result); err != nil {
		t.Fatalf("real OpenCode project doctor produced invalid JSON: %v", err)
	}
	if !result.Data.ReadOnly || len(result.Data.ProjectReadiness) != 1 {
		t.Fatalf("project doctor lost exact OpenCode binding: %+v", result.Data.ProjectReadiness)
	}
	item := result.Data.ProjectReadiness[0]
	if item.ClientID != "opencode" || item.InstallationID != installation.InstallationID ||
		item.Status != "prepared_native_unverified" || item.NativeEvidence != "native_execution_not_observed_by_doctor" ||
		!strings.Contains(strings.Join(item.NextActions, " | "), "v1/v2") {
		t.Fatalf("doctor promoted native OpenCode startup or changed v1/v2 selection: %+v", item)
	}
	foundUnknown := false
	for _, finding := range result.Data.Findings {
		if finding.Code == "project_native_unverified" && finding.Status == "unknown" {
			foundUnknown = true
		}
		if finding.Code == "activation_pending" {
			t.Fatalf("healthy Prepared OpenCode project was reported as broken: %s", report)
		}
	}
	if !foundUnknown {
		t.Fatalf("doctor omitted native-unverified project status: %+v", result.Data.Findings)
	}
	for _, scope := range []string{"", journey.b} {
		other := journey.run(t, scope, "doctor")
		var parsed struct {
			Data struct {
				ProjectReadiness []json.RawMessage `json:"project_readiness"`
			} `json:"data"`
		}
		if err := json.Unmarshal([]byte(other), &parsed); err != nil {
			t.Fatal(err)
		}
		if len(parsed.Data.ProjectReadiness) != 0 {
			t.Fatalf("OpenCode project A readiness leaked into unrelated scope: %s", other)
		}
	}
	for root, before := range snapshots {
		if !reflect.DeepEqual(before, securityFixtureTree(t, root)) {
			t.Fatalf("read-only OpenCode doctor modified original user/project state at scope %s", filepath.Base(root))
		}
	}
}
