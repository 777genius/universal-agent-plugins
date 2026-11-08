package report

import (
	"encoding/json"
	"reflect"
	"testing"
)

// Exercise every public finding-ID reference, not only the primary policy lanes.
// A public projection must not change the original report's private identities.
func TestPublicProjectionRemapsAllPolicyAndCheckReferences(t *testing.T) {
	r := New("validate", "revision")
	missing := r.add(Finding{Code: "plugin_manifest_missing", Layer: "normative", Rule: "core", Location: "plugin.json", Severity: "error"})
	other := r.add(Finding{Code: "mcp_server_invalid", Layer: "normative", Rule: "mcp/server", Severity: "error"})
	assessment := Assessment{Status: Fail, FindingIDs: []string{other, missing, missing}}
	r.Loadability = assessment
	r.Conformance = assessment
	r.HostSafety = assessment
	r.Readiness = assessment
	r.Release = assessment
	r.Runtime = assessment
	r.Compatibility = assessment
	r.Toolchain = assessment
	r.Checks = []Check{{"portable_configuration", assessment}}
	before, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	p := r.PublicResult("author.validate", "read", true, nil)
	var publicMissing string
	for _, f := range p.Findings {
		if f.Code == "missing_standard_manifest" {
			publicMissing = f.ID
		}
	}
	if publicMissing == "" || publicMissing == missing {
		t.Fatal("missing public manifest finding replacement")
	}
	expected := []string{other, publicMissing, publicMissing}
	for _, a := range []Assessment{p.Loadability, p.Conformance, p.HostSafety, p.Readiness, p.Release, p.Runtime, p.Compatibility, p.Toolchain, p.Checks[0].Assessment} {
		if !reflect.DeepEqual(a.FindingIDs, expected) {
			t.Fatalf("unmapped public references: %+v", a)
		}
	}
	after, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("public projection modified private policy evidence")
	}
}

// Combined error, missing-manifest and legacy advice must retain ordering,
// and an unsafe pathname must only appear through its deterministic digest.
func TestPublicProjectionWithholdsPathsAndPreservesAdviceOrder(t *testing.T) {
	r := New("validate", "revision")
	missing := r.add(Finding{Code: "plugin_manifest_missing", Layer: "normative", Rule: "core", Location: "plugin.json", Severity: "error"})
	r.Conformance = Assessment{Status: Fail, FindingIDs: []string{missing}}
	r.Readiness = r.Conformance
	r.Legacy = "present"
	r.Paths = []string{"plugin.json", "skills/my-token/SKILL.md", "skills//SKILL.md"}
	r.AddError("operation_refused", "Use exact inputs.")
	p := r.PublicResult("author.validate", "read", false, nil)
	if !reflect.DeepEqual(p.Paths, []string{"plugin.json"}) ||
		!reflect.DeepEqual(p.WithheldPathIDs, []string{opaque(r.Paths[1]), opaque(r.Paths[2])}) {
		t.Fatal("public path projection changed withholding or path order")
	}
	codes := make([]string, 0, len(p.NextActions))
	for _, action := range p.NextActions {
		codes = append(codes, action.Code)
	}
	if !reflect.DeepEqual(codes, []string{"operation_refused", "missing_standard_manifest", "legacy_workflow_unavailable"}) {
		t.Fatalf("public action precedence changed: %v", codes)
	}
	encoded, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	if string(encoded) == "" {
		t.Fatal("missing projected response")
	}
}
