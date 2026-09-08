package conformance

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/specregistry"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func decoder(t *testing.T) Decoder {
	t.Helper()
	r, e := specregistry.New()
	if e != nil {
		t.Fatal(e)
	}
	return Decoder{Registry: r}
}
func minimal() Input {
	return Input{Plugin: NewDocument("plugin.json", Present, []byte(`{"$schema":"`+domain.PluginSchemaV1+`","name":"good"}`)), MCP: NewDocument("mcp.json", Absent, nil), SkillsRoot: Absent, Coverage: Coverage{Filesystem: Pass, Skills: Pass}}
}
func mcpInput(body string) Input {
	i := minimal()
	i.MCP = NewDocument("mcp.json", Present, []byte(body))
	return i
}
func mcpBody(servers string) string {
	return `{"$schema":"` + domain.MCPSchemaV1 + `","mcpServers":` + servers + `}`
}
func skillInput(name, extra string) SkillInput {
	return SkillInput{name, NewDocument("skills/"+name+"/SKILL.md", Present, []byte("---\nname: "+name+"\ndescription: Works offline\n"+extra+"---\n"))}
}
func hasFinding(f Facts, code string, layer PolicyLayer, b domain.FailureBoundary) bool {
	for _, v := range f.Findings {
		if v.Code == code && v.Layer == layer && v.Boundary == b {
			return true
		}
	}
	return false
}

