package commands_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/commands"
	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/project"
	"github.com/777genius/plugin-kit-ai/cli/internal/authoring/report"
	"github.com/777genius/plugin-kit-ai/cli/internal/authoringcli"
	"github.com/777genius/plugin-kit-ai/cli/internal/exitx"
	"github.com/spf13/cobra"
)

const publicRevision = "ade20cefd90f85e6bec80754c83d1f2865701c9d"

type publicEnvelope struct {
	SchemaVersion int           `json:"schema_version"`
	Command       string        `json:"command"`
	Result        string        `json:"result"`
	Data          report.Public `json:"data"`
}

func publicApp(t *testing.T) commands.App {
	t.Helper()
	return commands.App{Projects: project.Service{Scratch: t.TempDir()}, Revision: publicRevision, PublicContract: true}
}
func publicDecode(t *testing.T, raw rawExecution) (publicEnvelope, int) {
	t.Helper()
	if len(raw.errout) != 0 {
		t.Fatalf("unexpected stderr: %s", raw.errout)
	}
	var e publicEnvelope
	d := json.NewDecoder(bytes.NewReader(raw.out))
	d.DisallowUnknownFields()
	if err := d.Decode(&e); err != nil {
		t.Fatalf("decode: %v\n%s", err, raw.out)
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		t.Fatalf("more than one document: %v", err)
	}
	code := 0
	if raw.err != nil {
		code = exitx.Code(raw.err)
	}
	expected := "success"
	if code != 0 {
		expected = "failure"
	}
	if e.SchemaVersion != 1 || e.Result != expected || e.Data.AuthoringSchemaVersion != 1 || e.Data.EngineVersion != "standard-first-slice/1" || e.Data.Revision != publicRevision {
		t.Fatalf("identity/result: %+v", e)
	}
	if e.Command != e.Data.Command || e.Command != e.Data.Requested.Operation || e.Data.Requested.Mode != e.Data.Mode || e.Data.Committed != e.Data.Effects.Committed {
		t.Fatalf("operation/effects disagree: %+v", e)
	}
	ids := map[string]bool{}
	for _, f := range e.Data.Findings {
		if ids[f.ID] {
			t.Fatal("duplicate finding")
		}
		ids[f.ID] = true
	}
	assessments := []report.Assessment{e.Data.Conformance, e.Data.Readiness, e.Data.HostSafety, e.Data.Release, e.Data.Loadability, e.Data.Compatibility, e.Data.Runtime, e.Data.Toolchain}
	for _, c := range e.Data.Checks {
		assessments = append(assessments, c.Assessment)
	}
	for _, a := range assessments {
		seen := map[string]bool{}
		for _, id := range a.FindingIDs {
			if !ids[id] || seen[id] {
				t.Fatalf("invalid reference %s", id)
			}
			seen[id] = true
		}
	}
	return e, code
}
func publicRun(t *testing.T, a commands.App, args []string, mount bool) (publicEnvelope, int, []byte) {
	t.Helper()
	raw := executeRaw(a, args, mount)
	e, c := publicDecode(t, raw)
	return e, c, raw.out
}
func noPolicy(t *testing.T, e publicEnvelope) {
	t.Helper()
	for _, a := range []report.Assessment{e.Data.Conformance, e.Data.Readiness, e.Data.HostSafety, e.Data.Release, e.Data.Loadability, e.Data.Compatibility, e.Data.Runtime, e.Data.Toolchain} {
		if a.Status != report.NotEvaluated {
			t.Fatalf("unevaluated policy changed: %+v", e)
		}
	}
	if e.Data.Effects.Attempted || e.Data.Committed || len(e.Data.Paths) != 0 {
		t.Fatalf("syntax/help claimed effects: %+v", e)
	}
}

