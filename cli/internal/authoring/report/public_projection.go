package report

import "strings"

// normalizePublicFindings maps the private missing-manifest code to its public
// name without changing the original report or its policy finding identities.
func (r *Report) normalizePublicFindings() map[string]string {
	r.Findings = append([]Finding{}, r.Findings...)
	replacements := map[string]string{}
	for i, f := range r.Findings {
		if f.Code != "plugin_manifest_missing" {
			continue
		}
		old := f.ID
		f.Code = "missing_standard_manifest"
		f.ID = ""
		tmp := New("", "")
		f.ID = tmp.add(f)
		r.Findings[i] = f
		replacements[old] = f.ID
	}
	return replacements
}

// remapPublicAssessment clones IDs before remapping references to public findings.
func remapPublicAssessment(a Assessment, replacements map[string]string) Assessment {
	a.FindingIDs = append([]string{}, a.FindingIDs...)
	for i, id := range a.FindingIDs {
		if replacement, ok := replacements[id]; ok {
			a.FindingIDs[i] = replacement
		}
	}
	return a
}

// remapPublicAssessments applies the same ID projection to each policy and check.
func (r *Report) remapPublicAssessments(replacements map[string]string) {
	r.Loadability = remapPublicAssessment(r.Loadability, replacements)
	r.Conformance = remapPublicAssessment(r.Conformance, replacements)
	r.HostSafety = remapPublicAssessment(r.HostSafety, replacements)
	r.Readiness = remapPublicAssessment(r.Readiness, replacements)
	r.Release = remapPublicAssessment(r.Release, replacements)
	r.Runtime = remapPublicAssessment(r.Runtime, replacements)
	r.Compatibility = remapPublicAssessment(r.Compatibility, replacements)
	r.Toolchain = remapPublicAssessment(r.Toolchain, replacements)
	r.Checks = append([]Check{}, r.Checks...)
	for i := range r.Checks {
		r.Checks[i].Assessment = remapPublicAssessment(r.Checks[i].Assessment, replacements)
	}
}

// projectPublicPaths withholds entire unsafe paths while preserving opaque IDs.
func (p *Public) projectPublicPaths(paths []string) {
	p.Paths = []string{}
	for _, path := range paths {
		safe := true
		for _, part := range strings.Split(path, "/") {
			if DisplayIdentity(part) == "" {
				safe = false
			}
		}
		if safe {
			p.Paths = append(p.Paths, path)
		} else {
			p.WithheldPathIDs = append(p.WithheldPathIDs, opaque(path))
		}
	}
}

// projectPublicActions preserves the existing public advice priority and order.
func (p *Public) projectPublicActions(replacements map[string]string) {
	if p.Error != nil {
		p.NextActions = append(p.NextActions, Action{Code: p.Error.Code, Message: p.Error.Action})
	}
	if len(replacements) > 0 {
		p.NextActions = append(p.NextActions, Action{Code: "missing_standard_manifest", Message: "Select the exact package root containing plugin.json; no ancestor search or format fallback is performed."})
	}
	if p.Error == nil && len(replacements) == 0 && p.Readiness.Status == Fail {
		p.NextActions = append(p.NextActions, Action{"review_findings", "author.validate", "Review the typed findings by policy layer, correct the selected package, then validate it again."})
	}
	if p.Legacy != "" && p.Legacy != "absent" {
		p.NextActions = append(p.NextActions, Action{Code: "legacy_workflow_unavailable", Message: "This v1 workflow is unavailable in v2. Use plugin-kit-ai 1.2.4 for the legacy workflow. Project migration is unavailable in v2."})
	}
	if p.Error == nil && p.Readiness.Status == Pass {
		if p.Requested.Operation == "author.init" || p.Requested.Operation == "author.skills.init" {
			p.NextActions = append(p.NextActions, Action{"inspect_package", "author.inspect", "Inspect the created package using its explicit package path."})
		}
		if p.Runtime.Status == Pass {
			p.NextActions = append(p.NextActions, Action{"runtime_verified", p.Requested.Operation, "The selected MCP runtime stages passed under the transport-specific bounded runtime policy."})
		} else {
			p.NextActions = append(p.NextActions, Action{"static_test", "author.test", "Run the offline static test with the package path; runtime behavior remains unevaluated."})
		}
	}
}
