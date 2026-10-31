package dependencymanifest

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

func TestViewsPreserveAuthorityAndIsolation(t *testing.T) {
	body := []byte("schema_version = 1\n[dependencies]\n\"acme/docs\" = \"^2.0\"\n[targets]\nunknown = [\"undeclared\"]\n")
	doc, err := Decode(body, "/package/uap.toml")
	if err != nil {
		t.Fatal(err)
	}
	body[0] = 'x'
	deps, err := doc.Package()
	if err != nil || len(deps) != 1 {
		t.Fatalf("package: %v %v", deps, err)
	}
	if _, err := doc.Root(); err == nil || !strings.Contains(err.Error(), "/package/uap.toml:5:") {
		t.Fatalf("root: %v", err)
	}
	deps["acme/docs"].Registry.Distribution = "changed/value"
	again, err := doc.Package()
	if err != nil || again["acme/docs"].Registry.Distribution != "acme/docs" {
		t.Fatalf("view changed document: %v %v", again, err)
	}
}

func TestCommonSourceGrammar(t *testing.T) {
	body := []byte(`schema_version = 1
[dependencies]
"acme/docs" = "^2.0"
[dependencies.exact]
registry = "uap"
distribution = "acme/exact"
release = 42
[dependencies.tool]
git = "https://github.com/acme/tools.git"
path = "plugins/tool"
ref = "main"
[dependencies.default]
git = "acme/default"
[dependencies.commit]
git = "acme/commit"
commit = "0123456789012345678901234567890123456789"
[targets]
codex = ["acme/docs", "exact", "tool", "default", "commit"]
`)
	doc, err := Decode(body, "uap.toml")
	if err != nil {
		t.Fatal(err)
	}
	root, err := doc.Root()
	if err != nil {
		t.Fatal(err)
	}
	deps, err := doc.Package()
	if err != nil {
		t.Fatal(err)
	}
	if len(root.Dependencies) != 5 || len(deps) != 5 || deps["exact"].Registry.Selector != (environment.Selector{Kind: environment.SelectorRelease, Value: "42"}) || deps["tool"].Git.Repository != "acme/tools" || deps["default"].Git.Selector.Kind != environment.SelectorDefault {
		t.Fatalf("unexpected requirements: %#v", deps)
	}
	root.Targets["codex"][0] = "changed"
	root.Dependencies["tool"].Git.Path = "changed"
	rootAgain, err := doc.Root()
	if err != nil || rootAgain.Targets["codex"][0] != "acme/docs" || rootAgain.Dependencies["tool"].Git.Path != "plugins/tool" {
		t.Fatalf("mutable root: %#v %v", rootAgain, err)
	}
}

func TestRootRequiresAssignmentsAndPackageCanonicalSources(t *testing.T) {
	for _, body := range []string{"schema_version=1\n[dependencies]\ndocs=\"^1.0\"", "schema_version=1\n[dependencies.docs]\nregistry=\"uap\"\ndistribution=\"docs\"\nrelease=1"} {
		doc, err := Decode([]byte(body), "uap.toml")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := doc.Root(); err == nil {
			t.Fatal("unassigned root accepted")
		}
		if _, err := doc.Package(); err == nil {
			t.Fatal("short package source accepted")
		}
	}
	doc, err := Decode([]byte("schema_version = 1"), "uap.toml")
	if err != nil {
		t.Fatal(err)
	}
	if deps, err := doc.Package(); err != nil || len(deps) != 0 {
		t.Fatalf("leaf: %v %v", deps, err)
	}
	if _, err := doc.Root(); err != nil {
		t.Fatalf("empty root: %v", err)
	}
}