func TestPublicParserAndHelpParity(t *testing.T) {
	a := publicApp(t)
	cases := []struct {
		args      []string
		operation string
		code      int
	}{
		{nil, "author", 0},
		{[]string{"--help"}, "author", 0},
		{[]string{"skills"}, "author.skills", 0},
		{[]string{"skills", "--help"}, "author.skills", 0},
		{[]string{"help", "skills", "init"}, "author.skills.init", 0},
		{[]string{"skills", "init", "-h"}, "author.skills.init", 0},
		{[]string{"init", "--help"}, "author.init", 0},
		{[]string{"init", "--include-root=invalid-secret", "--help"}, "author.init", 2},
		{[]string{"help", "skills", "init", "unknown-secret"}, "author.skills.init", 2},
		{[]string{"init"}, "author.init", 2},
		{[]string{"skills", "init"}, "author.skills.init", 2},
		{[]string{"skills", "unknown-secret"}, "author.skills", 2},
		{[]string{"unknown-secret"}, "author", 2},
		{[]string{"help", "unknown-secret"}, "author", 2},
		{[]string{"init", "--description", "test"}, "author.init", 2},
		{[]string{"init", "--description=author", "--name", "inspect"}, "author.init", 2},
		{[]string{"--target", "init", "skills", "init"}, "author.skills.init", 2},
		{[]string{"--unknown-secret", "init"}, "author", 2},
		{[]string{"validate", "--unknown-secret=inspect"}, "author.validate", 2},
		{[]string{"skills", "validate", "--unknown-secret"}, "author.skills.validate", 2},
		{[]string{"init", "--description"}, "author.init", 2},
		{[]string{"init", "-hx"}, "author.init", 2},
		{[]string{"validate", "one", "two"}, "author.validate", 2},
		{[]string{"init", "--help=false"}, "author.init", 2},
		{[]string{"init", "--include-root=invalid-secret"}, "author.init", 2},
		{[]string{"init", "--dry-run=false", "--help"}, "author.init", 2},
		{[]string{"init", "--target=claude", "--help"}, "author.init", 2},
		{[]string{"skills", "--target=claude", "--help"}, "author.skills", 2},
		{[]string{"compat"}, "author.compat", 2},
		{[]string{"init", "demo", "--template", "hybrid"}, "author.init", 2},
		{[]string{"init", "demo", "--template", "mcp-remote"}, "author.init", 2},
		{[]string{"init", "demo", "--template", "mcp-stdio"}, "author.init", 2},
		{[]string{"init", "demo", "--template", "skill", "--description="}, "author.init", 2},
		{[]string{"skills", "init", "demo"}, "author.skills.init", 2},
	}
	for _, tc := range cases {
		t.Run(strings.Join(tc.args, "_"), func(t *testing.T) {
			var first []byte
			for _, mount := range []bool{false, true} {
				args := append([]string{"--format=json"}, tc.args...)
				e, c, b := publicRun(t, a, args, mount)
				if c != tc.code || e.Command != tc.operation {
					t.Fatalf("code %d operation %s: %s", c, e.Command, b)
				}
				noPolicy(t, e)
				if bytes.Contains(b, []byte("unknown-secret")) || bytes.Contains(b, []byte("invalid-secret")) {
					t.Fatalf("token echo: %s", b)
				}
				if e.Data.Help != nil {
					// Assert the actual route before normalizing only its allowed
					// invocation prefix; all remaining payload bytes still match.
					prefix := assertPublicHelpRoute(t, e, mount)
					b = bytes.Replace(b, []byte(`"use":"`+prefix), []byte(`"use":"author`), 1)
				}
				if first != nil && !bytes.Equal(first, b) {
					t.Fatalf("entrypoint drift:\n%s\n%s", first, b)
				}
				first = b
			}
		})
	}
	if files, _ := os.ReadDir(a.Projects.Scratch); len(files) != 0 {
		t.Fatalf("scratch leaked: %v", files)
	}
}

func assertPublicHelpRoute(t *testing.T, e publicEnvelope, mount bool) string {
	t.Helper()
	prefix := "plugin-kit-ai"
	if mount {
		prefix = "agentplugins author"
	}
	route := strings.ReplaceAll(strings.TrimPrefix(e.Command, "author"), ".", " ")
	suffix := " [package-path]"
	switch e.Command {
	case "author":
		suffix = " <command>"
	case "author.skills", "author.capabilities":
		suffix = ""
	case "author.init":
		suffix = " <path>"
	case "author.skills.init":
		suffix = " <name> [package-path]"
	}
	if e.Data.Help == nil || e.Data.Help.Use != prefix+route+suffix {
		t.Fatalf("incorrect help route for %s", e.Command)
	}
	return prefix
}

