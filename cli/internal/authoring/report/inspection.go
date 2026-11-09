package report

import (
	"regexp"
	"sort"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/project"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

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
	items := inspectionItems(pkg)
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

// inspectionItems projects only bounded display fields from inert package facts.
func inspectionItems(pkg *domain.PackageEnvelope) map[string]DisplayComponent {
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
	return items
}
