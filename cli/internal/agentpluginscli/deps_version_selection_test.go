package agentpluginscli

import (
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/manifestedit"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

// A malformed selector must point at its dependency; editing one root must not
// erase a neighboring legacy source or its comments.
func TestVersionSelectionManifestForms(t *testing.T) {
	for _, test := range []struct {
		expression string
		valid      bool
	}{
		{"1.2.3", true}, {"^1.2", true}, {"^0.2.3", true}, {"^0.0", true},
		{"1.2", true}, {"~1.2.3", true}, {">=1.2.3 <2.0.0", true}, {"^1.2 || ^2.0", true}, {"1.2.3-beta", true}, {"1.2.3+build", true}, {"*", true},
		{"", false}, {"v1.2.3", false}, {" >=1.2.3", false},
	} {
		t.Run(test.expression, func(t *testing.T) {
			body := []byte("schema_version = 1\n[dependencies]\n# keep dependency context\ncontext7 = \"" + test.expression + "\" # keep inline\n[dependencies.legacy]\ngit = \"acme/tools\"\nref = \"main\"\n[targets]\ncodex = [\"context7\", \"legacy\"]\n")
			req, err := readDependencyManifestBytes("uap.toml", body)
			if !test.valid {
				if err == nil || !strings.Contains(err.Error(), "uap.toml:4:") {
					t.Fatalf("error = %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			unchanged, err := editEnvironmentIntent(environmentSnapshot{Requirements: req, ManifestBytes: body}, req)
			if err != nil || string(unchanged) != string(body) {
				t.Fatalf("unchanged intent rewrote bytes: %s %v", unchanged, err)
			}
			selector := req.Dependencies["context7"].Registry.Selector
			wantKind := environment.SelectorRange
			if test.expression == "1.2.3" || test.expression == "1.2.3-beta" || test.expression == "1.2.3+build" {
				wantKind = environment.SelectorVersion
			}
			if selector.Kind != wantKind || selector.Value != test.expression {
				t.Fatalf("selector = %+v", selector)
			}
			wanted := environment.Requirements{SchemaVersion: req.SchemaVersion, Dependencies: map[string]environment.Requirement{"legacy": req.Dependencies["legacy"]}, Targets: map[string][]string{}}
			delete(wanted.Dependencies, "context7")
			wanted.Targets["codex"] = []string{"legacy"}
			edited, err := manifestedit.Apply(body, manifestedit.Changes{Dependencies: dependencyIntentEdits(req, wanted), Targets: targetIntentEdits(req, wanted)})
			if err != nil {
				t.Fatal(err)
			}
			parsed, err := readDependencyManifestBytes("uap.toml", edited)
			if err != nil {
				t.Fatal(err)
			}
			if len(parsed.Dependencies) != 1 || parsed.Dependencies["legacy"].Git.Selector.Value != "main" {
				t.Fatalf("edited intent = %+v", parsed)
			}
			for _, comment := range []string{"# keep dependency context", "# keep inline"} {
				if !strings.Contains(string(edited), comment) {
					t.Fatalf("lost %q: %s", comment, edited)
				}
			}
			replace := manifestedit.Changes{Dependencies: []manifestedit.DependencyEdit{{Alias: "context7", Value: &manifestedit.Dependency{Version: test.expression}}}}
			restored, err := manifestedit.Apply(edited, replace)
			if err != nil || !strings.Contains(string(restored), test.expression) {
				t.Fatalf("restore: %s %v", restored, err)
			}
		})
	}
}
