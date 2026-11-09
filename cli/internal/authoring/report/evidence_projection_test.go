package report

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/project"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packageview"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/conformance"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestStaticEvidencePreservesComponentIdentityAndHostRefusals(t *testing.T) {
	const server = "token-private-mcp"
	const skill = "token-private-skill"
	const sourcePath = "skills/token-private-skill/SKILL.md"
	pkg := &domain.PackageEnvelope{}
	pkg.Manifest.Name = "demo"
	pkg.MCP.Servers = map[string]domain.MCPServer{
		server: {Type: "remote"},
	}
	pkg.Inventory.InvalidSkills = []string{skill}
	p := project.Result{
		Input: packageview.Input{
			Findings: []packageview.Finding{
				{Code: "legacy_manifest_ignored", Location: sourcePath},
				{Code: "skills_unreadable", Location: sourcePath},
			},
		},
		Facts: conformance.Facts{
			Package:     pkg,
			Conformance: conformance.Fail,
			Findings: []conformance.Finding{
				{Code: "mcp_invalid", Layer: conformance.Normative, Boundary: domain.BoundaryMCPServer, Item: opaque(server), Path: "mcp.json", Severity: domain.SeverityError},
				{Code: "skill_invalid", Layer: conformance.HostSafety, Boundary: domain.BoundarySkill, Item: opaque(skill), Path: "skills", Severity: domain.SeverityError},
			},
		},
	}
	r := New("validate", "revision")
	r.recordScope(p)
	r.recordFindings(p)
	r.recordLoadability(p)
	r.finish()

	if r.Conformance.Status != Fail || r.HostSafety.Status != Fail || r.Loadability.Status != Pass {
		t.Fatalf("evidence boundaries: normative=%s host=%s load=%s", r.Conformance.Status, r.HostSafety.Status, r.Loadability.Status)
	}
	if len(r.Findings) != 4 || len(r.Conformance.FindingIDs) != 1 || len(r.HostSafety.FindingIDs) != 3 {
		t.Fatalf("missing or duplicated evidence: findings=%d normative=%d host=%d", len(r.Findings), len(r.Conformance.FindingIDs), len(r.HostSafety.FindingIDs))
	}
	expected := map[string]string{
		"mcp_invalid":             opaque("mcp:" + server),
		"skill_invalid":           opaque("skill:" + skill),
		"legacy_manifest_ignored": opaque(sourcePath),
		"skills_unreadable":       opaque(sourcePath),
	}
	for _, finding := range r.Findings {
		if finding.ItemID != expected[finding.Code] {
			t.Fatalf("canonical identity changed for %s", finding.Code)
		}
		if finding.Code == "legacy_manifest_ignored" && finding.Severity != "warning" {
			t.Fatal("legacy advisory promoted to an error")
		}
		if finding.Code == "skills_unreadable" && finding.Severity != "error" {
			t.Fatal("unreadable host evidence silently accepted")
		}
	}
	if len(r.Components) != 2 {
		t.Fatalf("missing component evidence: %d", len(r.Components))
	}
	wire, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(wire), sourcePath) || strings.Contains(string(wire), server) || strings.Contains(string(wire), skill) {
		t.Fatal("private captured names escaped in public report")
	}
}

func TestStaticEvidenceScopeKeepsCoverageAndStableSchemaIDs(t *testing.T) {
	p := project.Result{
		Input: packageview.Input{
			Identity: packageview.Identity{ScopeID: "captured-v1", Digest: "scope-digest", TreeAlgorithm: "tree-v1", TreeDigest: "tree-digest"},
			Coverage: packageview.Coverage{ComponentsRequested: true, SkillsEnumerated: true, InventoryComplete: true, TreeComplete: true},
		},
		Facts: conformance.Facts{
			Coverage:    conformance.Coverage{Plugin: conformance.Pass, MCP: conformance.NotEvaluated, Skills: conformance.Pass, Filesystem: conformance.Pass, Complete: false},
			Conformance: conformance.NotEvaluated,
			SchemaIDs:   []string{"schema/b", "schema/a", "schema/a"},
		},
	}
	r := New("validate", "revision")
	r.recordScope(p)
	r.finish()
	if r.Identity.ScopeAlgorithm != "captured-v1" || r.Identity.ScopeDigest != "scope-digest" ||
		r.Identity.TreeAlgorithm != "tree-v1" || r.Identity.TreeDigest != "tree-digest" ||
		r.Identity.ReadProfile != packageview.ReadProfile {
		t.Fatal("captured identity changed")
	}
	if !r.Coverage.ComponentsRequested || !r.Coverage.SkillsEnumerated ||
		!r.Coverage.InventoryComplete || !r.Coverage.TreeComplete ||
		r.Coverage.Plugin != Pass || r.Coverage.MCP != NotEvaluated ||
		r.Coverage.Skills != Pass || r.Coverage.Filesystem != Pass || r.Coverage.FactsComplete {
		t.Fatal("static coverage or unknown evidence changed")
	}
	if !slices.Equal(r.SchemaIDs, []string{"schema/a", "schema/b"}) ||
		r.Conformance.Status != NotEvaluated || r.HostSafety.Status != Pass {
		t.Fatal("static conformance or deterministic schema projection changed")
	}
}
