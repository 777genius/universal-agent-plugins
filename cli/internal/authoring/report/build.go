package report

import (
	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/project"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packageview"
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
	if v.Legacy != packageview.Absent {
		id := r.add(Finding{Code: "legacy_release_rejected", Layer: "release_policy", Rule: "authoring/release-hygiene-v1", Location: "plugin/plugin.yaml", Severity: "error"})
		r.Release.Status = Fail
		r.Release.FindingIDs = append(r.Release.FindingIDs, id)
	}
}
