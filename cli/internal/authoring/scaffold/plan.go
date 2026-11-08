// Package scaffold provides private, offline standard-first template services.
// It does not wire commands or supply conformance policy.
package scaffold

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

)

type Template string

const (
	Skill     Template = "skill"
	MCPRemote Template = "mcp-remote"
	MCPStdio  Template = "mcp-stdio"
	Hybrid    Template = "hybrid"
)

// Options never consults the environment, Git, a clock, or an endpoint.
// Hybrid requires MCPChoice to be MCPRemote or MCPStdio. Stdio requires
// Runtime == "node"; Node >=22 is the only implemented runtime lane.
// SkillName defaults to Name, but is never silently normalized.
type Options struct {
	Template                       Template
	Name, Description, SkillName   string
	MCPChoice                      Template
	Runtime, RemoteURL             string
	AuthorName                     string
	License                        string // empty, MIT, or ISC; no default license
	CopyrightHolder, CopyrightYear string // required for either license
}

type File struct {
	Path  string // portable slash-separated relative path
	Bytes []byte
	Mode  fs.FileMode // exactly 0644 or 0755
}

// Plan is an immutable rendered tree. The zero value is invalid.
// Files returns a deep copy suitable for review. No destination is touched by planning.
type Plan struct{ files []File }

func (p Plan) Files() []File {
	files := make([]File, len(p.files))
	for i, f := range p.files {
		files[i] = f
		files[i].Bytes = append([]byte(nil), f.Bytes...)
	}
	return files
}

// NewPlan admits a deterministic, already-rendered standard package tree into
// the same validated staging and absence-preserving publication used by init.
// It is intentionally a narrow seam for explicit native import, not a generic
// filesystem builder.
func NewPlan(files []File) (Plan, error) {
	p := Plan{files: make([]File, len(files))}
	for i, f := range files {
		p.files[i] = f
		p.files[i].Bytes = append([]byte(nil), f.Bytes...)
	}
	sort.Slice(p.files, func(i, j int) bool { return p.files[i].Path < p.files[j].Path })
	if err := validateFiles(p.files); err != nil {
		return Plan{}, err
	}
	return p, nil
}

type IdentityError struct{ Field, Value, Suggestion string }

func (e *IdentityError) Error() string {
	return fmt.Sprintf("invalid %s %.128q; explicitly choose a valid identity, for example %q", e.Field, e.Value, e.Suggestion)
}

var skillName = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

func identityError(field, value string) error {
	preview := value
	if len(preview) > 128 {
		preview = preview[:128]
	}
	s := strings.Trim(nonSlug.ReplaceAllString(strings.ToLower(preview), "-"), "-")
	if len(s) > 64 {
		s = strings.TrimRight(s[:64], "-")
	}
	if s == "" || portableLeaf(s) != nil {
		s = "my-plugin"
	}
	return &IdentityError{field, value, s}
}

// nodeServerSource encodes every dynamic JavaScript string as a complete JSON literal.
// Keep serialization safe independently of the caller's identity validation.
func nodeServerSource(pluginName string) []byte {
	name, _ := json.Marshal(pluginName) // Marshaling a string cannot fail.
	greeting, _ := json.Marshal("Hello from " + pluginName + "!")
	// Write static code and complete encoded literals separately.
	var source bytes.Buffer
	source.WriteString("import { McpServer } from '@modelcontextprotocol/sdk/server/mcp.js';\nimport { StdioServerTransport } from '@modelcontextprotocol/sdk/server/stdio.js';\n\nconst server = new McpServer({ name: ")
	source.Write(name)
	source.WriteString(", version: '0.1.0' });\nserver.registerTool('hello', { description: 'Return a greeting', inputSchema: {} }, async () => ({\n  content: [{ type: 'text', text: ")
	source.Write(greeting)
	source.WriteString(" }],\n}));\nawait server.connect(new StdioServerTransport());\n")
	return source.Bytes()
}

func jsonBytes(v any) []byte {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		panic(err)
	}
	return append(b, '\n')
}
func textValue(v string, max int) bool {
	if len(v) > max*utf8.UTFMax || !utf8.ValidString(v) || strings.TrimSpace(v) == "" || utf8.RuneCountInString(v) > max {
		return false
	}
	for _, r := range v {
		if unicode.IsControl(r) && r != '\n' && r != '\t' {
			return false
		}
	}
	return true
}
func validateURL(raw string) error {
	if len(raw) > 4096 {
		return fmt.Errorf("remote URL exceeds 4096 bytes")
	}
	u, err := url.Parse(raw)
	if len(raw) > 4096 || err != nil || u == nil || (!strings.EqualFold(u.Scheme, "https") && !strings.EqualFold(u.Scheme, "http")) || u.Hostname() == "" || u.Opaque != "" || u.User != nil || u.Fragment != "" || strings.ContainsAny(raw, "\r\n\t ${}\\<>#") {
		return fmt.Errorf("remote MCP requires an explicit absolute http or https URL without credentials, fragments, or unresolved tokens")
	}
	if strings.EqualFold(u.Scheme, "http") && !strings.EqualFold(u.Hostname(), "localhost") {
		ip := net.ParseIP(u.Hostname())
		if ip == nil || !ip.IsLoopback() {
			return fmt.Errorf("remote template supports HTTPS, or HTTP on loopback only")
		}
	}
	if port := u.Port(); port != "" {
		n, e := strconv.Atoi(port)
		if e != nil || n < 1 || n > 65535 {
			return fmt.Errorf("remote URL port must be 1–65535")
		}
	} else if strings.HasSuffix(u.Host, ":") {
		return fmt.Errorf("remote URL port must not be empty")
	}
	return nil
}

// DefaultDescription is neutral deterministic template text for the explicit
// public command option. BuildPlan retains its explicit-input contract.
func DefaultDescription(t Template) string {
	switch t {
	case Skill:
		return "A Skill for documentation and task guidance."
	case MCPRemote:
		return "An Agent Plugins package with a remote MCP server."
	case MCPStdio:
		return "An Agent Plugins package with a local Node MCP server."
	case Hybrid:
		return "An Agent Plugins package with a Skill and an MCP server."
	default:
		return ""
	}
}
