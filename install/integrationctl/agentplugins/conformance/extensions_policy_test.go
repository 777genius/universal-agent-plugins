package conformance_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/loader"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/specregistry"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/conformance"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// These expectations intentionally distinguish original author validity from
// published 1.0 loading exceptions. No namespace interpreter is implemented here.
func TestExtensionsAuthorAndLoadingPolicy(t *testing.T) {
	registry, err := specregistry.New()
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, fields, report string
		authorValid, load    bool
	}{
		{"absent", "", "", true, true},
		{"empty", `,"extensions":{}`, "", true, true},
		{"container-null", `,"extensions":null`, "plugin_extensions_ignored", false, true},
		{"container-string", `,"extensions":"opaque"`, "plugin_extensions_ignored", false, true},
		{"container-array", `,"extensions":[]`, "plugin_extensions_ignored", false, true},
		{"unknown-object", `,"extensions":{"com.example.future":{"nested":[null,1]}}`, "", true, true},
		{"unknown-scalar-disputed", `,"extensions":{"com.example.future":"opaque"}`, "", false, false},
		{"unknown-null-disputed", `,"extensions":{"com.example.future":null}`, "", false, false},
		{"unknown-array-disputed", `,"extensions":{"com.example.future":[]}`, "", false, false},
		{"unknown-top-level", `,"future":{"opaque":true}`, "plugin_unknown_field", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := []byte(`{"$schema":"` + domain.PluginSchemaV1 + `","name":"extension-policy"` + tc.fields + `}`)
			facts, err := (conformance.Decoder{Registry: registry}).DecodePlugin(context.Background(), conformance.NewDocument("plugin.json", conformance.Present, body))
			if err != nil {
				t.Fatal(err)
			}
			wantCoverage := conformance.Fail
			if tc.authorValid {
				wantCoverage = conformance.Pass
			}
			if facts.Coverage.Plugin != wantCoverage {
				t.Fatalf("author coverage=%s, want=%s", facts.Coverage.Plugin, wantCoverage)
			}
			manifest, diagnostics, _, err := (conformance.InstallerDecoder{Registry: registry}).Plugin(body)
			if (err == nil) != tc.load {
				t.Fatalf("load error=%v, expected loaded=%v", err, tc.load)
			}
			if !tc.load {
				return
			}
			if string(manifest.Raw) != string(body) {
				t.Fatal("original manifest bytes changed")
			}
			if tc.report != "" {
				found := false
				for _, d := range diagnostics {
					if d.Code == tc.report {
						found = true
					}
				}
				if !found {
					t.Fatalf("missing %s report: %+v", tc.report, diagnostics)
				}
			}
			// Exercise discovery after tolerant manifest loading, without client launch.
			root := t.TempDir()
			files := map[string]string{
				"plugin.json":             string(body),
				"skills/healthy/SKILL.md": "---\nname: healthy\ndescription: Healthy sibling\n---\nInstructions.\n",
				"mcp.json":                `{"$schema":"` + domain.MCPSchemaV1 + `","mcpServers":{"healthy":{"type":"streamable-http","url":"https://example.test/mcp"}}}`,
			}
			for name, value := range files {
				path := filepath.Join(root, filepath.FromSlash(name))
				if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(value), 0600); err != nil {
					t.Fatal(err)
				}
			}
			pkg, err := (loader.Loader{Registry: registry}).Load(context.Background(), domain.LoadInput{SnapshotRoot: root})
			if err != nil {
				t.Fatal(err)
			}
			if len(pkg.Skills) != 1 || len(pkg.MCP.Servers) != 1 {
				t.Fatalf("healthy components lost: skills=%d mcp=%d", len(pkg.Skills), len(pkg.MCP.Servers))
			}
		})
	}
}
