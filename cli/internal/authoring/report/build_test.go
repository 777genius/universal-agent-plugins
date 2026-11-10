package report

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/project"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packageview"
)

// TestBuildAcquisitionAndReleaseIsolation pins retained host evidence boundaries
// without opening a project, running a toolchain, or exposing private path text.
func TestBuildAcquisitionAndReleaseIsolation(t *testing.T) {
	const privateLocation = "skills/token-private/SKILL.md"
	p := project.Result{Input: packageview.Input{
		Legacy: packageview.Present,
		Findings: []packageview.Finding{
			{Layer: "host", Code: "legacy_manifest_ignored", Location: privateLocation},
		},
	}}

	r := Build("validate", "revision", p, true)
	if r.HostSafety.Status != NotEvaluated || r.Loadability.Status != Fail ||
		r.Readiness.Status != Fail || r.Release.Status != Fail {
		t.Fatalf("policy boundaries changed: host=%s load=%s ready=%s release=%s",
			r.HostSafety.Status, r.Loadability.Status, r.Readiness.Status, r.Release.Status)
	}
	if len(r.Findings) != 2 {
		t.Fatalf("expected acquisition warning and release-only rejection, got %v", r.Findings)
	}

	var acquisition, release Finding
	for _, f := range r.Findings {
		switch f.Code {
		case "legacy_manifest_ignored":
			acquisition = f
		case "legacy_release_rejected":
			release = f
		}
	}
	if acquisition.ID == "" || acquisition.Layer != "host_safety" ||
		acquisition.Severity != "warning" || acquisition.Location != "" ||
		acquisition.ItemID != opaque(privateLocation) ||
		release.ID == "" || release.Layer != "release_policy" {
		t.Fatalf("incorrect acquisition/release findings: %v", r.Findings)
	}
	if !slices.Contains(r.Readiness.FindingIDs, acquisition.ID) ||
		slices.Contains(r.Readiness.FindingIDs, release.ID) ||
		!slices.Contains(r.Release.FindingIDs, acquisition.ID) ||
		!slices.Contains(r.Release.FindingIDs, release.ID) {
		t.Fatalf("policy references crossed boundaries: ready=%v release=%v",
			r.Readiness.FindingIDs, r.Release.FindingIDs)
	}
	body, err := json.Marshal(r)
	if err != nil || strings.Contains(string(body), privateLocation) {
		t.Fatalf("private source location escaped report: %v", err)
	}

	originalReadiness := slices.Clone(r.Readiness.FindingIDs)
	r.Release.FindingIDs[0] = "mutation-at-release-only"
	if !slices.Equal(r.Readiness.FindingIDs, originalReadiness) {
		t.Fatal("release evidence aliases readiness evidence")
	}

	withoutRelease := Build("validate", "revision", p, false)
	if withoutRelease.Release.Status != NotEvaluated || len(withoutRelease.Findings) != 1 {
		t.Fatalf("non-release command acquired release-only diagnostics: %+v", withoutRelease)
	}

	p.Input.Findings = append(p.Input.Findings,
		packageview.Finding{Layer: "host", Code: "skills_unreadable", Location: privateLocation})
	withError := Build("validate", "revision", p, false)
	if withError.HostSafety.Status != Fail || withError.Readiness.Status != Fail {
		t.Fatalf("fatal host finding was softened by incomplete inventory: %+v", withError.HostSafety)
	}
}
