package main

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// This E2E uses the real public Cobra CLI and production project composition,
// source acquisition, immutable store, planner, native filesystem and receipts.
// The injected host detection is inert. It cannot establish native agent/model
// execution, but it proves a prepared project is described truthfully and doctor
// never mutates A/B/user state while inspecting materialized project components.
func TestProjectDoctorReadinessProductionCompositionE2E(t *testing.T) {
	transport := newProjectJourneyTransport(t, "git")
	journey := newProjectJourney(t, transport)
	journey.run(t, journey.a, "add", transport.endpoint, "--alias", "docs", "--target", "gemini")
	installation, binding := journey.owned(t, journey.a)
	if binding.Activation != "prepared" {
		t.Fatalf("project fixture did not remain Prepared: %q", binding.Activation)
	}
	roots := []string{journey.a, journey.b, journey.cache}
	snapshots := make([]map[string]string, len(roots))
	for i, root := range roots {
		snapshots[i] = securityFixtureTree(t, root)
	}
	var selected struct {
		Data struct {
			ReadOnly         bool `json:"read_only"`
			ProjectReadiness []struct {
				ClientID       string   `json:"client_id"`
				InstallationID string   `json:"installation_id"`
				Status         string   `json:"status"`
				NativeEvidence string   `json:"native_evidence"`
				NextActions    []string `json:"next_actions"`
			} `json:"project_readiness"`
			Findings []struct {
				Code   string `json:"code"`
				Status string `json:"status"`
			} `json:"findings"`
		} `json:"data"`
	}
	output := journey.run(t, journey.a, "doctor")
	if err := json.Unmarshal([]byte(output), &selected); err != nil {
		t.Fatalf("public project doctor returned invalid JSON: %v", err)
	}
	if !selected.Data.ReadOnly || len(selected.Data.ProjectReadiness) != 1 {
		t.Fatalf("project doctor lost one selected project binding: %+v", selected.Data.ProjectReadiness)
	}
	entry := selected.Data.ProjectReadiness[0]
	if entry.ClientID != "gemini" || entry.InstallationID != installation.InstallationID ||
		entry.Status != "prepared_native_unverified" || entry.NativeEvidence != "native_execution_not_observed_by_doctor" ||
		len(entry.NextActions) < 2 {
		t.Fatalf("public doctor claimed a native activation or lost project identity: %+v", entry)
	}
	foundUnknown := false
	for _, finding := range selected.Data.Findings {
		if finding.Code == "project_native_unverified" && finding.Status == "unknown" {
			foundUnknown = true
		}
		if finding.Code == "activation_pending" {
			t.Fatal("normal Prepared project incorrectly marked as degraded/activation_pending")
		}
	}
	if !foundUnknown {
		t.Fatalf("public project doctor omitted truthful native-unverified warning: %+v", selected.Data.Findings)
	}
	for _, root := range []string{"", journey.b} {
		output := journey.run(t, root, "doctor")
		var other struct {
			Data struct {
				ProjectReadiness []json.RawMessage `json:"project_readiness"`
			} `json:"data"`
		}
		if err := json.Unmarshal([]byte(output), &other); err != nil {
			t.Fatal(err)
		}
		if len(other.Data.ProjectReadiness) != 0 {
			t.Fatalf("unrelated scope exposed project A's readiness report: %q", output)
		}
	}
	for i, root := range roots {
		if !reflect.DeepEqual(snapshots[i], securityFixtureTree(t, root)) {
			t.Fatalf("read-only doctor changed recorded state, user profile or sibling project at scope %d", i)
		}
	}
	for _, foreign := range []string{filepath.Join(journey.b, ".uap", "state-v2.json"), filepath.Join(journey.cache, "state-v2.json")} {
		if strings.Contains(output, foreign) {
			t.Fatal("public report revealed an unrelated state locator")
		}
	}
}
