package report

import (
	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/project"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packageview"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/conformance"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// Static package and acquisition evidence remains distinct from policy decisions.
// These projections use captured values only and never probe the host.
// recordScope captures the immutable identity and coverage projections first.
func (r *Report) recordScope(p project.Result) {
	v, f := p.Input, p.Facts
	r.Display, r.Legacy = inspection(p), string(v.Legacy)
	r.Identity = Identity{ScopeAlgorithm: v.Identity.ScopeID, ScopeDigest: v.Identity.Digest, TreeAlgorithm: v.Identity.TreeAlgorithm, TreeDigest: v.Identity.TreeDigest, ReadProfile: packageview.ReadProfile, TreeExclusions: []string{"root .git", "root non-directory .uap-integrations.lock"}}
	c := f.Coverage
	r.Coverage = Coverage{v.Coverage.ComponentsRequested, v.Coverage.SkillsEnumerated, v.Coverage.InventoryComplete, v.Coverage.TreeComplete, state(c.Plugin), state(c.MCP), state(c.Skills), state(c.Filesystem), c.Complete}
	for _, p := range f.Profiles {
		r.Profiles = append(r.Profiles, Profile{p.ID, p.Revision, p.Digest})
	}
	r.SchemaIDs = append(r.SchemaIDs, f.SchemaIDs...)
	r.Conformance = assessment(state(f.Conformance))
	r.HostSafety = assessment(Pass)
}

// knownFindingIDs retains canonical component identity only within its own
// boundary. The key includes the kind, so equal private MCP and Skill names
// cannot overwrite each other's public identity.
func knownFindingIDs(pkg *domain.PackageEnvelope) map[string]string {
	ids := map[string]string{}
	if pkg == nil {
		return ids
	}
	for name := range pkg.MCP.Servers {
		registerFindingID(ids, "mcp", name)
	}
	for name := range pkg.MCP.InvalidServer {
		registerFindingID(ids, "mcp", name)
	}
	for name := range pkg.Skills {
		registerFindingID(ids, "skill", name)
	}
	for _, name := range pkg.Inventory.InvalidSkills {
		registerFindingID(ids, "skill", name)
	}
	return ids
}

func registerFindingID(ids map[string]string, kind, name string) {
	ids[kind+":"+opaque(name)] = opaque(kind + ":" + name)
}

func canonicalFindingItem(f conformance.Finding, ids map[string]string) string {
	var kind string
	switch f.Boundary {
	case domain.BoundaryMCPServer:
		kind = "mcp"
	case domain.BoundarySkill:
		kind = "skill"
	default:
		return f.Item
	}
	if id, ok := ids[kind+":"+f.Item]; ok {
		return id
	}
	return f.Item
}

// recordFindings records typed diagnostics and their policy classification.
func (r *Report) recordFindings(p project.Result) {
	v, facts := p.Input, p.Facts
	ids := knownFindingIDs(facts.Package)
	for _, f := range facts.Findings {
		item := canonicalFindingItem(f, ids)
		id := r.add(Finding{Code: f.Code, Layer: string(f.Layer), Rule: f.RuleRef, Location: f.Path, ItemID: item, Severity: string(f.Severity)})
		if f.Layer == conformance.Normative {
			r.Conformance.FindingIDs = append(r.Conformance.FindingIDs, id)
		} else {
			r.HostSafety.Status = Fail
			r.HostSafety.FindingIDs = append(r.HostSafety.FindingIDs, id)
		}
	}
	r.recordAcquisitionFindings(p)
	if !v.Coverage.InventoryComplete && r.HostSafety.Status == Pass {
		r.HostSafety.Status = NotEvaluated
	}
}

// recordAcquisitionFindings handles source acquisition diagnostics separately
// from the package's canonical, typed conformance findings.
func (r *Report) recordAcquisitionFindings(p project.Result) {
	for _, f := range p.Input.Findings {
		severity := "error"
		if f.Code == "legacy_manifest_ignored" || f.Code == "complete_tree_identity_unavailable" {
			severity = "warning"
		}
		// Even escaped relative filesystem names may carry opaque values. Expose
		// their stable identity, never arbitrary names/link targets from inventory.
		id := r.add(Finding{Code: f.Code, Layer: "host_safety", Rule: "packageview/acquisition", ItemID: opaque(f.Location), Severity: severity})
		r.HostSafety.FindingIDs = append(r.HostSafety.FindingIDs, id)
		if severity == "error" {
			r.HostSafety.Status = Fail
		}
	}
}

// recordLoadability records static package presence and physical-name safety without opening files.
func (r *Report) recordLoadability(p project.Result) {
	f := p.Facts
	if f.Package != nil {
		r.Loadability.Status = Pass
		r.Identity.ManifestDigest = f.Package.ManifestDigest
		if pathpolicy.ValidateLeafID(domain.ComputePhysicalArtifactID(f.Package.Manifest.Name, "authoring-preview")) != nil {
			id := r.add(Finding{Code: "physical_name_unsafe", Layer: "host_safety", Rule: "host/portable-leaf", Location: "plugin.json", Severity: "error"})
			r.HostSafety.Status = Fail
			r.HostSafety.FindingIDs = append(r.HostSafety.FindingIDs, id)
		}
		r.components(p)
	} else {
		r.Loadability.Status = Fail
		for _, f := range r.Findings {
			r.Loadability.FindingIDs = append(r.Loadability.FindingIDs, f.ID)
		}
	}
}
