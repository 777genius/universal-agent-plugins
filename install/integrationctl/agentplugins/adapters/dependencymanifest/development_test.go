package dependencymanifest

import (
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

func TestDevelopmentDependenciesRootAndConsumer(t *testing.T) {
	body := []byte(`schema_version = 1
[dependencies]
"acme/ordinary" = "^1.2"
[dev-dependencies]
build = "~2.1.0"
[dev-dependencies.tool]
git = "acme/build"
ref = "main"
[targets]
codex = ["acme/ordinary", "build", "tool"]
`)
	doc, err := Decode(body, "uap.toml")
	if err != nil {
		t.Fatal(err)
	}
	root, err := doc.Root()
	if err != nil {
		t.Fatal(err)
	}
	if len(root.Dependencies) != 3 || len(root.Development) != 2 ||
		!root.Development["build"] || !root.Development["tool"] {
		t.Fatalf("local classification: %+v", root)
	}
	consumer, err := doc.Package()
	if err != nil || len(consumer) != 1 || consumer["acme/ordinary"].Registry == nil {
		t.Fatalf("build-only dependency leaked to consumer: %+v %v", consumer, err)
	}
	previous, err := environment.ManifestDigest(root)
	if err != nil {
		t.Fatal(err)
	}
	delete(root.Development, "build")
	updated, err := environment.ManifestDigest(root)
	if err != nil || updated == previous {
		t.Fatalf("development classification not fingerprinted: %v", err)
	}
	again, err := doc.Root()
	if err != nil || len(again.Development) != 2 {
		t.Fatalf("mutable view: %+v %v", again, err)
	}
}

func TestDevelopmentDependenciesRejectConflicts(t *testing.T) {
	for _, input := range []string{
		`schema_version=1
[dependencies]
a="1.0.0"
[dev-dependencies]
a="1.0.0"
[targets]
codex=["a"]
`,
		`schema_version=1
[dependencies.a]
git="acme/same"
[dev-dependencies.b]
git="acme/same"
[targets]
codex=["a","b"]
`,
	} {
		if _, err := Decode([]byte(input), "uap.toml"); err == nil || !strings.Contains(err.Error(), "dev-dependencies") {
			t.Fatalf("accepted duplicate development dependency: %v", err)
		}
	}
}

func TestDevelopmentDependenciesMustHaveRootTargets(t *testing.T) {
	doc, err := Decode([]byte(`schema_version=1
[dev-dependencies]
build="^1.2"
`), "uap.toml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := doc.Root(); err == nil {
		t.Fatal("unassigned development root accepted")
	}
	dependencies, err := doc.Package()
	if err != nil || len(dependencies) != 0 {
		t.Fatalf("dev-only package exports %v %v", dependencies, err)
	}
}