func TestPublicHelpExecutableAndAncestry(t *testing.T) {
	for _, route := range []string{"", "skills", "init", "inspect", "validate", "test", "compat", "doctor", "capabilities", "skills init", "skills validate"} {
		for _, mount := range []bool{false, true} {
			for _, helpVerb := range []bool{false, true} {
				args := append(strings.Fields(route), "--help")
				if helpVerb {
					args = append([]string{"help"}, strings.Fields(route)...)
				}
				a := publicApp(t)
				e, code, _ := publicRun(t, a, append(args, "--format=json"), mount)
				if code != 0 {
					t.Fatal("help failed")
				}
				assertPublicHelpRoute(t, e, mount)
				noPolicy(t, e)
				human := executeRaw(a, args, mount)
				if human.err != nil || len(human.errout) != 0 || !bytes.HasPrefix(human.out, []byte("Usage: "+e.Data.Help.Use+"\n")) {
					t.Fatal("human usage disagrees with the verified JSON route")
				}
			}
		}
	}
}

func TestPublicCredentialFamilySinks(t *testing.T) {
	// Non-live synthetic values are assembled only in memory. Failure logs never
	// include shaped inputs or raw output, even when exercising a disclosure bug.
	for family, value := range map[string]string{
		"gitlab": "glpat-" + strings.Repeat("b", 20),
		"slack":  "xoxb-" + strings.Repeat("1", 12) + "-" + strings.Repeat("2", 12) + "-" + strings.Repeat("a", 24),
	} {
		for _, field := range []string{"name", "version", "server", "namespace", "executable", "skill"} {
			for _, mount := range []bool{false, true} {
				for _, format := range []string{"json", "human"} {
					t.Run(fmt.Sprintf("%s/%s/mount=%t/%s", family, field, mount, format), func(t *testing.T) {
						a, root := publicApp(t), t.TempDir()
						manifest := map[string]any{"$schema": "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json", "name": "visible-demo", "version": "preview-1"}
						serverName := "visible-server"
						server := map[string]any{"type": "stdio", "command": "node", "args": []string{"server.mjs"}}
						switch field {
						case "name", "version":
							manifest[field] = value
						case "server":
							serverName = value
						case "namespace":
							manifest["extensions"] = map[string]any{value: map[string]any{}}
						case "executable":
							server["command"] = value
						}
						body, _ := json.Marshal(manifest)
						write(t, root, "plugin.json", string(body))
						body, _ = json.Marshal(map[string]any{"$schema": "https://agent-plugins.org/schemas/1.0.0/mcp.schema.json", "mcpServers": map[string]any{serverName: server}})
						write(t, root, "mcp.json", string(body))
						check := func(args []string, mutation bool) {
							raw := executeRaw(a, append(args, "--format="+format), mount)
							disclosed := bytes.Contains(raw.out, []byte(value)) || bytes.Contains(raw.errout, []byte(value))
							t.Logf("disclosed=%t output_sha256=%x", disclosed, sha256.Sum256(raw.out))
							if disclosed || raw.err != nil || len(raw.errout) != 0 {
								t.Fatal("credential projection disclosed input or operation failed")
							}
							if format == "json" {
								e, _ := publicDecode(t, raw) // Safe only after the disclosure assertion.
								if e.Data.Committed != mutation || mutation && (len(e.Data.Paths) != 0 || len(e.Data.WithheldPathIDs) != 1 || !e.Data.Effects.Attempted) {
									t.Fatal("incorrect mutation projection")
								}
							}
						}
						if field == "skill" {
							check([]string{"skills", "init", value, root, "--description=Example"}, true)
							if _, err := os.Stat(filepath.Join(root, "skills", value, "SKILL.md")); err != nil {
								t.Fatal("display policy prevented the requested Skill commit")
							}
						}
						before := tree(t, root)
						check([]string{"inspect", root}, false)
						if !reflect.DeepEqual(before, tree(t, root)) {
							t.Fatal("inspection changed source")
						}
					})
				}
			}
		}
	}
}