func TestDecodeRejectsInvalidManifestWithSourceLocation(t *testing.T) {
	cases := []struct{ name, body, location string }{
		{"empty", "", ":1:"},
		{"missing schema", "[dependencies]", ":1:"},
		{"schema type", "schema_version=\"1\"", ":1:"},
		{"schema version", "schema_version=2", ":1:"},
		{"unknown", "schema_version=1\n[dev-dependencies]\na=\"1.0.0\"", ":2:"},
		{"duplicate", "schema_version=1\nschema_version=1", ":1:"},
		{"dependency type", "schema_version=1\n[dependencies]\na=42", ":3:"},
		{"dependencies type", "schema_version=1\ndependencies=[]", ":2:"},
		{"target type", "schema_version=1\n[targets]\ncodex=[1]", ":3:"},
		{"targets type", "schema_version=1\ntargets=[]", ":2:"},
		{"empty selector", "schema_version=1\n[dependencies]\na=\"\"", ":3:"},
		{"invalid selector", "schema_version=1\n[dependencies]\na=\"latest\"", ":3:"},
		{"unknown field", "schema_version=1\n[dependencies.a]\ngit=\"acme/a\"\nversion=\"1.0\"", ":4:"},
		{"field type", "schema_version=1\n[dependencies.a]\ngit=1", ":3:"},
		{"release type", "schema_version=1\n[dependencies.a]\nregistry=\"uap\"\ndistribution=\"acme/a\"\nrelease=true", ":2:"},
		{"same source", "schema_version=1\n[dependencies]\n\"acme/a\"=\"1.0.0\"\n\"Acme/A\"=\"1.0.0\"", ":3:"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Decode([]byte(tc.body), "/source/uap.toml")
			if err == nil || !strings.Contains(err.Error(), "/source/uap.toml"+tc.location) {
				t.Fatalf("expected source%s, got %v", tc.location, err)
			}
		})
	}
}

func TestLegacyPresenceIsTypedAndLossless(t *testing.T) {
	for _, value := range []string{"null", "{}", "[]", "42", "\"bad\"", "{broken"} {
		raw := json.RawMessage(`{"other":true,"dependencies":` + value + `}`)
		original := string(raw)
		ext := map[string]json.RawMessage{legacyNamespace: raw}
		err := RejectLegacyDependencies(ext)
		if !errors.Is(err, ErrDependencyDeclarationMoved) || !strings.Contains(err.Error(), "uap.toml") {
			t.Fatalf("%s: %v", value, err)
		}
		if string(ext[legacyNamespace]) != original {
			t.Fatal("extension modified")
		}
	}
	for _, raw := range []string{`{}`, `null`, `{"other":{"dependencies":{}}}`} {
		if err := RejectLegacyDependencies(map[string]json.RawMessage{legacyNamespace: json.RawMessage(raw)}); err != nil {
			t.Fatalf("unrelated extension: %v", err)
		}
	}
	if ErrDependencyDeclarationMoved.Error() != "dependency_declaration_moved" {
		t.Fatal("changed error code")
	}
}

func TestRootTargetReferencesAndPackageIgnoresAssignments(t *testing.T) {
	for _, targets := range []string{`codex=["missing"]`, `codex=["acme/docs", "acme/docs"]`, `codex=[]`} {
		body := "# quoted key in a comment is not a declaration\n" +
			"schema_version=1\n[dependencies]\n\"acme/docs\"=\"1.0.0\"\n[targets]\n" + targets
		doc, err := Decode([]byte(body), "uap.toml")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := doc.Root(); err == nil {
			t.Fatalf("root accepted %s", targets)
		}
		if _, err := doc.Package(); err != nil {
			t.Fatalf("author targets got authority: %v", err)
		}
	}
}

func TestInlineFieldErrorUsesDeclarationLocation(t *testing.T) {
	body := "schema_version=1\n[dependencies]\n\"acme/docs\" = { registry=\"uap\", distribution=\"acme/docs\", release=1, version=\"1.0.0\" }"
	_, err := Decode([]byte(body), "uap.toml")
	if err == nil || !strings.Contains(err.Error(), "uap.toml:3:") {
		t.Fatalf("inline location: %v", err)
	}
}

func TestErrorRetainsStructuredLocationAndCause(t *testing.T) {
	cause := errors.New("validation failed")
	err := manifestError("/package/uap.toml", []byte("schema_version=1\n[targets]\ncodex=[1]"), cause, "targets", "codex")
	var location *Error
	if !errors.As(err, &location) || location.Source != "/package/uap.toml" || location.Line != 3 || !errors.Is(location.Err, cause) {
		t.Fatalf("structured location: %#v", err)
	}
	if !errors.Is(err, cause) || err.Error() != "/package/uap.toml:3: validation failed" {
		t.Fatalf("cause/text changed: %v", err)
	}
	_, err = Decode([]byte("schema_version=1\n[targets]\ncodex=[1]"), "/package/uap.toml")
	if !errors.As(err, &location) || location.Line != 3 {
		t.Fatalf("decoder location: %v", err)
	}
}
