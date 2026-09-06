// Package report owns the allowlisted public contract. No package envelope, raw
// document, diagnostic message, environment value or command token is serialized.
package report

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"

	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/project"
	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/readiness"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packageview"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/conformance"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/planner"
)

type State string

const (
	Pass         State = "pass"
	Fail         State = "fail"
	NotEvaluated State = "not_evaluated"
)

type Assessment struct {
	Status     State    `json:"status"`
	FindingIDs []string `json:"finding_ids"`
}
type Finding struct {
	ID       string `json:"id"`
	Code     string `json:"code"`
	Layer    string `json:"layer"`
	Rule     string `json:"rule"`
	Location string `json:"location,omitempty"`
	ItemID   string `json:"item_id,omitempty"`
	Severity string `json:"severity"`
}
type Profile struct {
	ID       string `json:"id"`
	Revision string `json:"revision"`
	Digest   string `json:"digest"`
}
type Identity struct {
	ScopeAlgorithm string   `json:"scope_algorithm"`
	ScopeDigest    string   `json:"scope_digest,omitempty"`
	TreeAlgorithm  string   `json:"tree_algorithm,omitempty"`
	TreeDigest     string   `json:"tree_digest,omitempty"`
	ManifestDigest string   `json:"manifest_digest,omitempty"`
	ReadProfile    string   `json:"read_profile"`
	TreeExclusions []string `json:"tree_exclusions"`
}
type Coverage struct {
	ComponentsRequested bool  `json:"components_requested"`
	SkillsEnumerated    bool  `json:"skills_enumerated"`
	InventoryComplete   bool  `json:"inventory_complete"`
	TreeComplete        bool  `json:"tree_complete"`
	Plugin              State `json:"plugin"`
	MCP                 State `json:"mcp"`
	Skills              State `json:"skills"`
	Filesystem          State `json:"filesystem"`
	FactsComplete       bool  `json:"facts_complete"`
}
type Component struct {
	ID           string   `json:"id"`
	Type         string   `json:"type"`
	Status       State    `json:"status"`
	Requirements []string `json:"requirements"`
}
type Check struct {
	ID string `json:"id"`
	Assessment
}
type Error struct {
	Code   string `json:"code"`
	Action string `json:"action"`
}
type Report struct {
	Compatibility Assessment                    `json:"compatibility"`
	Clients       []planner.ClientCompatibility `json:"clients,omitempty"`
	Capabilities  *readiness.Capabilities       `json:"capabilities,omitempty"`
	Toolchain     Assessment                    `json:"toolchain"`
	DoctorChecks  []readiness.Check             `json:"doctor_checks,omitempty"`
	Schema        string                        `json:"schema"`
	Engine        string                        `json:"engine"`
	Revision      string                        `json:"revision"`
	Command       string                        `json:"command"`
	Mode          string                        `json:"mode"`
	Root          string                        `json:"root,omitempty"`
	Identity      Identity                      `json:"identity"`
	Coverage      Coverage                      `json:"coverage"`
	Profiles      []Profile                     `json:"profiles"`
	SchemaIDs     []string                      `json:"schema_ids"`
	Loadability   Assessment                    `json:"loadability"`
	Conformance   Assessment                    `json:"normative_conformance"`
	HostSafety    Assessment                    `json:"host_safety"`
	Readiness     Assessment                    `json:"authoring_readiness"`
	Release       Assessment                    `json:"release_policy"`
	Runtime       Assessment                    `json:"runtime_evidence"`
	Findings      []Finding                     `json:"findings"`
	Components    []Component                   `json:"components"`
	Checks        []Check                       `json:"checks"`
	Committed     bool                          `json:"committed"`
	Paths         []string                      `json:"affected_paths"`
	Error         *Error                        `json:"error,omitempty"`
}

func assessment(s State) Assessment { return Assessment{Status: s, FindingIDs: []string{}} }
func New(command, revision string) Report {
	return Report{Schema: "agentplugins-authoring-report/v1", Engine: "standard-first-slice/1", Revision: revision,
		Compatibility: assessment(NotEvaluated), Toolchain: assessment(NotEvaluated), Command: command, Mode: "read", Coverage: Coverage{Plugin: NotEvaluated, MCP: NotEvaluated, Skills: NotEvaluated, Filesystem: NotEvaluated}, Profiles: []Profile{}, SchemaIDs: []string{}, Findings: []Finding{}, Components: []Component{}, Checks: []Check{}, Paths: []string{},
		Loadability: assessment(NotEvaluated), Conformance: assessment(NotEvaluated), HostSafety: assessment(NotEvaluated), Readiness: assessment(NotEvaluated), Release: assessment(NotEvaluated), Runtime: assessment(NotEvaluated)}
}
func opaque(s string) string {
	h := sha256.Sum256([]byte(s))
	return "sha256:" + hex.EncodeToString(h[:])
}
func (r *Report) add(f Finding) string {
	f.ID = ""
	body, _ := json.Marshal(f)
	f.ID = opaque(string(body))
	for _, old := range r.Findings {
		if old.ID == f.ID {
			return f.ID
		}
	}
	r.Findings = append(r.Findings, f)
	return f.ID
}
func (r *Report) AddError(code, action string) {
	r.Error = &Error{code, action}
	id := r.add(Finding{Code: code, Layer: "host_safety", Rule: "authoring/operation", Severity: "error"})
	r.HostSafety.Status = Fail
	r.HostSafety.FindingIDs = append(r.HostSafety.FindingIDs, id)
	r.Readiness.Status = Fail
	r.Readiness.FindingIDs = append(r.Readiness.FindingIDs, id)
	if r.Release.Status != NotEvaluated {
		r.Release.Status = Fail
		r.Release.FindingIDs = append(r.Release.FindingIDs, id)
	}
	for i := range r.Checks {
		if r.Checks[i].ID == "package_hygiene" {
			r.Checks[i].Assessment = r.HostSafety
		}
	}
	r.finish()
}

