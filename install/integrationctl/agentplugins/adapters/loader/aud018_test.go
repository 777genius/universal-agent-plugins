package loader

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"path/filepath"
	"testing"
)

// AUD-018 is an explicit correction, not an extraction invariant. The local
// struct demonstrates the former Go decoder behavior; the facade assertions
// establish the corrected exact-key authority in both source member orders.
func TestAUD018ExactCoreKeysAndBeforeAfter(t *testing.T) {
	canonical := `"$schema":"` + domain.PluginSchemaV1 + `","name":"good","version":"original","description":"description","author":{"name":"author"},"homepage":"home","repository":"repo","license":"license","keywords":["key"]`
	for _, tc := range []struct {
		unknown  string
		oldName  string
		oldError bool
	}{
		{`"Name":"shadow"`, "shadow", false}, {`"Name":5`, "good", true},
		{`"$SCHEMA":"shadow-schema"`, "good", false}, {`"Version":5`, "good", true},
		{`"Author":5`, "good", true}, {`"Description":[]`, "good", true},
		{`"Homepage":5`, "good", true}, {`"Repository":5`, "good", true},
		{`"License":5`, "good", true}, {`"Keywords":5`, "good", true},
	} {
		for _, before := range []bool{false, true} {
			t.Run(tc.unknown+map[bool]string{true: "/before", false: "/after"}[before], func(t *testing.T) {
				body := "{" + canonical + "," + tc.unknown + "}"
				if before {
					body = "{" + tc.unknown + "," + canonical + "}"
				}
				var previous struct {
					Schema      string         `json:"$schema"`
					Name        string         `json:"name"`
					Version     string         `json:"version"`
					Author      *domain.Author `json:"author"`
					Description string         `json:"description"`
					Homepage    string         `json:"homepage"`
					Repository  string         `json:"repository"`
					License     string         `json:"license"`
					Keywords    []string       `json:"keywords"`
				}
				oldErr := json.Unmarshal([]byte(body), &previous)
				if (oldErr != nil) != tc.oldError {
					t.Fatalf("baseline reproduction error=%v", oldErr)
				}
				if !before && previous.Name != tc.oldName {
					t.Fatalf("baseline name=%q", previous.Name)
				}
				root := t.TempDir()
				writeLoaderFile(t, filepath.Join(root, "plugin.json"), body)
				writeLoaderFile(t, filepath.Join(root, "mcp.json"), `{"$schema":"`+domain.MCPSchemaV1+`","mcpServers":{"good":{"type":"stdio","command":"node"}}}`)
				pkg, e := testLoader(t).Load(context.Background(), domain.LoadInput{SnapshotRoot: root})
				if e != nil {
					t.Fatal(e)
				}
				m := pkg.Manifest
				if m.Name != "good" || m.SchemaURI != domain.PluginSchemaV1 || m.Version != "original" || m.Author == nil || m.Author.Name != "author" || m.Description != "description" || m.Homepage != "home" || m.Repository != "repo" || m.License != "license" || len(m.Keywords) != 1 || m.Keywords[0] != "key" {
					t.Fatalf("canonical values changed: %+v", m)
				}
				if string(m.Raw) != body || len(m.Unknown) != 1 || !hasDiagnostic(pkg.Diagnostics, "plugin_unknown_field") || !pkg.MCP.Enabled {
					t.Fatalf("opacity or MCP precedence changed: %+v", pkg)
				}
			})
		}
	}
	// A folded name cannot repair an absent canonical name.
	root := t.TempDir()
	writeLoaderFile(t, filepath.Join(root, "plugin.json"), `{"$schema":"`+domain.PluginSchemaV1+`","Name":"repair"}`)
	_, e := testLoader(t).Load(context.Background(), domain.LoadInput{SnapshotRoot: root})
	var load *domain.LoadError
	if !errors.As(e, &load) || load.Diagnostic.Code != "plugin_schema_invalid" {
		t.Fatal(e)
	}
}
