package report

import (
	"encoding/json"
	"reflect"
	"sort"
	"testing"
)

// Every concrete policy field must pass through the same normalization and
// public identity projection without mutating the private report.
func TestAllPolicyLanesNormalizeAndRemapWithoutPrivateMutation(t *testing.T) {
	r := New("author.validate", "revision")
	missing := r.add(Finding{Code: "plugin_manifest_missing", Layer: "normative", Rule: "core", Severity: "error"})
	other := r.add(Finding{Code: "mcp_server_invalid", Layer: "normative", Rule: "mcp/server", Severity: "error"})
	for _, a := range []*Assessment{
		&r.Loadability, &r.Conformance, &r.HostSafety, &r.Readiness,
		&r.Release, &r.Runtime, &r.Compatibility, &r.Toolchain,
	} {
		a.Status = Fail
		a.FindingIDs = []string{other, missing, other}
	}
	r.Checks = []Check{{ID: "portable_configuration", Assessment: Assessment{
		Status: Fail, FindingIDs: []string{missing, other, missing},
	}}}
	r.finish()
	expectedPrivate := []string{missing, other}
	sort.Strings(expectedPrivate)
	for _, a := range []Assessment{
		r.Loadability, r.Conformance, r.HostSafety, r.Readiness,
		r.Release, r.Runtime, r.Compatibility, r.Toolchain, r.Checks[0].Assessment,
	} {
		if !reflect.DeepEqual(a.FindingIDs, expectedPrivate) {
			t.Fatalf("private assessment not normalized: %+v", a)
		}
	}
	before, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	public := r.PublicResult("author.validate", "read", true, nil)
	var publicMissing string
	for _, finding := range public.Findings {
		if finding.Code == "missing_standard_manifest" {
			publicMissing = finding.ID
		}
	}
	if publicMissing == "" || publicMissing == missing {
		t.Fatal("public missing-manifest identity not remapped")
	}
	expectedPublic := []string{other, publicMissing}
	sort.Strings(expectedPublic)
	for _, a := range []Assessment{
		public.Loadability, public.Conformance, public.HostSafety, public.Readiness,
		public.Release, public.Runtime, public.Compatibility, public.Toolchain,
		public.Checks[0].Assessment,
	} {
		if a.Status != Fail || !reflect.DeepEqual(a.FindingIDs, expectedPublic) {
			t.Fatalf("public assessment identity drift: %+v", a)
		}
	}
	after, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("public projection mutated private report")
	}
}
