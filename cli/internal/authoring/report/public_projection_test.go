package report

import (
	"encoding/json"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// TestPublicProjectionRemapsAllPolicyAndCheckReferences covers every policy lane.
// Public projection may not change the private report's finding identities.
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
	expected := []string{other, publicMissing}
	sort.Strings(expected)
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

// TestPublicProjectionWithholdsPathsAndPreservesAdviceOrder pins advice order
// and represents unsafe pathnames only by their deterministic digests.
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
	if strings.Contains(string(encoded), "my-token") {
		t.Fatal("withheld private path leaked in public JSON")
	}
	var wire struct {
		WithheldPathIDs []string `json:"withheld_path_ids"`
		NextActions     []Action `json:"next_actions"`
	}
	if err := json.Unmarshal(encoded, &wire); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(wire.WithheldPathIDs, p.WithheldPathIDs) {
		t.Fatalf("encoded withheld path IDs changed: %v", wire.WithheldPathIDs)
	}
	wireCodes := make([]string, 0, len(wire.NextActions))
	for _, action := range wire.NextActions {
		wireCodes = append(wireCodes, action.Code)
	}
	if !reflect.DeepEqual(wireCodes, codes) {
		t.Fatalf("encoded advice order changed: %v", wireCodes)
	}
}

// TestPublicProjectionAdviceSuccessAndReviewPaths pins advice ordering when
// there is no manifest replacement or operation error to take precedence.
func TestPublicProjectionAdviceSuccessAndReviewPaths(t *testing.T) {
	cases := []struct {
		name      string
		operation string
		readiness State
		runtime   State
		legacy    string
		want      []Action
	}{
		{"readiness failure", "author.validate", Fail, NotEvaluated, "", []Action{{Code: "review_findings", Operation: "author.validate"}}},
		{"initialization runtime pass", "author.init", Pass, Pass, "", []Action{{Code: "inspect_package", Operation: "author.inspect"}, {Code: "runtime_verified", Operation: "author.init"}}},
		{"skills initialization static", "author.skills.init", Pass, NotEvaluated, "", []Action{{Code: "inspect_package", Operation: "author.inspect"}, {Code: "static_test", Operation: "author.test"}}},
		{"validation static", "author.validate", Pass, NotEvaluated, "", []Action{{Code: "static_test", Operation: "author.test"}}},
		{"legacy review priority", "author.validate", Fail, NotEvaluated, "present", []Action{{Code: "review_findings", Operation: "author.validate"}, {Code: "legacy_workflow_unavailable"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := New("validate", "revision")
			r.Readiness = Assessment{Status: tc.readiness}
			r.Runtime = Assessment{Status: tc.runtime}
			r.Legacy = tc.legacy
			p := r.PublicResult(tc.operation, "read", false, nil)
			got := make([]Action, 0, len(p.NextActions))
			for _, action := range p.NextActions {
				got = append(got, Action{Code: action.Code, Operation: action.Operation})
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("public advice order/operations changed: got %+v want %+v", got, tc.want)
			}
		})
	}
}
