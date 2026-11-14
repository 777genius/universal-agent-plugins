package conformance

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

type InputState string

const (
	Present    InputState = "present"
	Absent     InputState = "absent"
	Unreadable InputState = "unreadable"
	WrongKind  InputState = "wrong_kind"
	Blocked    InputState = "blocked"
)

// Document owns an immutable copy. Path is the exact logical package-relative name,
// never a snapshot filename. A zero Document is unavailable, not absent.
type Document struct {
	Path  string     `json:"path"`
	State InputState `json:"state"`
	body  []byte
}

func NewDocument(path string, state InputState, body []byte) Document {
	d := Document{Path: path, State: state}
	if state == Present {
		if len(body) > DefaultLimits().MCPBytes {
			d.State = Blocked
		} else {
			d.body = append([]byte(nil), body...)
		}
	}
	return d
}

// Bytes is an explicit internal lossless-data capability; reports never call it.
func (d Document) Bytes() []byte { return append([]byte(nil), d.body...) }

type SkillInput struct {
	Directory string
	Document  Document
}
type Input struct {
	Plugin Document
	MCP    Document
	Skills []SkillInput
	// SkillsRoot describes directory observation, not SKILL.md named "skills".
	SkillsRoot InputState
	Coverage   Coverage
	// Paths are supplied by a safe rooted reader. No lookup is performed here.
	Paths []PathObservation
}
type PathObservation struct {
	Item, Field string
	State       Outcome
}
type Outcome string

const (
	Pass         Outcome = "pass"
	Fail         Outcome = "fail"
	NotEvaluated Outcome = "not_evaluated"
)

type Coverage struct {
	Plugin Outcome `json:"plugin"`
	MCP    Outcome `json:"mcp"`
	Skills Outcome `json:"skills"`
	// Filesystem is supplied evidence of package containment, not inferred from JSON.
	Filesystem Outcome `json:"filesystem"`
	Complete   bool    `json:"complete"`
}
type PolicyLayer string

const (
	Normative       PolicyLayer = "normative"
	HostSafety      PolicyLayer = "host_safety"
	InstallerPolicy PolicyLayer = "installer_policy"
)

type Finding struct {
	Ordinal  int                    `json:"ordinal,omitempty"`
	ID       string                 `json:"id"`
	Code     string                 `json:"code"`
	RuleRef  string                 `json:"rule_ref"`
	Path     string                 `json:"path"`
	Pointer  string                 `json:"pointer,omitempty"`
	Item     string                 `json:"item,omitempty"`
	Layer    PolicyLayer            `json:"layer"`
	Boundary domain.FailureBoundary `json:"boundary"`
	Severity domain.Severity        `json:"severity"`
}
type Facts struct {
	// Package is an internal service value, never a report DTO. It has no trust,
	// source root or tree-digest claim and must not bypass the installer facade.
	Package                     *domain.PackageEnvelope `json:"-"`
	Findings                    []Finding               `json:"findings"`
	Coverage                    Coverage                `json:"coverage"`
	Profiles                    []ProfileIdentity       `json:"profiles"`
	SchemaIDs                   []string                `json:"schema_ids"`
	Conformance                 Outcome                 `json:"conformance"`
	raw                         map[string]Document
	maxFindings                 int
	truncated, normativeFailure bool
}

func (f Facts) Document(path string) (Document, bool) { d, ok := f.raw[path]; return d, ok }

// Limits are host policy, never normative. Nonpositive values select defaults;
// larger values are clamped to the reviewed profile ceilings.
type Limits struct{ PluginBytes, MCPBytes, SkillBytes, FrontmatterBytes, AggregateBytes, Depth, Tokens, Members, Diagnostics int }

func DefaultLimits() Limits {
	return Limits{1 << 20, 4 << 20, 1 << 20, 64 << 10, 16 << 20, 64, 100000, 10000, 256}
}
func (l Limits) bounded() Limits {
	d := DefaultLimits()
	a := []*int{&l.PluginBytes, &l.MCPBytes, &l.SkillBytes, &l.FrontmatterBytes, &l.AggregateBytes, &l.Depth, &l.Tokens, &l.Members, &l.Diagnostics}
	b := []int{d.PluginBytes, d.MCPBytes, d.SkillBytes, d.FrontmatterBytes, d.AggregateBytes, d.Depth, d.Tokens, d.Members, d.Diagnostics}
	for i, p := range a {
		if *p <= 0 || *p > b[i] {
			*p = b[i]
		}
	}
	return l
}

type Decoder struct {
	Registry SchemaRegistry
	Limits   Limits
}