func TestCoreSchemaFacts(t *testing.T) {
	d := decoder(t)
	tests := []struct {
		name, fields string
		out          Outcome
		typed        bool
	}{
		{"minimal", `"name":"good"`, Pass, true},
		{"reserved-physical", `"name":"con"`, Pass, true},
		{"empty-version", `"name":"good","version":""`, Pass, true},
		{"arbitrary-version", `"name":"good","version":"not semver"`, Pass, true},
		{"type-only-metadata", `"name":"good","homepage":"x","repository":"?","license":"whatever","author":{"url":"x","email":"x"}`, Pass, true},
		{"name-max", `"name":"` + strings.Repeat("a", 64) + `"`, Pass, true},
		{"name-over", `"name":"` + strings.Repeat("a", 65) + `"`, Fail, false},
		{"name-missing", `"version":"1"`, Fail, false},
		{"name-empty", `"name":""`, Fail, false},
		{"uppercase", `"name":"Good"`, Fail, false},
		{"double-hyphen", `"name":"g--d"`, Fail, false},
		{"double-dot", `"name":"g..d"`, Fail, false},
		{"unknown", `"name":"good","secret":{"n":99999999999999999999999999}`, Fail, true},
		{"opaque-namespace", `"name":"good","extensions":{"anything":{"n":99999999999999999999999999}}`, Pass, true},
		{"extensions-null", `"name":"good","extensions":null`, Fail, true},
		{"extensions-array", `"name":"good","extensions":[]`, Fail, true},
		{"extensions-scalar", `"name":"good","extensions":"secret"`, Fail, true},
		{"extension-member", `"name":"good","extensions":{"x":2}`, Fail, false},
		{"author-unknown", `"name":"good","author":{"Name":"secret"}`, Fail, false},
	}
	for _, field := range []string{"version", "description", "homepage", "repository", "license", "author", "keywords"} {
		tests = append(tests, struct {
			name, fields string
			out          Outcome
			typed        bool
		}{"wrong-type-" + field, `"name":"good","` + field + `":5`, Fail, false})
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			i := minimal()
			i.Plugin = NewDocument("plugin.json", Present, []byte(`{"$schema":"`+domain.PluginSchemaV1+`",`+tc.fields+`}`))
			f, e := d.Decode(context.Background(), i)
			if e != nil || f.Conformance != tc.out || (f.Package != nil) != tc.typed {
				t.Fatalf("outcome=%s package=%v err=%v findings=%+v", f.Conformance, f.Package != nil, e, f.Findings)
			}
		})
	}
}
func TestMCPBoundaryMatrix(t *testing.T) {
	d := decoder(t)
	tests := []struct {
		name, body, code string
		out              Outcome
		b                domain.FailureBoundary
		keep             int
	}{
		{"empty", mcpBody(`{}`), "", Pass, domain.BoundaryMCP, 0},
		{"good", mcpBody(`{"good":{"type":"stdio","command":"node helper"}}`), "", Pass, domain.BoundaryMCP, 1},
		{"malformed", `{`, "document_json_invalid", Fail, domain.BoundaryMCP, 0},
		{"missing-schema", `{"mcpServers":{}}`, "mcp_schema_missing", Fail, domain.BoundaryMCP, 0},
		{"mismatch", `{"$schema":"https://agent-plugins.org/schemas/2.0.0/mcp.schema.json","mcpServers":{}}`, "mcp_schema_mismatch", Fail, domain.BoundaryMCP, 0},
		{"same-version-unsupported", `{"$schema":"https://wrong.test/schemas/1.0.0/mcp.schema.json","mcpServers":{}}`, "mcp_schema_unsupported", NotEvaluated, domain.BoundaryMCP, 0},
		{"servers-missing", `{"$schema":"` + domain.MCPSchemaV1 + `"}`, "mcp_servers_missing", Fail, domain.BoundaryMCP, 0},
		{"servers-null", mcpBody(`null`), "mcp_servers_invalid", Fail, domain.BoundaryMCP, 0},
		{"servers-array", mcpBody(`[]`), "mcp_servers_invalid", Fail, domain.BoundaryMCP, 0},
		{"unknown-root", `{"$schema":"` + domain.MCPSchemaV1 + `","mcpServers":{},"other":1}`, "mcp_schema_invalid", Fail, domain.BoundaryMCP, 0},
		{"server-type", mcpBody(`{"bad":{"command":"node"},"good":{"type":"stdio","command":"node"}}`), "server_schema", Fail, domain.BoundaryMCPServer, 1},
		{"server-scalar", mcpBody(`{"bad":5,"good":{"type":"stdio","command":"node"}}`), "server_json", Fail, domain.BoundaryMCPServer, 1},
		{"empty-name", mcpBody(`{"":{"type":"stdio","command":"node"}}`), "", Pass, domain.BoundaryMCPServer, 1},
		{"empty-name-invalid", mcpBody(`{"":{"type":"sse","url":"http://remote.test"}}`), "remote_url_tls", Fail, domain.BoundaryMCPServer, 0},
		{"duplicate-server-member", mcpBody(`{"bad":{"type":"stdio","command":"a","command":"b"},"good":{"type":"stdio","command":"node"}}`), "json_duplicate_key", NotEvaluated, domain.BoundaryMCPServer, 1},
		{"duplicate-server-name", mcpBody(`{"bad":{},"bad":{}}`), "json_duplicate_key", NotEvaluated, domain.BoundaryMCP, 0},
		{"duplicate-root", `{"$schema":"` + domain.MCPSchemaV1 + `","mcpServers":{},"mcpServers":{}}`, "json_duplicate_key", NotEvaluated, domain.BoundaryMCP, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			i := mcpInput(tc.body)
			i.SkillsRoot = Present
			i.Skills = []SkillInput{skillInput("good", "")}
			f, e := d.Decode(context.Background(), i)
			if e != nil || f.Conformance != tc.out || f.Package == nil || len(f.Package.MCP.Servers) != tc.keep || len(f.Package.Skills) != 1 {
				t.Fatalf("out=%s keep=%v err=%v findings=%+v", f.Conformance, f.Package, e, f.Findings)
			}
			if tc.code != "" {
				found := false
				for _, v := range f.Findings {
					if v.Code == tc.code && v.Boundary == tc.b {
						found = true
					}
				}
				if !found {
					t.Fatalf("missing %s/%s: %+v", tc.code, tc.b, f.Findings)
				}
			}
		})
	}
}
func TestRemoteSemanticProfile(t *testing.T) {
	d := decoder(t)
	for _, tc := range []struct {
		url   string
		valid bool
	}{
		{"https://remote.test/mcp", true}, {"HTTPS://remote.test", true}, {"http://localhost/x", true}, {"http://LOCALHOST/x", true}, {"http://127.0.0.1", true}, {"http://[::1]", true}, {"http://[::ffff:127.0.0.1]", true},
		{"http://[::1%25zone]", false}, {"http://remote.test", false}, {"/relative", false}, {"ftp://remote.test", false}, {"https:opaque", false}, {"https:///path", false}, {"https://user:secret@remote.test", false}, {"https://remote.test/#", false}, {"https://remote.test/#fragment", false},
	} {
		t.Run(tc.url, func(t *testing.T) {
			encoded, _ := json.Marshal(tc.url)
			f, e := d.Decode(context.Background(), mcpInput(mcpBody(`{"s":{"type":"sse","url":`+string(encoded)+`}}`)))
			if e != nil || (f.Conformance == Pass) != tc.valid {
				t.Fatalf("%s %v %+v", f.Conformance, e, f.Findings)
			}
		})
	}
	for _, tc := range []struct {
		headers string
		valid   bool
	}{
		{`{"X-Tenant":"public"}`, true}, {`{"X-Tab":"one\ttwo"}`, true}, {`{"Bad Name":"secret"}`, false}, {`{"X-Test":"a\rb"}`, false}, {`{"X-Test":"a\u007fb"}`, false}, {`{"X-Test":"a","x-test":"b"}`, false},
	} {
		t.Run(tc.headers, func(t *testing.T) {
			f, e := d.Decode(context.Background(), mcpInput(mcpBody(`{"s":{"type":"sse","url":"https://remote.test","headers":`+tc.headers+`}}`)))
			if e != nil || (f.Conformance == Pass) != tc.valid {
				t.Fatalf("%s %v %+v", f.Conformance, e, f.Findings)
			}
		})
	}
}
func TestPinnedSkillProfile(t *testing.T) {
	d := decoder(t)
	tests := []struct {
		name, extra, code string
		out               Outcome
	}{
		{"good", "", "", Pass}, {"unknown", "future: {anything: yes}\n", "", Pass},
		{"opaque-key-types", "future: {1: {2: x}}\n", "", Pass},
		{"root-merge", "future: &defaults {license: MIT, other: {1: x}}\n<<: *defaults\n", "", Pass}, {"metadata", "metadata: {version: '1'}\n", "", Pass},
		{"numeric-metadata", "metadata: {version: 1}\n", "skill_metadata_type", Fail}, {"nested-metadata", "metadata: {version: {nested: x}}\n", "skill_metadata_type", Fail},
		{"compat-empty", "compatibility: ''\n", "skill_compatibility_length", Fail}, {"compat-max", "compatibility: '" + strings.Repeat("a", 500) + "'\n", "", Pass}, {"compat-over", "compatibility: '" + strings.Repeat("a", 501) + "'\n", "skill_compatibility_length", Fail},
		{"allowed", "allowed-tools: 'Bash(git:*) Read'\n", "", Pass}, {"allowed-type", "allowed-tools: [Read]\n", "skill_optional_type", Fail},
		{"alias", "license: &license MIT\nmetadata: {license: *license}\n", "", Pass},
		{"merge", "future: &meta {license: MIT}\nmetadata: {<<: *meta}\n", "", Pass},
		{"duplicate", "license: MIT\nlicense: Apache\n", "yaml_duplicate_key", NotEvaluated},
		{"cycle", "future: &cycle {child: *cycle}\n", "yaml_alias_cycle", NotEvaluated},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			i := minimal()
			i.SkillsRoot = Present
			i.Skills = []SkillInput{skillInput("good", tc.extra)}
			f, e := d.Decode(context.Background(), i)
			if e != nil || f.Conformance != tc.out {
				t.Fatalf("%s %v %+v", f.Conformance, e, f.Findings)
			}
			if tc.code != "" {
				found := false
				for _, v := range f.Findings {
					found = found || v.Code == tc.code
				}
				if !found {
					t.Fatal(f.Findings)
				}
			}
		})
	}
	for _, tc := range []struct {
		name  string
		valid bool
	}{{"a", true}, {"résumé", true}, {"資料", false}, {strings.Repeat("a", 64), true}, {strings.Repeat("a", 65), false}, {"a--b", false}, {"-a", false}, {"a-", false}, {"Upper", false}, {"a.b", false}} {
		t.Run(tc.name, func(t *testing.T) {
			f, e := d.DecodeSkill(context.Background(), skillInput(tc.name, ""))
			if e != nil || (f.Coverage.Skills == Pass) != tc.valid {
				t.Fatalf("%s %v %+v", f.Coverage.Skills, e, f.Findings)
			}
		})
	}
	for _, count := range []int{1, 1024, 1025} {
		body := "---\nname: good\ndescription: '" + strings.Repeat("é", count) + "'\n---"
		f, e := d.DecodeSkill(context.Background(), SkillInput{"good", NewDocument("skills/good/SKILL.md", Present, []byte(body))})
		if e != nil || (f.Coverage.Skills == Pass) != (count <= 1024) {
			t.Fatal(f, e)
		}
	}
	for _, body := range []string{"---\nname: good\ndescription: OK\n---", "---\r\nname: good\r\ndescription: OK\r\n---\r\n"} {
		f, e := d.DecodeSkill(context.Background(), SkillInput{"good", NewDocument("skills/good/SKILL.md", Present, []byte(body))})
		if e != nil || f.Coverage.Skills != Pass {
			t.Fatal(f, e)
		}
	}
}
func TestObservationCoverageAndFormat(t *testing.T) {
	d := decoder(t)
	for _, state := range []InputState{Absent, Unreadable, WrongKind, Blocked, ""} {
		t.Run(string(state), func(t *testing.T) {
			i := minimal()
			i.MCP = NewDocument("mcp.json", state, nil)
			f, e := d.Decode(context.Background(), i)
			want := NotEvaluated
			if state == Absent {
				want = Pass
			}
			if state == WrongKind {
				want = Fail
			}
			if e != nil || f.Conformance != want {
				t.Fatalf("%s %v %+v", f.Conformance, e, f.Findings)
			}
		})
	}
	for _, path := range []string{".codex-plugin/plugin.json", "plugin/plugin.yaml", "nested/plugin.json", "../plugin.json", "/plugin.json"} {
		i := minimal()
		i.Plugin.Path = path
		f, e := d.Decode(context.Background(), i)
		if e != nil || f.Package != nil || f.Conformance != NotEvaluated {
			t.Fatal(f, e)
		}
	}
	i := minimal()
	i.Coverage.Filesystem = ""
	f, e := d.Decode(context.Background(), i)
	if e != nil || f.Conformance != NotEvaluated || f.Coverage.Complete {
		t.Fatal(f, e)
	}
	i = minimal()
	i.SkillsRoot = Present
	i.Coverage.Skills = ""
	f, e = d.Decode(context.Background(), i)
	if e != nil || f.Conformance != NotEvaluated {
		t.Fatal(f, e)
	}
	i = minimal()
	i.Plugin = NewDocument("plugin.json", Present, []byte(`{"$schema":"https://future.test/schema","name":"good"}`))
	f, e = d.Decode(context.Background(), i)
	if e != nil || f.Conformance != NotEvaluated || !hasFinding(f, "plugin_schema_unsupported", InstallerPolicy, domain.BoundaryPlugin) {
		t.Fatal(f, e)
	}
}
func TestStdioPathFactsRemainSeparate(t *testing.T) {
	d := decoder(t)
	for _, command := range []string{"node", "node helper", "${PLUGIN_ROOT}"} {
		q, _ := json.Marshal(command)
		f, e := d.Decode(context.Background(), mcpInput(mcpBody(`{"s":{"type":"stdio","command":`+string(q)+`}}`)))
		if e != nil || f.Conformance != Pass {
			t.Fatal(f, e)
		}
	}
	for _, command := range []string{"./CON.txt", "./bin/../server", "./missing"} {
		for _, state := range []Outcome{Pass, Fail, NotEvaluated} {
			q, _ := json.Marshal(command)
			i := mcpInput(mcpBody(`{"s":{"type":"stdio","command":` + string(q) + `}}`))
			i.Paths = []PathObservation{{"s", "command", state}}
			f, e := d.Decode(context.Background(), i)
			if e != nil || f.Conformance != state {
				t.Fatalf("%s %s: %+v %v", command, state, f, e)
			}
		}
	}
	for _, cwd := range []string{"./", "./CON", "./a/../b", "${PLUGIN_ROOT}/x", "${PLUGIN_DATA}/x"} {
		q, _ := json.Marshal(cwd)
		i := mcpInput(mcpBody(`{"s":{"type":"stdio","command":"node","cwd":` + string(q) + `}}`))
		i.Paths = []PathObservation{{"s", "cwd", Pass}}
		f, e := d.Decode(context.Background(), i)
		if e != nil || f.Conformance != Pass {
			t.Fatal(f, e)
		}
	}
}
func TestSafeDeterministicReportAndCancellation(t *testing.T) {
	d := decoder(t)
	i := mcpInput(mcpBody(`{"super-secret-item":{"type":"sse","url":"https://user:super-secret-value@remote.test","headers":{"Authorization":"super-secret-header"}},"good":{"type":"stdio","command":"node","env":{"PRIVATE":"super-secret-env"}}}`))
	i.Plugin = NewDocument("plugin.json", Present, []byte(`{"$schema":"`+domain.PluginSchemaV1+`","name":"good","extensions":{"opaque":{"key":"super-secret-extension"}},"super-secret-field":true}`))
	var expected string
	for n := 0; n < 20; n++ {
		f, e := d.Decode(context.Background(), i)
		if e != nil {
			t.Fatal(e)
		}
		body, e := json.Marshal(f)
		if e != nil || strings.Contains(string(body), "super-secret") {
			t.Fatalf("unsafe report %s %v", body, e)
		}
		if n > 0 && string(body) != expected {
			t.Fatal("nondeterministic report")
		}
		expected = string(body)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, e := d.Decode(ctx, minimal())
	if !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}
func TestJSONStructuralMatrix(t *testing.T) {
	d := decoder(t)
	for _, body := range []string{"", "null", "[]", "5", "{", "{} {}", `{"name":}`, `{"a":1,}`} {
		t.Run(fmt.Sprintf("%q", body), func(t *testing.T) {
			i := minimal()
			i.Plugin = NewDocument("plugin.json", Present, []byte(body))
			f, e := d.Decode(context.Background(), i)
			if e != nil || f.Conformance != Fail {
				t.Fatal(f, e)
			}
		})
	}
	for _, body := range []string{`{"$schema":"` + domain.PluginSchemaV1 + `","name":"a","\u006eame":"b"}`, `{"$schema":"` + domain.PluginSchemaV1 + `","name":"good","author":{"name":"a","name":"b"}}`} {
		i := minimal()
		i.Plugin = NewDocument("plugin.json", Present, []byte(body))
		f, e := d.Decode(context.Background(), i)
		if e != nil || f.Conformance == Pass || !hasFinding(f, "json_duplicate_key", HostSafety, domain.BoundaryPlugin) {
			t.Fatal(f, e)
		}
	}
	i := minimal()
	i.Plugin = NewDocument("plugin.json", Present, append(i.Plugin.Bytes(), []byte(" \n\t")...))
	f, e := d.Decode(context.Background(), i)
	if e != nil || f.Conformance != Pass {
		t.Fatal(f, e)
	}
}

func TestInvalidCapturedComponentInventory(t *testing.T) {
	d := decoder(t)
	for _, state := range []InputState{Present, WrongKind} {
		i := minimal()
		i.MCP = NewDocument("mcp.json", state, []byte("{"))
		i.SkillsRoot = Present
		i.Skills = []SkillInput{{"skills", NewDocument("skills/skills/SKILL.md", Unreadable, nil)}, skillInput("good", "")}
		f, e := d.Decode(context.Background(), i)
		if e != nil || f.Conformance != Fail || !f.Package.Inventory.MCPPresent || f.Package.Inventory.MCPEnabled || len(f.Package.Inventory.InvalidSkills) != 1 || f.Package.Inventory.InvalidSkills[0] != "skills" || f.Package.Inventory.InvalidSkillsRoot || len(f.Package.Skills) != 1 {
			t.Fatalf("%+v %v", f, e)
		}
	}
}

func TestSkillFieldAndFramingOrigins(t *testing.T) {
	d := decoder(t)
	for _, tc := range []struct{ body, code string }{
		{"---\ndescription: Works\n---", "skill_required_field"},
		{"---\nname: good\n---", "skill_required_field"},
		{"---\nname: 5\ndescription: Works\n---", "skill_required_type"},
		{"---\nname: ''\ndescription: Works\n---", "skill_required_type"},
		{"---\nname: good\ndescription: ''\n---", "skill_required_type"},
		{"---\nname: good\ndescription: 5\n---", "skill_required_type"},
		{"---\nname: good\ndescription: Works\nlicense: 5\n---", "skill_optional_type"},
		{"---\nname: good\ndescription: Works\ncompatibility: []\n---", "skill_optional_type"},
		{"---\nname: good\ndescription: Works\nmetadata: []\n---", "skill_metadata_type"},
		{"---\nname: good\ndescription: Works\nmetadata: {5: x}\n---", "skill_metadata_type"},
		{"---\nnull\n---", "skill_frontmatter_type"},
		{"---\n[1,2]\n---", "skill_frontmatter_type"},
		{"---\nname: [\n---", "skill_yaml_invalid"},
		{"---\nname: good\n", "skill_frontmatter_close"},
		{"name: good\n", "skill_frontmatter_open"},
	} {
		t.Run(tc.code+fmt.Sprintf("/%x", len(tc.body)), func(t *testing.T) {
			f, e := d.DecodeSkill(context.Background(), SkillInput{"good", NewDocument("skills/good/SKILL.md", Present, []byte(tc.body))})
			if e != nil || f.Coverage.Skills != Fail || !hasFinding(f, tc.code, Normative, domain.BoundarySkill) {
				t.Fatalf("%s: %+v %v", tc.code, f, e)
			}
		})
	}
	body := append([]byte{0xef, 0xbb, 0xbf}, skillInput("good", "").Document.Bytes()...)
	if _, e := ParseInstallerSkill("good", body); e == nil {
		t.Fatal("installer BOM behavior changed")
	}
	f, e := d.DecodeSkill(context.Background(), SkillInput{"good", NewDocument("skills/good/SKILL.md", Present, body)})
	if e != nil || f.Coverage.Skills != Pass || string(f.Package.Skills["good"].Raw) != string(body) {
		t.Fatal(f, e)
	}
}

func TestPartialDiscoveryRetainsObservedFacts(t *testing.T) {
	d := decoder(t)
	for _, state := range []InputState{Absent, Unreadable, Blocked} {
		for _, invalid := range []bool{false, true} {
			i := minimal()
			i.SkillsRoot = state
			i.Skills = []SkillInput{skillInput("good", "")}
			if invalid {
				i.Skills = append(i.Skills, SkillInput{"bad", NewDocument("skills/bad/SKILL.md", Present, []byte("not frontmatter"))})
			}
			f, e := d.Decode(context.Background(), i)
			want := NotEvaluated
			if invalid {
				want = Fail
			}
			if e != nil || f.Conformance != want || len(f.Package.Skills) != 1 || f.Coverage.Complete {
				t.Fatalf("%s invalid=%v: %+v %v", state, invalid, f, e)
			}
			if invalid && !hasFinding(f, "skill_frontmatter_open", Normative, domain.BoundarySkill) {
				t.Fatal("observed normative failure erased")
			}
		}
	}
	i := minimal()
	i.SkillsRoot = Present
	i.Coverage.Skills = Fail
	f, e := d.Decode(context.Background(), i)
	if e != nil || f.Conformance != NotEvaluated {
		t.Fatal("failed discovery is not a normative failure", f, e)
	}
}