func TestPublicFormatArityAndDelimiter(t *testing.T) {
	a := publicApp(t)
	for _, tc := range []struct {
		args []string
		json bool
	}{
		{[]string{"--format=json", "--format", "human", "init"}, false},
		{[]string{"--format=human", "--format", "json", "init"}, true},
		{[]string{"init", "--description", "--format=json"}, false},
		{[]string{"--format=json", "init", "--description", "--format=human"}, true},
		{[]string{"init", "--", "--format=json"}, false},
		{[]string{"--format=json", "init", "--", "--format=human"}, true},
		{[]string{"--format=json", "init", "--format"}, true},
	} {
		for _, mount := range []bool{false, true} {
			raw := executeRaw(a, tc.args, mount)
			if bytes.HasPrefix(raw.out, []byte("{")) != tc.json {
				t.Fatalf("format selection %v: %s", tc.args, raw.out)
			}
			if tc.json {
				e, c := publicDecode(t, raw)
				if c != 2 || e.Command != "author.init" {
					t.Fatalf("unexpected result %+v code %d", e, c)
				}
			}
		}
	}
	// A path named like a command is still an argument after a leaf or --.
	for _, args := range [][]string{{"validate", "test", "init"}, {"--format=json", "--", "init"}} {
		if !strings.HasPrefix(args[0], "--format") {
			args = append([]string{"--format=json"}, args...)
		}
		e, c, _ := publicRun(t, a, args, false)
		if c != 2 || e.Data.Effects.Attempted {
			t.Fatalf("delimiter/position selection: %+v", e)
		}
	}
}

func TestPublicLiteralExamplesAndEveryLeaf(t *testing.T) {
	lanes := []struct {
		name  string
		flags []string
	}{
		{"docs-skill", []string{"--template", "skill"}},
		{"docs-helper", []string{"--template", "mcp-remote", "--url", "https://docs.example.com/mcp"}},
		{"local-helper", []string{"--template", "mcp-stdio", "--runtime", "node"}},
		{"hybrid-helper", []string{"--template", "hybrid", "--mcp-template", "mcp-remote", "--url", "https://docs.example.com/mcp"}},
		{"hybrid-local", []string{"--template", "hybrid", "--mcp-template", "mcp-stdio", "--runtime", "node"}},
	}
	for _, lane := range lanes {
		t.Run(lane.name, func(t *testing.T) {
			var first [][]byte
			var firstTree map[string]string
			for _, mount := range []bool{false, true} {
				parent := physicalMutationRoot(t)
				t.Chdir(parent)
				a := publicApp(t)
				args := append([]string{"init", lane.name, "--format=json"}, lane.flags...)
				e, c, b := publicRun(t, a, args, mount)
				if c != 0 || !e.Data.Committed || !e.Data.Effects.Attempted || e.Data.Mode != "local_mutation" || e.Data.Inspection.Name != lane.name {
					t.Fatalf("literal example: %s", b)
				}
				root := filepath.Join(parent, lane.name)
				body, err := os.ReadFile(filepath.Join(root, "plugin.json"))
				if err != nil {
					t.Fatal(err)
				}
				var manifest map[string]any
				json.Unmarshal(body, &manifest)
				if manifest["version"] != "0.1.0" || manifest["description"] == "" || manifest["author"] != nil || manifest["license"] != nil {
					t.Fatalf("defaults: %s", body)
				}
				for _, path := range []string{"LICENSE", "plugin/plugin.yaml", ".codex-plugin/plugin.json", "hooks"} {
					if _, err := os.Stat(filepath.Join(root, path)); !os.IsNotExist(err) {
						t.Fatalf("unexpected generated %s", path)
					}
				}
				gotTree := tree(t, root)
				if firstTree == nil {
					firstTree = gotTree
				} else if !reflect.DeepEqual(firstTree, gotTree) {
					t.Fatal("template tree parity")
				}
				results := [][]byte{b}
				for _, verb := range []string{"validate", "inspect", "test", "compat", "doctor", "skills validate"} {
					args := append(strings.Fields(verb), root, "--format=json")
					if verb == "compat" {
						args = append(args, "--target", "claude,codex")
					}
					r, code, out := publicRun(t, a, args, mount)
					if verb != "doctor" && code != 0 {
						t.Fatalf("%s: %s", verb, out)
					}
					if r.Data.Conformance.Status != report.Pass || r.Data.Runtime.Status != report.NotEvaluated || r.Data.Identity.TreeDigest == "" || r.Data.Root != "" {
						t.Fatalf("policy/identity: %s", out)
					}
					if bytes.Contains(out, []byte(parent)) || bytes.Contains(out, []byte(a.Projects.Scratch)) {
						t.Fatalf("local path leaked: %s", out)
					}
					results = append(results, out)
				}
				r, code, out := publicRun(t, a, []string{"skills", "init", "extra-skill", root, "--description", "Use for extra documentation requests", "--format=json"}, mount)
				if code != 0 || !r.Data.Committed || !reflect.DeepEqual(r.Data.Paths, []string{"skills/extra-skill/SKILL.md"}) {
					t.Fatalf("Skills mutation: %s", out)
				}
				results = append(results, out)
				if first == nil {
					first = results
				} else if !reflect.DeepEqual(first, results) {
					t.Fatal("full policy/digest/identity parity")
				}
				if entries, _ := os.ReadDir(a.Projects.Scratch); len(entries) != 0 {
					t.Fatalf("scratch leaked: %v", entries)
				}
			}
		})
	}
}