func Build(command, revision string, p project.Result, release bool) Report {
	r := New(command, revision)
	v, f := p.Input, p.Facts
	r.Identity = Identity{ScopeAlgorithm: v.Identity.ScopeID, ScopeDigest: v.Identity.Digest, TreeAlgorithm: v.Identity.TreeAlgorithm, TreeDigest: v.Identity.TreeDigest, ReadProfile: packageview.ReadProfile, TreeExclusions: []string{"root .git", "root non-directory .plugin-kit-ai.lock"}}
	c := f.Coverage
	r.Coverage = Coverage{v.Coverage.ComponentsRequested, v.Coverage.SkillsEnumerated, v.Coverage.InventoryComplete, v.Coverage.TreeComplete, state(c.Plugin), state(c.MCP), state(c.Skills), state(c.Filesystem), c.Complete}
	for _, p := range f.Profiles {
		r.Profiles = append(r.Profiles, Profile{p.ID, p.Revision, p.Digest})
	}
	r.SchemaIDs = append(r.SchemaIDs, f.SchemaIDs...)
	r.Conformance = assessment(state(f.Conformance))
	r.HostSafety = assessment(Pass)
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
	for _, f := range v.Findings {
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
	if !v.Coverage.InventoryComplete && r.HostSafety.Status == Pass {
		r.HostSafety.Status = NotEvaluated
	}
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
	r.Readiness.Status = combine(r.Loadability.Status, r.Conformance.Status, r.HostSafety.Status)
	if r.Readiness.Status == Pass && (!c.Complete || !v.Coverage.TreeComplete) {
		r.Readiness.Status = NotEvaluated
	}
	r.Readiness.FindingIDs = append(r.Readiness.FindingIDs, r.Loadability.FindingIDs...)
	r.Readiness.FindingIDs = append(r.Readiness.FindingIDs, r.Conformance.FindingIDs...)
	r.Readiness.FindingIDs = append(r.Readiness.FindingIDs, r.HostSafety.FindingIDs...)
	if release {
		r.Release = r.Readiness
		r.Release.FindingIDs = append([]string{}, r.Readiness.FindingIDs...)
		if v.Legacy != "absent" {
			id := r.add(Finding{Code: "legacy_release_rejected", Layer: "release_policy", Rule: "authoring/release-hygiene-v1", Location: "plugin/plugin.yaml", Severity: "error"})
			r.Release.Status = Fail
			r.Release.FindingIDs = append(r.Release.FindingIDs, id)
		}
	}
	if command == "test" {
		r.Checks = []Check{{"portable_configuration", r.Conformance}, {"package_hygiene", r.HostSafety},
			{"static_skills", r.boundary(r.Coverage.Skills, "skills")}, {"static_mcp", r.boundary(r.Coverage.MCP, "mcp.json")},
			{"runtime", r.Runtime}}
	}
	r.finish()
	return r
}
func state(s conformance.Outcome) State {
	if s == conformance.Pass {
		return Pass
	}
	if s == conformance.Fail {
		return Fail
	}
	return NotEvaluated
}
func combine(states ...State) State {
	result := Pass
	for _, s := range states {
		if s == Fail {
			return Fail
		}
		if s != Pass {
			result = NotEvaluated
		}
	}
	return result
}
func (r Report) boundary(s State, location string) Assessment {
	a := assessment(s)
	for _, f := range r.Findings {
		if f.Location == location {
			a.FindingIDs = append(a.FindingIDs, f.ID)
		}
	}
	return a
}
func (r *Report) components(p project.Result) {
	pkg := p.Facts.Package
	for name, s := range pkg.MCP.Servers {
		req := []string{}
		if s.Type == "stdio" {
			req = append(req, "executable_unresolved")
			if s.StdioRequirement != nil {
				req = append(req, "executable_"+string(s.StdioRequirement.Kind))
				if s.StdioRequirement.UsesPluginRoot {
					req = append(req, "plugin_root")
				}
				if s.StdioRequirement.UsesPluginData {
					req = append(req, "plugin_data")
				}
			}
			if env, ok := s.Decoded["env"].(map[string]any); ok && len(env) > 0 {
				req = append(req, "environment_values_redacted")
			}
		} else {
			req = append(req, "remote_endpoint_uncontacted")
			if h, ok := s.Decoded["headers"].(map[string]any); ok && len(h) > 0 {
				req = append(req, "header_values_redacted")
			}
		}
		status := Pass
		for _, finding := range p.Facts.Findings {
			if finding.Code == "diagnostics_truncated" {
				status = combine(status, NotEvaluated)
			}
			if finding.Boundary != domain.BoundaryMCPServer || finding.Item != opaque(name) {
				continue
			}
			if finding.Layer == conformance.Normative {
				status = Fail
			} else {
				status = combine(status, NotEvaluated)
			}
		}
		r.Components = append(r.Components, Component{opaque("mcp:" + name), "mcp_" + s.Type, status, req})
	}
	for name := range pkg.MCP.InvalidServer {
		r.Components = append(r.Components, Component{opaque("mcp:" + name), "mcp_server", Fail, []string{}})
	}
	for name, s := range pkg.Skills {
		req := []string{}
		if s.Compatibility != "" {
			req = append(req, "declared_compatibility_unresolved")
		}
		if s.AllowedTools != "" {
			req = append(req, "declared_tools_unresolved")
		}
		r.Components = append(r.Components, Component{opaque("skill:" + name), "skill", Pass, req})
	}
	for _, name := range pkg.Inventory.InvalidSkills {
		r.Components = append(r.Components, Component{opaque("skill:" + name), "skill", Fail, []string{}})
	}
	for name := range pkg.Manifest.Extensions {
		r.Components = append(r.Components, Component{opaque("extension:" + name), "opaque_extension", NotEvaluated, []string{}})
	}
}
func (r *Report) finish() {
	sort.Slice(r.Findings, func(i, j int) bool { return r.Findings[i].ID < r.Findings[j].ID })
	sort.Slice(r.Components, func(i, j int) bool { return r.Components[i].ID < r.Components[j].ID })
	r.SchemaIDs = unique(r.SchemaIDs)
	for _, a := range []*Assessment{&r.Loadability, &r.Conformance, &r.HostSafety, &r.Readiness, &r.Release, &r.Runtime, &r.Compatibility, &r.Toolchain} {
		a.FindingIDs = unique(a.FindingIDs)
	}
	for i := range r.Checks {
		r.Checks[i].FindingIDs = unique(r.Checks[i].FindingIDs)
	}
}
func unique(v []string) []string {
	sort.Strings(v)
	out := []string{}
	for _, s := range v {
		if len(out) == 0 || out[len(out)-1] != s {
			out = append(out, s)
		}
	}
	return out
}
func (r Report) Successful() bool {
	if r.Command == "capabilities" {
		return r.Error == nil && r.Capabilities != nil
	}
	return r.Error == nil && r.Readiness.Status == Pass && r.Release.Status != Fail && r.Compatibility.Status != Fail && (r.Command != "doctor" || r.Toolchain.Status == Pass)
}

// AddCompatibility attaches only the planner's explicitly sanitized static DTO.
func (r *Report) AddCompatibility(clients []planner.ClientCompatibility) {
	r.Clients = clients
	if clients == nil {
		return
	}
	r.Compatibility = assessment(Pass)
	for _, c := range clients {
		for _, component := range c.Components {
			if component.Support == "unsupported" {
				id := r.add(Finding{Code: "target_component_unsupported", Layer: "compatibility", Rule: "planner/static-support", ItemID: opaque(string(c.ClientID) + ":" + string(component.Kind)), Severity: "error"})
				r.Compatibility.Status = Fail
				r.Compatibility.FindingIDs = append(r.Compatibility.FindingIDs, id)
			}
		}
	}
	r.finish()
}

// Doctor readiness is distinct from valid package authoring and from execution.
// Unknown runtime proof must never become a passed toolchain assessment.
func (r *Report) AddDoctor(checks []readiness.Check) {
	r.DoctorChecks = checks
	r.Toolchain = assessment(NotEvaluated)
	if r.Loadability.Status == Pass && r.Readiness.Status == Pass {
		r.Toolchain.Status = Pass
	}
	for _, check := range checks {
		if check.Status == "pass" {
			continue
		}
		severity := "warning"
		if check.Status == "fail" {
			severity = "error"
			r.Toolchain.Status = Fail
		} else if r.Toolchain.Status == Pass {
			r.Toolchain.Status = NotEvaluated
		}
		id := r.add(Finding{Code: check.ID, Layer: "toolchain", Rule: "authoring/static-evidence", ItemID: check.ItemID, Severity: severity})
		r.Toolchain.FindingIDs = append(r.Toolchain.FindingIDs, id)
	}
	r.finish()
}
