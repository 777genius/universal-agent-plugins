package loader

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/conformance"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestNativeCaseVariantCompatibilityUnchanged(t *testing.T) {
	for _, tc := range []struct{ body, name, code string }{
		{`{"name":"good","Name":"shadow"}`, "shadow", ""},
		{`{"Name":"shadow","name":"good"}`, "good", ""},
		{`{"name":"good","Name":5}`, "", "plugin_manifest_decode_failed"},
	} {
		root := t.TempDir()
		writeLoaderFile(t, filepath.Join(root, ".codex-plugin", "plugin.json"), tc.body)
		pkg, e := testOpenAILoader(t).Load(context.Background(), domain.LoadInput{SnapshotRoot: root})
		if tc.code != "" {
			var load *domain.LoadError
			if !errors.As(e, &load) || load.Diagnostic.Code != tc.code {
				t.Fatal(e)
			}
			continue
		}
		if e != nil || pkg.Manifest.Name != tc.name || pkg.FormatID != domain.FormatIDOpenAIPlugin {
			t.Fatal(pkg, e)
		}
	}
}
func TestInstallerFacadeCharacterization(t *testing.T) {
	for _, tc := range []struct {
		name, extra    string
		installerValid bool
		author         conformance.Outcome
	}{
		{"numeric-metadata", "metadata: {count: 5}\n", true, conformance.Fail},
		{"nested-metadata", "metadata: {nested: {value: x}}\n", true, conformance.Fail},
		{"empty-compatibility", "compatibility: ''\n", true, conformance.Fail},
		{"unknown-field", "future: true\n", false, conformance.Pass},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeMinimalPlugin(t, root, "good")
			body := "---\nname: good\ndescription: Works\n" + tc.extra + "---\n"
			writeLoaderFile(t, filepath.Join(root, "skills", "good", "SKILL.md"), body)
			loader := testLoader(t)
			pkg, e := loader.Load(context.Background(), domain.LoadInput{SnapshotRoot: root})
			if e != nil || (len(pkg.Skills) == 1) != tc.installerValid {
				t.Fatal(pkg, e)
			}
			f, e := (conformance.Decoder{Registry: loader.Registry}).DecodeSkill(context.Background(), conformance.SkillInput{Directory: "good", Document: conformance.NewDocument("skills/good/SKILL.md", conformance.Present, []byte(body))})
			if e != nil || f.Coverage.Skills != tc.author {
				t.Fatal(f, e)
			}
		})
	}
	for _, tc := range []struct {
		name, body string
		code       string
		servers    int
	}{
		{"unknown-opaque", `{"$schema":"` + domain.PluginSchemaV1 + `","name":"good","opaque":{"x":1,"x":2}}`, "plugin_unknown_field", 1},
		{"nonobject-extensions", `{"$schema":"` + domain.PluginSchemaV1 + `","name":"good","extensions":null}`, "plugin_extensions_ignored", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeLoaderFile(t, filepath.Join(root, "plugin.json"), tc.body)
			writeLoaderFile(t, filepath.Join(root, "mcp.json"), `{"$schema":"`+domain.MCPSchemaV1+`","mcpServers":{"bad":{"type":"stdio","command":"a","command":"b"},"good":{"type":"stdio","command":"node"}}}`)
			writeLoaderFile(t, filepath.Join(root, "skills", "good", "SKILL.md"), "---\nname: good\ndescription: Works\n---")
			pkg, e := testLoader(t).Load(context.Background(), domain.LoadInput{SnapshotRoot: root})
			if e != nil || !hasDiagnostic(pkg.Diagnostics, tc.code) || len(pkg.MCP.Servers) != tc.servers || len(pkg.Skills) != 1 || len(pkg.MCP.InvalidServer) != 1 || string(pkg.Manifest.Raw) != tc.body {
				t.Fatal(pkg, e)
			}
		})
	}
}

type interfaceRegistry interface {
	conformance.SchemaRegistry
	Digest(string) (string, bool)
}
type recordingRegistry struct {
	interfaceRegistry
	uris []string
}

func (r *recordingRegistry) Validate(uri string, value any) error {
	r.uris = append(r.uris, uri)
	return r.interfaceRegistry.Validate(uri, value)
}
func TestPortableNameDoesNotPreventComponentDecode(t *testing.T) {
	root := t.TempDir()
	writeMinimalPlugin(t, root, "con")
	writeLoaderFile(t, filepath.Join(root, "mcp.json"), "not JSON")
	writeLoaderFile(t, filepath.Join(root, "skills", "bad", "SKILL.md"), "not YAML")
	original := testLoader(t)
	registry := &recordingRegistry{interfaceRegistry: original.Registry}
	pkg, e := (Loader{Registry: registry}).Load(context.Background(), domain.LoadInput{SnapshotRoot: root})
	if e != nil || pkg.Manifest.Name != "con" || len(pkg.Diagnostics) == 0 || len(registry.uris) == 0 || registry.uris[0] != domain.PluginSchemaV1 {
		t.Fatalf("precedence: %v %v", e, registry.uris)
	}
	f, e := (conformance.Decoder{Registry: original.Registry}).DecodePlugin(context.Background(), conformance.NewDocument("plugin.json", conformance.Present, []byte(`{"$schema":"`+domain.PluginSchemaV1+`","name":"con"}`)))
	if e != nil || f.Package == nil || f.Coverage.Plugin != conformance.Pass || f.Package.Manifest.Name != "con" {
		t.Fatal(f, e)
	}
}
func TestInstallerRetainsInvalidUTF8Compatibility(t *testing.T) {
	root := t.TempDir()
	body := `{"$schema":"` + domain.PluginSchemaV1 + `","name":"good","description":"` + string([]byte{0xff}) + `"}`
	writeLoaderFile(t, filepath.Join(root, "plugin.json"), body)
	pkg, e := testLoader(t).Load(context.Background(), domain.LoadInput{SnapshotRoot: root})
	if e != nil || pkg.Manifest.Description != "�" || !strings.Contains(string(pkg.Manifest.Raw), string([]byte{0xff})) {
		t.Fatal(pkg, e)
	}
}