func TestPublicCWDAndExplicitInputs(t *testing.T) {
	parent := physicalMutationRoot(t)
	t.Chdir(parent)
	a := publicApp(t)
	root := filepath.Join(parent, "explicit-destination")
	_, c, b := publicRun(t, a, []string{"init", root, "--name", "explicit-name", "--description", "Explicit description", "--template", "skill", "--format=json"}, false)
	if c != 0 {
		t.Fatalf("explicit caller: %s", b)
	}
	before := tree(t, root)
	_, c, b = publicRun(t, a, []string{"init", root, "--name", "other-name", "--template", "skill", "--format=json"}, true)
	if c != 1 || !reflect.DeepEqual(before, tree(t, root)) {
		t.Fatalf("collision changed destination: %s", b)
	}
	t.Chdir(root)
	for _, mount := range []bool{false, true} {
		for _, verb := range []string{"validate", "inspect", "test", "doctor", "compat", "skills validate"} {
			args := append(strings.Fields(verb), "--include-root", "--format=json")
			if verb == "compat" {
				args = append(args, "--target=claude,codex")
			}
			e, c, b := publicRun(t, a, args, mount)
			if c != 0 || e.Data.Root != root {
				t.Fatalf("omitted exact CWD: %s", b)
			}
		}
	}
	child := filepath.Join(root, "child")
	os.Mkdir(child, 0700)
	t.Chdir(child)
	e, c, b := publicRun(t, a, []string{"validate", "--format=json"}, false)
	if c != 1 || !bytes.Contains(b, []byte("missing_standard_manifest")) || e.Data.Inspection != nil {
		t.Fatalf("ancestor search: %s", b)
	}
	for _, args := range [][]string{
		{"init", filepath.Join(parent, "ambiguous"), "--template", "skill"},
		{"init", "UPPER", "--template", "skill"},
		{"init", "demo", "--name=", "--template", "skill"},
		{"init", "demo", "--template", "hybrid", "--mcp", "mcp-remote"},
	} {
		_, c, b := publicRun(t, a, append(args, "--format=json"), false)
		if c != 2 {
			t.Fatalf("unsafe implicit input: %s", b)
		}
	}
	_, _, b = publicRun(t, a, []string{"init", "UPPER", "--template", "skill", "--format=json"}, false)
	if !bytes.Contains(b, []byte("Suggested identity: upper")) {
		t.Fatalf("safe suggestion missing: %s", b)
	}
}

