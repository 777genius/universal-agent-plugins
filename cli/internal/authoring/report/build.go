package report

import (
	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/project"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packageview"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/conformance"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// Build assembles the existing authoring policy evidence without re-reading sources.
func Build(command, revision string, p project.Result, release bool) Report {
	r := New(command, revision)
	r.recordScope(p)
	r.recordFindings(p)
	r.recordLoadability(p)
	dependencies := r.recordDependencies(p)
	r.recordReadiness(p, dependencies)
	if release {
		r.recordRelease(p)
	}
	if command == "test" {
		r.Checks = []Check{{"portable_configuration", r.Conformance}, {"package_hygiene", r.HostSafety},
			{"static_skills", r.boundary(r.Coverage.Skills, "skills")}, {"static_mcp", r.boundary(r.Coverage.MCP, "mcp.json")},
			{"runtime", r.Runtime}}
	}
	r.finish()
	return r
}

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

// recordDependencies projects retained dependency diagnostics into an independent assessment.
func (r *Report) recordDependencies(p project.Result) Assessment {
	dependencies := assessment(Pass)
	for _, diagnostic := range p.DependencyDiagnostics {
		id := r.add(Finding{Code: diagnostic.Code, Layer: "package_management", Rule: "uap/package-dependencies", Location: diagnostic.Location, Severity: "error"})
		dependencies.Status = Fail
		dependencies.FindingIDs = append(dependencies.FindingIDs, id)
	}
	return dependencies
}

// recordReadiness combines prior assessments in the original finding-reference order.
func (r *Report) recordReadiness(p project.Result, dependencies Assessment) {
	v, f := p.Input, p.Facts
	c := f.Coverage
	commands := r.commandsAssessment(f.Package)
	r.Readiness.Status = combine(r.Loadability.Status, r.Conformance.Status, r.HostSafety.Status, commands.Status, dependencies.Status)
	if r.Readiness.Status == Pass && (!c.Complete || !v.Coverage.TreeComplete) {
		r.Readiness.Status = NotEvaluated
	}
	r.Readiness.FindingIDs = append(r.Readiness.FindingIDs, dependencies.FindingIDs...)
	r.Readiness.FindingIDs = append(r.Readiness.FindingIDs, commands.FindingIDs...)
	r.Readiness.FindingIDs = append(r.Readiness.FindingIDs, r.Loadability.FindingIDs...)
	r.Readiness.FindingIDs = append(r.Readiness.FindingIDs, r.Conformance.FindingIDs...)
	r.Readiness.FindingIDs = append(r.Readiness.FindingIDs, r.HostSafety.FindingIDs...)
}

// recordRelease copies readiness evidence before applying release-only legacy restrictions.
func (r *Report) recordRelease(p project.Result) {
	v := p.Input
	r.Release = r.Readiness
	r.Release.FindingIDs = append([]string{}, r.Readiness.FindingIDs...)
	if v.Legacy != "absent" {
		id := r.add(Finding{Code: "legacy_release_rejected", Layer: "release_policy", Rule: "authoring/release-hygiene-v1", Location: "plugin/plugin.yaml", Severity: "error"})
		r.Release.Status = Fail
		r.Release.FindingIDs = append(r.Release.FindingIDs, id)
	}
}
