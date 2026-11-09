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

// recordFindings maps retained, typed diagnostics into stable public IDs.
func (r *Report) recordFindings(p project.Result) {
	v, f := p.Input, p.Facts
	// Translate retained canonical identities within their component boundary.
	serverIDs := map[string]string{}
	skillIDs := map[string]string{}
	if f.Package != nil {
		for name := range f.Package.MCP.Servers {
			serverIDs[opaque(name)] = opaque("mcp:" + name)
		}
		for name := range f.Package.MCP.InvalidServer {
			serverIDs[opaque(name)] = opaque("mcp:" + name)
		}
		for name := range f.Package.Skills {
			skillIDs[opaque(name)] = opaque("skill:" + name)
		}
		for _, name := range f.Package.Inventory.InvalidSkills {
			skillIDs[opaque(name)] = opaque("skill:" + name)
		}
	}
	for _, f := range f.Findings {
		switch f.Boundary {
		case domain.BoundarySkill:
			if id, ok := skillIDs[f.Item]; ok {
				f.Item = id
			}
		case domain.BoundaryMCPServer:
			if id, ok := serverIDs[f.Item]; ok {
				f.Item = id
			}
		}
		id := r.add(Finding{Code: f.Code, Layer: string(f.Layer), Rule: f.RuleRef, Location: f.Path, ItemID: f.Item, Severity: string(f.Severity)})
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