func TestPublicFailureBoundariesAndCredentialProjection(t *testing.T) {
	a := publicApp(t)
	for _, tc := range []struct {
		name, plugin, mcp string
		legacy            bool
		code              int
	}{
		{"missing", "", "", false, 1},
		{"legacy-only", "", "", true, 1},
		{"both", `{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"demo"}`, "", true, 1},
		{"malformed", `{"SECRET-PARSER-VALUE":`, "", false, 1},
		{"unknown-schema", `{"$schema":"https://secret.invalid/SECRET-SCHEMA","name":"demo"}`, "", false, 1},
		{"bad-core", `{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":123}`, "", false, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			if tc.plugin != "" {
				write(t, root, "plugin.json", tc.plugin)
			}
			if tc.legacy {
				write(t, root, "plugin/plugin.yaml", "SECRET-LEGACY-BYTES")
			}
			var first []byte
			for _, mount := range []bool{false, true} {
				e, c, b := publicRun(t, a, []string{"validate", root, "--release-policy", "--format=json"}, mount)
				if c != tc.code || bytes.Contains(b, []byte("SECRET")) {
					t.Fatalf("failure projection: %s", b)
				}
				if tc.name == "missing" || tc.name == "legacy-only" {
					if !bytes.Contains(b, []byte("missing_standard_manifest")) || bytes.Contains(b, []byte("plugin_manifest_missing")) {
						t.Fatalf("missing projection: %s", b)
					}
				}
				if tc.legacy && (!bytes.Contains(b, []byte("1.2.4")) || !bytes.Contains(b, []byte("migration is unavailable")) || bytes.Contains(b, []byte("migrate project"))) {
					t.Fatalf("legacy guidance: %s", b)
				}
				if tc.name == "both" && (e.Data.Conformance.Status != report.Pass || e.Data.Release.Status != report.Fail) {
					t.Fatalf("policy conflation: %s", b)
				}
				if first != nil && !bytes.Equal(first, b) {
					t.Fatal("failure parity")
				}
				first = b
			}
		})
	}
	// Independently vary identity fields and private payload fields. Ordinary
	// display names remain useful; credential-like identities are withheld whole.
	for _, field := range []string{"ordinary", "name", "version", "server", "headers", "env", "args", "extension", "malformed-mcp"} {
		t.Run(field, func(t *testing.T) {
			root := t.TempDir()
			secret := "test-token"
			manifest := map[string]any{"$schema": "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json", "name": "public-demo", "version": "preview-1", "extensions": map[string]any{"com.example.docs": map[string]any{"private": secret}}}
			serverName := "docs-server"
			server := map[string]any{"type": "stdio", "command": "node", "args": []string{"server.mjs"}}
			switch field {
			case "name":
				manifest["name"] = secret
			case "version":
				manifest["version"] = secret
			case "server":
				serverName = secret
			case "headers":
				server = map[string]any{"type": "streamable-http", "url": "https://docs.example.com/mcp", "headers": map[string]any{"Authorization": secret}}
			case "env":
				server["env"] = map[string]any{"PASSWORD": secret}
			case "args":
				server["args"] = []string{secret}
			case "extension":
				manifest["extensions"] = map[string]any{secret: map[string]any{"value": secret}}
			}
			mb, _ := json.Marshal(manifest)
			write(t, root, "plugin.json", string(mb))
			mc, _ := json.Marshal(map[string]any{"$schema": "https://agent-plugins.org/schemas/1.0.0/mcp.schema.json", "mcpServers": map[string]any{serverName: server}})
			if field == "malformed-mcp" {
				mc = []byte(`{"` + secret + `":`)
			}
			write(t, root, "mcp.json", string(mc))
			for _, mount := range []bool{false, true} {
				e, _, b := publicRun(t, a, []string{"inspect", root, "--format=json"}, mount)
				if bytes.Contains(b, []byte(secret)) || bytes.Contains(b, []byte(root)) {
					t.Fatalf("credential/path echo: %s", b)
				}
				if field == "ordinary" && (e.Data.Inspection == nil || e.Data.Inspection.Name != "public-demo" || e.Data.Inspection.Version != "preview-1" || !bytes.Contains(b, []byte(`"namespace":"com.example.docs"`)) || !bytes.Contains(b, []byte(`"executable":"node"`))) {
					t.Fatalf("unusable inspection: %s", b)
				}
				human := executeRaw(a, []string{"inspect", root}, mount)
				if bytes.Contains(human.out, []byte(secret)) || bytes.Contains(human.out, []byte(root)) {
					t.Fatal("human inspection disclosed private data")
				}
			}
		})
	}
}

