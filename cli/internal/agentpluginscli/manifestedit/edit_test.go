package manifestedit

import (
	"bytes"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

// This contract detects lost comments, accidental alias splitting, unrelated
// intent changes, and newline conversion when replacing a selector and targets.
func TestApplyPreservesManifestContract(t *testing.T) {
	for _, newline := range []string{"\n", "\r\n"} {
		t.Run(map[string]string{"\n": "LF", "\r\n": "CRLF"}[newline], func(t *testing.T) {
			input := `# manifest note
schema_version = 1

# dependency note
[dependencies."review.alpha"] # heading note
git = 'acme/tools' # source note
path = 'plugins/review'
ref = 'main' # selector note

[dependencies.other]
git = 'acme/other'
commit = '1111111111111111111111111111111111111111'

[targets] # targets note
# assignment note
codex = [
 "review.alpha", # element note
 # array note
 "other",
] # target trailer
claude = [
 # untouched array note
 "other", # untouched element note
]
`
			input = strings.ReplaceAll(input, "\n", newline)
			out, err := Apply([]byte(input), Changes{
				Dependencies: []DependencyEdit{{Alias: "review.alpha", Value: &Dependency{Git: "acme/tools", Path: "plugins/review", Commit: "2222222222222222222222222222222222222222"}}},
				Targets:      []TargetEdit{{Name: "codex", Aliases: []string{"review.alpha"}}, {Name: "custom.name", Aliases: []string{"other"}}},
			})
			if err != nil {
				t.Fatal(err)
			}
			for _, comment := range []string{"# manifest note", "# dependency note", "# heading note", "# source note", "# selector note", "# targets note", "# assignment note", "# target trailer", "# untouched array note", "# untouched element note"} {
				if !bytes.Contains(out, []byte(comment)) {
					t.Fatalf("lost %s:\n%s", comment, out)
				}
			}
			if newline == "\r\n" && bytes.Contains(bytes.ReplaceAll(out, []byte("\r\n"), nil), []byte("\n")) {
				t.Fatalf("lost CRLF: %q", out)
			}
			if newline == "\n" && bytes.Contains(out, []byte("\r")) {
				t.Fatalf("added CR: %q", out)
			}
			before, after := decode(t, []byte(input)), decode(t, out)
			deps := before["dependencies"].(map[string]any)
			deps["review.alpha"].(map[string]any)["commit"] = "2222222222222222222222222222222222222222"
			delete(deps["review.alpha"].(map[string]any), "ref")
			targets := before["targets"].(map[string]any)
			targets["codex"] = []any{"review.alpha"}
			targets["custom.name"] = []any{"other"}
			if !reflect.DeepEqual(before, after) {
				t.Fatalf("unexpected semantics\nwant %#v\ngot %#v\n%s", before, after, out)
			}
			if strings.Index(string(out), "[dependencies.other]") < strings.Index(string(out), `[dependencies."review.alpha"]`) {
				t.Fatal("table order changed")
			}
		})
	}
}

// Removing sections must not delete adjacent sections or comments, and registry
// replacement must remove stale git fields rather than create mixed sources.
func TestApplyUpsertRemoveAndRegistry(t *testing.T) {
	input := []byte("schema_version = 1\n[dependencies]\n[dependencies.old] # retired heading\n# retired key\ngit = 'old/repo' # retired source\n[dependencies.keep]\ngit = 'keep/repo'\nref = 'main'\n[targets]\ncodex = ['old', 'keep'] # retired target\nclaude = ['keep']\n")
	out, err := Apply(input, Changes{
		Dependencies: []DependencyEdit{
			{Alias: "old"},
			{Alias: "keep", Value: &Dependency{Registry: "uap", Distribution: "acme/review", Release: "7"}},
			{Alias: "new.quoted", Value: &Dependency{Git: "acme/new", Ref: "branch\nwith\t\"quote\\é\x01"}},
		},
		Targets: []TargetEdit{{Name: "codex", Remove: true}, {Name: "claude", Aliases: []string{}}, {Name: "missing", Remove: true}},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := decode(t, out)
	want := map[string]any{
		"schema_version": int64(1),
		"dependencies": map[string]any{
			"keep":       map[string]any{"registry": "uap", "distribution": "acme/review", "release": "7"},
			"new.quoted": map[string]any{"git": "acme/new", "ref": "branch\nwith\t\"quote\\é\x01"},
		},
		"targets": map[string]any{"claude": []any{}},
	}
	if !reflect.DeepEqual(want, got) {
		t.Fatalf("want %#v, got %#v\n%s", want, got, out)
	}
	for _, c := range []string{"# retired heading", "# retired key", "# retired source", "# retired target"} {
		if !bytes.Contains(out, []byte(c)) {
			t.Fatalf("lost %s", c)
		}
	}
	// A missing parent table is safely created using the same syntax adapter.
	out, err = Apply([]byte("schema_version = 1\n"), Changes{Dependencies: []DependencyEdit{{Alias: "demo", Value: &Dependency{Git: "acme/demo"}}}, Targets: []TargetEdit{{Name: "codex", Aliases: []string{"demo"}}}})
	if err != nil {
		t.Fatal(err)
	}
	if decode(t, out)["targets"].(map[string]any)["codex"].([]any)[0] != "demo" {
		t.Fatalf("missing assignment: %s", out)
	}
}

// Refuse unsupported table shapes before returning any bytes, preventing a
// caller from accidentally publishing a partially transformed document.
func TestApplyRejectsUnsupportedLayout(t *testing.T) {
	for _, input := range []string{
		`dependencies = {demo = {git = "acme/demo"}}`,
		`dependencies.demo.git = "acme/demo"`,
		"[dependencies]\ndemo = {git = 'acme/demo'}",
		"[dependencies]\ndemo.git = 'acme/demo'",
		"[dependencies.demo.nested]\ngit = 'acme/demo'",
		"[[dependencies.demo]]\ngit = 'acme/demo'",
		`targets = {codex = ["demo"]}`,
		"[targets.codex]\ndemo = true",
		"[targets]\ncodex.extra = ['demo']",
		"[targets]\ncodex = {aliases = ['demo']}",
		"schema_version = 1\r\n[targets]\ncodex = []\r\n",
	} {
		t.Run(input, func(t *testing.T) {
			out, err := Apply([]byte(input), Changes{Dependencies: []DependencyEdit{{Alias: "demo", Value: &Dependency{Git: "acme/new"}}}})
			if !errors.Is(err, ErrUnsupportedLayout) || out != nil {
				t.Fatalf("got %q, %v", out, err)
			}
		})
	}
}

func decode(t *testing.T, body []byte) map[string]any {
	t.Helper()
	var out map[string]any
	if err := toml.Unmarshal(body, &out); err != nil {
		t.Fatalf("strict parse: %v\n%s", err, body)
	}
	return out
}

// Canonical keys must remain single keys during selector changes and removal;
// splitting a slash/dot key would silently change the dependency graph.
func TestApplyCanonicalKeysPreserveIntentCommentsAndCRLF(t *testing.T) {
	input := []byte("# environment\r\nschema_version = 1\r\n[dependencies]\r\n# canonical source\r\n\"acme/review.alpha\" = '^1.0' # selected version\r\n\"acme/keep.beta\" = '2.0.0' # retained source\r\n[targets]\r\ncodex = ['acme/review.alpha', 'acme/keep.beta'] # assignments\r\n")
	out, err := Apply(input, Changes{Dependencies: []DependencyEdit{{Alias: "acme/review.alpha", Value: &Dependency{Version: "^2.0"}}}})
	if err != nil {
		t.Fatal(err)
	}
	want := decode(t, input)
	want["dependencies"].(map[string]any)["acme/review.alpha"] = "^2.0"
	if !reflect.DeepEqual(want, decode(t, out)) {
		t.Fatalf("canonical update changed intent: %s", out)
	}
	out, err = Apply(out, Changes{Dependencies: []DependencyEdit{{Alias: "acme/review.alpha", Value: &Dependency{Registry: "uap", Distribution: "acme/review.alpha", Release: "7"}}}})
	if err != nil {
		t.Fatal(err)
	}
	want["dependencies"].(map[string]any)["acme/review.alpha"] = map[string]any{"registry": "uap", "distribution": "acme/review.alpha", "release": "7"}
	if !reflect.DeepEqual(want, decode(t, out)) {
		t.Fatalf("canonical table conversion changed intent: %s", out)
	}
	out, err = Apply(out, Changes{Dependencies: []DependencyEdit{{Alias: "acme/review.alpha"}}, Targets: []TargetEdit{{Name: "codex", Aliases: []string{"acme/keep.beta"}}}})
	if err != nil {
		t.Fatal(err)
	}
	delete(want["dependencies"].(map[string]any), "acme/review.alpha")
	want["targets"].(map[string]any)["codex"] = []any{"acme/keep.beta"}
	if !reflect.DeepEqual(want, decode(t, out)) {
		t.Fatalf("canonical removal changed unrelated intent: %s", out)
	}
	for _, comment := range []string{"# environment", "# canonical source", "# selected version", "# retained source", "# assignments"} {
		if !bytes.Contains(out, []byte(comment)) {
			t.Fatalf("lost %s: %s", comment, out)
		}
	}
	if bytes.Contains(bytes.ReplaceAll(out, []byte("\r\n"), nil), []byte("\n")) {
		t.Fatalf("lost CRLF: %q", out)
	}
}

// Replacing website intent must remove stale transport fields in both directions.
func TestWebsiteSkillReplacement(t *testing.T) {
	input := []byte("schema_version=1\n[dependencies.pdf]\ngit='acme/tools'\nref='main'\n[targets]\ncodex=['pdf']\n")
	out, err := Apply(input, Changes{Dependencies: []DependencyEdit{{Alias: "pdf", Value: &Dependency{Website: "https://publisher.example", Kind: "skill", Skill: "pdf"}}}})
	if err != nil {
		t.Fatal(err)
	}
	deps := decode(t, out)["dependencies"].(map[string]any)["pdf"].(map[string]any)
	if deps["website"] != "https://publisher.example" || deps["kind"] != "skill" || deps["skill"] != "pdf" || deps["git"] != nil || deps["ref"] != nil {
		t.Fatal(deps)
	}
	out, err = Apply(out, Changes{Dependencies: []DependencyEdit{{Alias: "pdf", Value: &Dependency{Git: "acme/tools", Ref: "main"}}}})
	if err != nil {
		t.Fatal(err)
	}
	deps = decode(t, out)["dependencies"].(map[string]any)["pdf"].(map[string]any)
	if deps["website"] != nil || deps["kind"] != nil || deps["skill"] != nil || deps["git"] != "acme/tools" {
		t.Fatal(deps)
	}
}
