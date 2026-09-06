package report

import (
	"regexp"
	"sort"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/project"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

const AuthoringSchemaVersion = 1

// Public embeds the existing policy report, with independently versioned command
// semantics. Inspection contains display values, never retained parser objects.
type CommandHelp struct {
	Use      string   `json:"use"`
	Flags    []string `json:"flags"`
	Guidance string   `json:"guidance"`
}

type Public struct {
	Help *CommandHelp `json:"help,omitempty"`
	Report
	AuthoringSchemaVersion int         `json:"authoring_schema_version"`
	EngineVersion          string      `json:"engine_version"`
	Requested              Requested   `json:"requested"`
	Effects                Effects     `json:"effects"`
	NextActions            []Action    `json:"next_actions"`
	Inspection             *Inspection `json:"inspection,omitempty"`
	Surface                []string    `json:"commands,omitempty"`
	WithheldPathIDs        []string    `json:"withheld_path_ids,omitempty"`
}
type Requested struct {
	Operation string `json:"operation"`
	Mode      string `json:"mode"`
}
type Effects struct {
	// Attempted means the selected service was entered, not that a write occurred.
	Attempted bool `json:"attempted"`
	Committed bool `json:"committed"`
}
type Action struct {
	Code      string `json:"code"`
	Operation string `json:"operation,omitempty"`
	Message   string `json:"message"`
}
type Inspection struct {
	Name       string             `json:"name,omitempty"`
	Version    string             `json:"version,omitempty"`
	Schema     string             `json:"schema"`
	Components []DisplayComponent `json:"components"`
	Truncated  bool               `json:"truncated,omitempty"`
}
type DisplayComponent struct {
	ID             string `json:"id"`
	Type           string `json:"type"`
	Name           string `json:"name,omitempty"`
	Namespace      string `json:"namespace,omitempty"`
	Executable     string `json:"executable,omitempty"`
	ExecutableKind string `json:"executable_kind,omitempty"`
}

// Recognize bounded GitLab PAT and Slack token families, including shapes
// embedded in a larger identity. Short ordinary names sharing a prefix remain
// useful. This supplements, rather than replaces, the existing display rules.
var publicCredentialFamily = regexp.MustCompile(`(?i)(glpat-[a-z0-9_-]{20}|xox[baprs]-[0-9]{10,13}-[0-9]{10,13}-[a-z0-9]{24})`)

// DisplayIdentity is a conservative display policy, never a conformance rule.
// It withholds whole values rather than leaking prefixes of unsafe input. Short
// ordinary names and non-SemVer versions remain useful; paths, controls, URL and
// credential-like tokens do not become public identity or suggestion text.
func DisplayIdentity(s string) string {
	if len(s) == 0 || len(s) > 64 {
		return ""
	}
	if publicCredentialFamily.MatchString(s) {
		return ""
	}
	lower := strings.ToLower(s)
	for _, token := range []string{"secret", "token", "password", "passwd", "credential", "authorization", "bearer", "ghp_", "gho_", "github_pat", "sk-", "akia"} {
		if strings.Contains(lower, token) {
			return ""
		}
	}
	run := 0
	for _, c := range s {
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' {
			run++
			if run > 24 {
				return ""
			}
			continue
		}
		if !strings.ContainsRune("._-+", c) {
			return ""
		}
		run = 0
	}
	if s == "." || s == ".." || strings.Contains(s, "..") {
		return ""
	}
	return s
}