func TestPublicFreshInventoryCancellationAndOutput(t *testing.T) {
	a := publicApp(t)
	expected := []string{"author.capabilities", "author.compat", "author.doctor", "author.init", "author.inspect", "author.skills.init", "author.skills.validate", "author.test", "author.validate"}
	for _, mount := range []bool{false, true} {
		e, c, b := publicRun(t, a, []string{"capabilities", "--format=json"}, mount)
		if c != 0 || !reflect.DeepEqual(e.Data.Capabilities.Commands, expected) || !reflect.DeepEqual(e.Data.Surface, expected) {
			t.Fatalf("actual leaves: %s", b)
		}
		for _, name := range []string{"migrate", "bootstrap", "version", "install", "__docs"} {
			if bytes.Contains(b, []byte("author."+name)) {
				t.Fatalf("deferred name: %s", b)
			}
		}
	}
	// Remove a real factory leaf in the composition; no separate capability table
	// may keep advertising it. Add a real leaf with flag arity before error routing.
	builder := func(fs ...authoringcli.Factory) (*cobra.Command, error) {
		r, err := authoringcli.NewPluginKitRoot(fs...)
		if err != nil {
			return nil, err
		}
		for _, c := range r.Commands() {
			if c.Name() == "doctor" {
				r.RemoveCommand(c)
			}
		}
		r.PersistentFlags().StringP("fixture-option", "q", "", "test-only string arity")
		return r, nil
	}
	var out bytes.Buffer
	err := a.Execute(context.Background(), []string{"-qinit", "capabilities", "--format=json"}, authoringcli.Streams{Out: &out, Err: io.Discard}, builder)
	e, _ := publicDecode(t, rawExecution{out: out.Bytes(), err: err})
	if e.Command != "author.capabilities" || len(e.Data.Capabilities.Commands) != 8 {
		t.Fatalf("factory inventory/arity: %s", out.Bytes())
	}
	for _, args := range [][]string{{"--scope=user", "author", "init", "--help"}, {"--accept-security-risk=false", "author", "inspect", "--help"}, {"author", "skills", "--security-details=false", "--help"}} {
		out.Reset()
		err = a.Execute(context.Background(), append(args, "--format=json"), authoringcli.Streams{Out: &out, Err: io.Discard}, mounted)
		e, c := publicDecode(t, rawExecution{out: out.Bytes(), err: err})
		if c != 2 {
			t.Fatalf("installer flag accepted: %s", out.Bytes())
		}
		noPolicy(t, e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	out.Reset()
	err = a.Execute(ctx, []string{"inspect", "--format=json"}, authoringcli.Streams{Out: &out, Err: io.Discard}, authoringcli.NewPluginKitRoot)
	e, c := publicDecode(t, rawExecution{out: out.Bytes(), err: err})
	if c != 1 || e.Data.Error.Code != "canceled" {
		t.Fatalf("cancel: %s", out.Bytes())
	}
	noPolicy(t, e)
	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	checks := 0
	duringDecode := faultContext{Context: ctx, check: func() {
		checks++
		if checks == 3 {
			cancel()
		}
	}}
	out.Reset()
	err = a.Execute(duringDecode, []string{"skills", "init", "demo", "--description=Example", "--format=json"}, authoringcli.Streams{Out: &out, Err: io.Discard}, authoringcli.NewPluginKitRoot)
	e, c = publicDecode(t, rawExecution{out: out.Bytes(), err: err})
	if c != 1 || e.Data.Error.Code != "canceled" {
		t.Fatal("decode cancellation became syntax failure")
	}
	noPolicy(t, e)
	w := &publicBrokenWriter{}
	err = a.Execute(context.Background(), []string{"init", "--format=json"}, authoringcli.Streams{Out: w, Err: io.Discard}, authoringcli.NewPluginKitRoot)
	if exitx.Code(err) != 1 || w.calls != 1 {
		t.Fatalf("output precedence: %v calls %d", err, w.calls)
	}
	var wg sync.WaitGroup
	results := make([]rawExecution, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i] = executeRaw(a, []string{"skills", "--format=json"}, i%2 == 0)
		}(i)
	}
	wg.Wait()
	var first []byte
	for i, raw := range results {
		e, _ := publicDecode(t, raw)
		prefix := assertPublicHelpRoute(t, e, i%2 == 0)
		raw.out = bytes.Replace(raw.out, []byte(`"use":"`+prefix), []byte(`"use":"author`), 1)
		if first != nil && !bytes.Equal(first, raw.out) {
			t.Fatal("concurrent factory drift")
		}
		first = raw.out
	}
}

