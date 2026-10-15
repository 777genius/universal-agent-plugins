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