func inspection(p project.Result) *Inspection {
	pkg := p.Facts.Package
	if pkg == nil {
		return nil
	}
	d := &Inspection{Name: DisplayIdentity(pkg.Manifest.Name), Version: DisplayIdentity(pkg.Manifest.Version), Schema: pkg.SchemaURI, Components: []DisplayComponent{}}
	// Iterate sorted identities, so the bounded display is deterministic even for
	// a package larger than the display budget. The policy report retains coverage.
	items := map[string]DisplayComponent{}
	add := func(kind, name, typ string) DisplayComponent {
		return DisplayComponent{ID: opaque(kind + ":" + name), Type: typ, Name: DisplayIdentity(name)}
	}
	for name, s := range pkg.MCP.Servers {
		c := add("mcp", name, "mcp_"+s.Type)
		if req := s.StdioRequirement; req != nil {
			c.ExecutableKind = string(req.Kind)
			// Bare executable names are requirements, not argv. Bundled paths remain
			// opaque; no source/scratch path or arbitrary launcher argument is exposed.
			if req.Kind == domain.ExecutableBare {
				c.Executable = DisplayIdentity(req.Command)
			}
		}
		items[c.ID] = c
	}
	for name := range pkg.MCP.InvalidServer {
		c := add("mcp", name, "mcp_server")
		items[c.ID] = c
	}
	for name := range pkg.Skills {
		c := add("skill", name, "skill")
		items[c.ID] = c
	}
	for _, name := range pkg.Inventory.InvalidSkills {
		c := add("skill", name, "skill")
		items[c.ID] = c
	}
	for name := range pkg.Manifest.Extensions {
		c := add("extension", name, "opaque_extension")
		c.Namespace = c.Name
		c.Name = ""
		items[c.ID] = c
	}
	ids := make([]string, 0, len(items))
	for id := range items {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for i, id := range ids {
		if i == 128 {
			d.Truncated = true
			break
		}
		d.Components = append(d.Components, items[id])
	}
	return d
}

// AddOperationError does not evaluate package policies. Syntax, help and
// cancellation before decoding supply no facts about package validity or safety.
func (r *Report) AddOperationError(code, action string) {
	r.Error = &Error{Code: code, Action: action}
	r.add(Finding{Code: code, Layer: "operation", Rule: "authoring/command", Severity: "error"})
	r.finish()
}

// PublicResult is a value projection: it does not rewrite installer facts or the
// private vertical-slice report, including its diagnostic IDs and policy states.
func (r Report) PublicResult(operation, mode string, attempted bool, surface []string) Public {
	r.Command, r.Mode = operation, mode
	r.Findings = append([]Finding{}, r.Findings...)
	replacements := map[string]string{}
	for i, f := range r.Findings {
		if f.Code != "plugin_manifest_missing" {
			continue
		}
		old := f.ID
		f.Code = "missing_standard_manifest"
		f.ID = ""
		// Use the same stable finding constructor, then remap every reference.
		tmp := New("", "")
		f.ID = tmp.add(f)
		r.Findings[i] = f
		replacements[old] = f.ID
	}
	remap := func(a Assessment) Assessment {
		a.FindingIDs = append([]string{}, a.FindingIDs...)
		for i, id := range a.FindingIDs {
			if replacement, ok := replacements[id]; ok {
				a.FindingIDs[i] = replacement
			}
		}
		return a
	}
	r.Loadability = remap(r.Loadability)
	r.Conformance = remap(r.Conformance)
	r.HostSafety = remap(r.HostSafety)
	r.Readiness = remap(r.Readiness)
	r.Release = remap(r.Release)
	r.Runtime = remap(r.Runtime)
	r.Compatibility = remap(r.Compatibility)
	r.Toolchain = remap(r.Toolchain)
	r.Checks = append([]Check{}, r.Checks...)
	for i := range r.Checks {
		r.Checks[i].Assessment = remap(r.Checks[i].Assessment)
	}
	r.finish()
	p := Public{Report: r, AuthoringSchemaVersion: AuthoringSchemaVersion, EngineVersion: r.Engine,
		Requested: Requested{operation, mode}, Effects: Effects{attempted, r.Committed}, NextActions: []Action{}, Inspection: r.Display, Surface: surface}
	p.Paths = []string{}
	for _, path := range r.Paths {
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
	if r.Error != nil {
		p.NextActions = append(p.NextActions, Action{Code: r.Error.Code, Message: r.Error.Action})
	}
	if len(replacements) > 0 {
		p.NextActions = append(p.NextActions, Action{Code: "missing_standard_manifest", Message: "Select the exact package root containing plugin.json; no ancestor search or format fallback is performed."})
	}
	if r.Error == nil && len(replacements) == 0 && r.Readiness.Status == Fail {
		p.NextActions = append(p.NextActions, Action{"review_findings", "author.validate", "Review the typed findings by policy layer, correct the selected package, then validate it again."})
	}
	if r.Legacy != "" && r.Legacy != "absent" {
		p.NextActions = append(p.NextActions, Action{Code: "legacy_workflow_unavailable", Message: "This v1 workflow is unavailable in v2. Use plugin-kit-ai 1.2.4 for the legacy workflow. Project migration is unavailable in v2."})
	}
	if r.Error == nil && r.Readiness.Status == Pass {
		if operation == "author.init" || operation == "author.skills.init" {
			p.NextActions = append(p.NextActions, Action{"inspect_package", "author.inspect", "Inspect the created package using its explicit package path."})
		}
		p.NextActions = append(p.NextActions, Action{"static_test", "author.test", "Run the offline static test with the package path; runtime behavior remains unevaluated."})
	}
	return p
}