type publicBrokenWriter struct{ calls int }

func (w *publicBrokenWriter) Write([]byte) (int, error) {
	w.calls++
	return 0, errors.New("SECRET-output-error")
}

func TestPublicHelpUsesActualFlagsWithoutValues(t *testing.T) {
	a := publicApp(t)
	for _, mount := range []bool{false, true} {
		e, c, b := publicRun(t, a, []string{"init", "--description", "test-token", "--help", "--format=json"}, mount)
		if c != 0 || e.Data.Help == nil || !strings.Contains(strings.Join(e.Data.Help.Flags, ","), "--mcp-template <value>") || bytes.Contains(b, []byte("test-token")) || !strings.Contains(e.Data.Help.Guidance, "--scope") {
			t.Fatalf("help contract: %s", b)
		}
	}
}

func TestPublicAnnotatedErrorAndRealCleanup(t *testing.T) {
	a := publicApp(t)
	var out bytes.Buffer
	err := a.Execute(context.Background(), []string{"inspect", "--format=json"}, authoringcli.Streams{Out: &out, Err: io.Discard}, func(fs ...authoringcli.Factory) (*cobra.Command, error) {
		r, err := authoringcli.NewPluginKitRoot(fs...)
		r.PersistentPreRunE = func(*cobra.Command, []string) error { return exitx.Wrap(errors.New("test-token"), 1) }
		return r, err
	})
	e, code := publicDecode(t, rawExecution{out: out.Bytes(), err: err})
	if code != 1 || bytes.Contains(out.Bytes(), []byte("test-token")) {
		t.Fatal("annotated error lost or disclosed")
	}
	noPolicy(t, e)
	for _, canceled := range []bool{false, true} {
		for _, mount := range []bool{false, true} {
			parent := physicalMutationRoot(t)
			ctx, cancel := context.WithCancel(context.Background())
			stage := ""
			fault := faultContext{Context: ctx, check: func() {
				if stage != "" {
					return
				}
				entries, err := os.ReadDir(parent)
				if err != nil {
					t.Fatal(err)
				}
				for _, entry := range entries {
					if strings.HasPrefix(entry.Name(), ".authoring-") {
						stage = filepath.Join(parent, entry.Name())
						write(t, stage, "preserve", "retained fixture")
						if canceled {
							cancel()
						}
						return
					}
				}
			}}
			out.Reset()
			args := []string{"init", filepath.Join(parent, "demo"), "--name=demo", "--template=skill", "--format=json"}
			builder := authoringcli.NewPluginKitRoot
			if mount {
				args = append([]string{"author"}, args...)
				builder = mounted
			}
			err := a.Execute(fault, args, authoringcli.Streams{Out: &out, Err: io.Discard}, builder)
			cancel()
			e, code := publicDecode(t, rawExecution{out: out.Bytes(), err: err})
			if stage == "" || code != 1 || e.Data.Error == nil || e.Data.Error.Code != "private_cleanup_failed" || e.Data.Committed == canceled || !e.Data.Effects.Attempted || (len(e.Data.Paths) == 0) != canceled {
				t.Fatalf("real cleanup precedence: %s", out.Bytes())
			}
			if body, err := os.ReadFile(filepath.Join(stage, "preserve")); err != nil || string(body) != "retained fixture" {
				t.Fatal("unowned entry removed")
			}
			if bytes.Contains(out.Bytes(), []byte(parent)) || bytes.Contains(out.Bytes(), []byte(a.Projects.Scratch)) {
				t.Fatal("cleanup path disclosed")
			}
		}
	}
}
