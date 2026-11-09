package report

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/project"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packageview"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/conformance"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestFindingIdentityIsBoundedByComponentType(t *testing.T) {
	const shared = "same-private-name"
	const missing = "unlisted-private-name"
	pkg := &domain.PackageEnvelope{}
	pkg.MCP.Servers = map[string]domain.MCPServer{
		shared: {Type: "remote"},
	}
	pkg.Inventory.InvalidSkills = []string{shared}
	p := project.Result{
		Input: packageview.Input{Coverage: packageview.Coverage{InventoryComplete: true}},
		Facts: conformance.Facts{
			Package: pkg,
			Conformance: conformance.Fail,
			Findings: []conformance.Finding{
				{Code: "mcp", Layer: conformance.Normative, Boundary: domain.BoundaryMCPServer, Item: opaque(shared), Severity: domain.SeverityError},
				{Code: "skill", Layer: conformance.HostSafety, Boundary: domain.BoundarySkill, Item: opaque(shared), Severity: domain.SeverityError},
				{Code: "unlisted", Layer: conformance.Normative, Boundary: domain.BoundaryMCPServer, Item: opaque(missing), Severity: domain.SeverityError},
				{Code: "unbounded", Layer: conformance.Normative, Item: opaque(shared), Severity: domain.SeverityError},
			},
		},
	}
	r := New("validate", "revision")
	r.recordScope(p)
	r.recordFindings(p)
	r.finish()
	expected := map[string]string{
		"mcp": opaque("mcp:" + shared),
		"skill": opaque("skill:" + shared),
		"unlisted": opaque(missing),
		"unbounded": opaque(shared),
	}
	if len(r.Findings) != len(expected) || len(r.Conformance.FindingIDs) != 3 ||
		len(r.HostSafety.FindingIDs) != 1 || r.HostSafety.Status != Fail {
		t.Fatalf("finding boundaries: findings=%d normative=%d host=%d host_status=%s",
			len(r.Findings), len(r.Conformance.FindingIDs), len(r.HostSafety.FindingIDs), r.HostSafety.Status)
	}
	for _, f := range r.Findings {
		if f.ItemID != expected[f.Code] {
			t.Errorf("%s: got item %s, want %s", f.Code, f.ItemID, expected[f.Code])
		}
	}
	wire, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(wire), shared) || strings.Contains(string(wire), missing) {
		t.Fatal("raw captured component names leaked in public evidence")
	}
}

func TestFindingIdentityWithoutInventoryDoesNotGrantHostPass(t *testing.T) {
	p := project.Result{
		Facts: conformance.Facts{
			Findings: []conformance.Finding{
				{Code: "unmapped", Layer: conformance.Normative, Boundary: domain.BoundarySkill, Item: opaque("unknown"), Severity: domain.SeverityError},
			},
		},
	}
	r := New("validate", "revision")
	r.recordScope(p)
	r.recordFindings(p)
	r.finish()
	if r.HostSafety.Status != NotEvaluated || len(r.Findings) != 1 ||
		r.Findings[0].ItemID != opaque("unknown") || len(r.Conformance.FindingIDs) != 1 {
		t.Fatal("missing inventory changed an unknown finding or granted host pass")
	}
}