func (d Decoder) newFacts() Facts {
	return Facts{Profiles: ProfileIdentities(), Coverage: Coverage{Plugin: NotEvaluated, MCP: NotEvaluated, Skills: NotEvaluated, Filesystem: NotEvaluated}, raw: map[string]Document{}, maxFindings: d.Limits.bounded().Diagnostics}
}
func mergeOutcome(a, b Outcome) Outcome {
	if a == Fail || b == Fail {
		return Fail
	}
	if a != Pass || b != Pass {
		return NotEvaluated
	}
	return Pass
}
func (f *Facts) add(code, rule, path, pointer, item string, layer PolicyLayer, boundary domain.FailureBoundary) {
	if layer == Normative {
		f.normativeFailure = true
	}
	if f.maxFindings > 0 && len(f.Findings) >= f.maxFindings {
		f.truncated = true
		return
	}

	// Item names can themselves contain secrets. Use stable opaque identities in
	// public findings; canonical names remain available only in the internal model.
	if item != "" {
		item = sha256Digest([]byte(item))
	}
	f.Findings = append(f.Findings, Finding{Code: code, RuleRef: rule, Path: path, Pointer: pointer, Item: item, Layer: layer, Boundary: boundary, Severity: domain.SeverityError})
}
func (f *Facts) finish(l Limits) {
	sort.Slice(f.Findings, func(i, j int) bool {
		av, bv := f.Findings[i], f.Findings[j]
		av.ID = ""
		bv.ID = ""
		a, _ := json.Marshal(av)
		b, _ := json.Marshal(bv)
		return string(a) < string(b)
	})
	seen := map[string]bool{}
	out := f.Findings[:0]
	for _, v := range f.Findings {
		if v.Code == "diagnostics_truncated" {
			f.truncated = true
			continue
		}
		v.ID = ""
		b, _ := json.Marshal(v)
		v.ID = sha256Digest(b)
		if !seen[v.ID] {
			seen[v.ID] = true
			out = append(out, v)
		}
	}
	f.Findings = out
	if len(f.Findings) > l.Diagnostics || f.truncated {
		if len(f.Findings) >= l.Diagnostics {
			f.Findings = f.Findings[:l.Diagnostics-1]
		}
		marker := Finding{Code: "diagnostics_truncated", RuleRef: "host/bounds", Layer: HostSafety, Boundary: domain.BoundaryPlugin, Severity: domain.SeverityError}
		body, _ := json.Marshal(marker)
		marker.ID = sha256Digest(body)
		f.Findings = append(f.Findings, marker)
		f.Coverage.Complete = false
	}
	f.Conformance = mergeOutcome(mergeOutcome(f.Coverage.Plugin, f.Coverage.MCP), f.Coverage.Skills)
	f.Conformance = mergeOutcome(f.Conformance, f.Coverage.Filesystem)
	for _, v := range f.Findings {
		if v.Layer == Normative || f.normativeFailure {
			f.Conformance = Fail
		}
	}
	if f.normativeFailure {
		f.Conformance = Fail
	}
	f.Coverage.Complete = f.Coverage.Plugin != NotEvaluated && f.Coverage.MCP != NotEvaluated && f.Coverage.Skills != NotEvaluated && f.Coverage.Filesystem != NotEvaluated
	for _, v := range f.Findings {
		if v.Layer == HostSafety || v.Layer == InstallerPolicy {
			f.Coverage.Complete = false
			if f.Conformance == Pass {
				f.Conformance = NotEvaluated
			}
		}
	}
}
func validRelative(p string) bool {
	if p == "" || len(p) > 4096 || !utf8.ValidString(p) || strings.ContainsAny(p, "\\\x00") || strings.HasPrefix(p, "/") {
		return false
	}
	for _, s := range strings.Split(p, "/") {
		if s == "" || s == "." || s == ".." {
			return false
		}
	}
	return true
}
func (d Decoder) observe(ctx context.Context, f *Facts, doc Document, expected string, max int, boundary domain.FailureBoundary, optional bool) (bool, Outcome, error) {
	if err := ctx.Err(); err != nil {
		return false, NotEvaluated, err
	}
	if doc.Path != expected || !validRelative(doc.Path) {
		f.add("input_path_invalid", "host/input", reportPath(boundary), "", "", HostSafety, boundary)
		return false, NotEvaluated, nil
	}
	f.raw[doc.Path] = doc
	switch doc.State {
	case Absent:
		if optional {
			return false, Pass, nil
		}
		f.add("plugin_manifest_missing", "plugins/5.1", expected, "", "", Normative, boundary)
		return false, Fail, nil
	case Present:
		if len(doc.body) > max {
			f.add("document_bytes_limit", "host/bounds", reportPath(boundary), "", "", HostSafety, boundary)
			return false, NotEvaluated, nil
		}
		if !utf8.Valid(doc.body) {
			f.add("document_utf8_invalid", "host/utf8", reportPath(boundary), "", "", HostSafety, boundary)
			return false, NotEvaluated, nil
		}
		return true, Pass, nil
	case WrongKind:
		if boundary == domain.BoundarySkill {
			return false, Pass, nil
		} // Not discovered under §7.1.
		f.add("document_wrong_kind", "plugins/6.2", reportPath(boundary), "", "", Normative, boundary)
		return false, Fail, nil
	default:
		f.add("document_unavailable", "host/input", reportPath(boundary), "", "", HostSafety, boundary)
		return false, NotEvaluated, nil
	}
}
func reportPath(b domain.FailureBoundary) string {
	switch b {
	case domain.BoundaryMCP, domain.BoundaryMCPServer:
		return "mcp.json"
	case domain.BoundarySkill:
		return "skills"
	default:
		return "plugin.json"
	}
}
