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

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/specregistry"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
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

func BuildPlan(o Options) (Plan, error) {
	fail := func(err error) (Plan, error) { return Plan{}, err }
	if len(o.Template) > 32 || len(o.MCPChoice) > 32 || len(o.Runtime) > 32 || len(o.License) > 32 {
		return fail(fmt.Errorf("template, MCP choice, runtime, and license values must be bounded identifiers"))
	}
	if len(o.Name) > 64 || !utf8.ValidString(o.Name) {
		return fail(identityError("plugin name", o.Name))
	}
	registry, err := specregistry.New()
	if err != nil {
		return fail(err)
	}
	manifest := map[string]any{"$schema": domain.PluginSchemaV1, "name": o.Name, "version": "0.1.0"}
	if registry.Validate(domain.PluginSchemaV1, manifest) != nil {
		return fail(identityError("plugin name", o.Name))
	}
	if err = portableLeaf(o.Name); err != nil {
		return fail(fmt.Errorf("%w (%v)", identityError("host-safe plugin name", o.Name), err))
	}
	if !textValue(o.Description, 1024) {
		return fail(fmt.Errorf("description must contain 1–1024 characters of nonempty text"))
	}
	manifest["description"] = o.Description
	if o.AuthorName != "" {
		if !textValue(o.AuthorName, 256) {
			return fail(fmt.Errorf("invalid explicit author name"))
		}
		manifest["author"] = map[string]any{"name": o.AuthorName}
	}
	hasSkill := o.Template == Skill || o.Template == Hybrid
	lane := o.Template
	switch o.Template {
	case Skill, MCPRemote, MCPStdio:
		if o.MCPChoice != "" {
			return fail(fmt.Errorf("MCP choice is only accepted for hybrid"))
		}
	case Hybrid:
		if o.MCPChoice != MCPRemote && o.MCPChoice != MCPStdio {
			return fail(fmt.Errorf("hybrid requires an explicit mcp-remote or mcp-stdio choice"))
		}
		lane = o.MCPChoice
	default:
		return fail(fmt.Errorf("unsupported template %q", o.Template))
	}
	if lane == MCPStdio {
		if o.Runtime != "node" {
			return fail(fmt.Errorf("stdio requires runtime node (Node >=22); runtime %q is not implemented", o.Runtime))
		}
		if o.RemoteURL != "" {
			return fail(fmt.Errorf("remote URL is not accepted for stdio"))
		}
	} else if o.Runtime != "" {
		return fail(fmt.Errorf("runtime is only accepted for stdio; only node is implemented"))
	}
	if lane == MCPRemote {
		if err = validateURL(o.RemoteURL); err != nil {
			return fail(err)
		}
	} else if o.RemoteURL != "" {
		return fail(fmt.Errorf("remote URL is only accepted for remote MCP"))
	}
	if hasSkill {
		if o.SkillName == "" {
			o.SkillName = o.Name
		}
		if len(o.SkillName) > 64 || !skillName.MatchString(o.SkillName) {
			return fail(identityError("skill name", o.SkillName))
		}
		if err = portableLeaf(o.SkillName); err != nil {
			return fail(fmt.Errorf("%w (%v)", identityError("host-safe skill name", o.SkillName), err))
		}
	} else if o.SkillName != "" {
		return fail(fmt.Errorf("skill name requires a skill component"))
	}
	license, err := licenseText(o)
	if err != nil {
		return fail(err)
	}
	if license != "" {
		manifest["license"] = o.License
	}
	p := Plan{}
	add := func(path string, b []byte) { p.files = append(p.files, File{path, b, 0644}) }
	if err = registry.Validate(domain.PluginSchemaV1, manifest); err != nil {
		return fail(err)
	}
	add("plugin.json", jsonBytes(manifest))
	add(".gitignore", []byte("node_modules/\n.DS_Store\n"))
	readme := "# " + o.Name + "\n\n" + o.Description + "\n\nThis package uses Agent Plugins 1.0: `plugin.json`, with portable components in `skills/` and/or `mcp.json`.\n"
	if lane == MCPStdio {
		readme += "\nThe stdio server requires Node >=22 and the official MCP SDK pinned in package-lock.json. Dependency installation and runtime execution are separate, explicit author actions. Creation performs neither; runtime behavior has not been tested.\n"
	}
	if lane == MCPRemote {
		readme += "\nThe remote MCP URL is configuration only. Creation does not contact the endpoint or verify authentication or runtime behavior.\n"
	}
	add("README.md", []byte(readme))
	if license != "" {
		add("LICENSE", []byte(license))
	}
	if hasSkill {
		desc, _ := json.Marshal(o.Description)
		add("skills/"+o.SkillName+"/SKILL.md", []byte("---\nname: "+o.SkillName+"\ndescription: "+string(desc)+"\n---\n\n# "+o.SkillName+"\n\n"+o.Description+"\n\nUse this skill when the request matches its description. Clarify missing requirements before taking action and report the result.\n"))
	}
	if lane == MCPRemote || lane == MCPStdio {
		server := map[string]any{"type": "streamable-http", "url": o.RemoteURL}
		if lane == MCPStdio {
			server = map[string]any{"type": "stdio", "command": "node", "args": []any{"${PLUGIN_ROOT}/src/server.mjs"}}
		}
		mcp := map[string]any{"$schema": domain.MCPSchemaV1, "mcpServers": map[string]any{o.Name: server}}
		if err = registry.Validate(domain.MCPSchemaV1, mcp); err != nil {
			return fail(err)
		}
		add("mcp.json", jsonBytes(mcp))
	}
	if lane == MCPStdio {
		pkg, lock, err := nodePackages(o.Name)
		if err != nil {
			return fail(err)
		}
		add("package.json", pkg)
		add("package-lock.json", lock)
		add("src/server.mjs", nodeServerSource(o.Name))
	}
	sort.Slice(p.files, func(i, j int) bool { return p.files[i].Path < p.files[j].Path })
	if err = validateFiles(p.files); err != nil {
		return fail(err)
	}
	return p, nil
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
