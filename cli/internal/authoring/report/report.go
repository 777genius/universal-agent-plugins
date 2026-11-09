// Package report owns the allowlisted public contract. No package envelope, raw
// document, diagnostic message, environment value or command token is serialized.
package report

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"

	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/readiness"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/conformance"
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
type RuntimeDetail struct {
	Transport  string `json:"transport,omitempty"`
	Initialize State  `json:"initialize"`
	ListTools  State  `json:"list_tools"`
	ToolCall   State  `json:"tool_call"`
	ToolCount  int    `json:"tool_count"`
	Cleanup    State  `json:"cleanup"`
}
type Error struct {
	Code   string `json:"code"`
	Action string `json:"action"`
}
type JSONDocument struct {
	Path         string `json:"path"`
	BeforeSHA256 string `json:"before_sha256"`
	AfterSHA256  string `json:"after_sha256"`
	Changed      bool   `json:"changed"`
}
type ImportIssue struct {
	Code   string `json:"code"`
	ItemID string `json:"item_id"`
}
type NativeImport struct {
	Client              string        `json:"client"`
	SourceSHA256        string        `json:"source_sha256"`
	SafeServers         int           `json:"safe_servers"`
	SkippedServers      []ImportIssue `json:"skipped_servers"`
	UnsupportedTopLevel []ImportIssue `json:"unsupported_top_level"`
}
type Report struct {
	// Display is a sanitized projection retained only for explicit public rendering.
	Display       *Inspection                   `json:"-"`
	Legacy        string                        `json:"-"`
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
	RuntimeDetail *RuntimeDetail                `json:"runtime_details,omitempty"`
	Bootstrap     *BootstrapDetail              `json:"bootstrap,omitempty"`
	Findings      []Finding                     `json:"findings"`
	Components    []Component                   `json:"components"`
	Checks        []Check                       `json:"checks"`
	Committed     bool                          `json:"committed"`
	Paths         []string                      `json:"affected_paths"`
	JSONDocument  *JSONDocument                 `json:"json_document,omitempty"`
	NativeImport  *NativeImport                 `json:"native_import,omitempty"`
	Error         *Error                        `json:"error,omitempty"`
}

type BootstrapDetail struct {
	Runtime string   `json:"runtime"`
	Manager string   `json:"manager"`
	Argv    []string `json:"argv"`
	Planned bool     `json:"planned"`
}

func (r *Report) SetRuntime(transport string, initialized, listed, called bool, toolCount int, cleanup bool, code string) {
	stateOf := func(v bool) State {
		if v {
			return Pass
		}
		return NotEvaluated
	}
	r.RuntimeDetail = &RuntimeDetail{Transport: transport, Initialize: stateOf(initialized), ListTools: stateOf(listed), ToolCall: stateOf(called), ToolCount: toolCount, Cleanup: stateOf(cleanup)}
	r.Runtime = assessment(Pass)
	if code != "" {
		id := r.add(Finding{Code: code, Layer: "runtime", Rule: "authoring/mcp-runtime-v1", Severity: "error"})
		r.Runtime.Status = Fail
		r.Runtime.FindingIDs = append(r.Runtime.FindingIDs, id)
	}
	if !cleanup {
		r.Runtime.Status = Fail
	}
	for i := range r.Checks {
		if r.Checks[i].ID == "runtime" {
			r.Checks[i].Assessment = r.Runtime
		}
	}
	r.finish()
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
	return r.Error == nil && r.Readiness.Status == Pass && r.Release.Status != Fail && r.Compatibility.Status != Fail && r.Runtime.Status != Fail && (r.Command != "doctor" || r.Toolchain.Status == Pass)
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
			if component.Support == "unsupported" && !r.addCommandCompatibility(c, component) {
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
