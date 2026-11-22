package manifestedit

import (
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

func TestMoveDevelopmentDependencyWithoutTouchingUnrelatedKeys(t *testing.T) {
	input := []byte(`# note
schema_version=1
[dependencies]
ordinary="^1.0" # keep
[dev-dependencies]
build="~2.0" # development
[targets]
codex=["ordinary","build"]
`)
	after, err := Apply(input, Changes{Dependencies: []DependencyEdit{
		{Alias: "build", Group: "dev-dependencies"},
		{Alias: "build", Group: "dependencies", Value: &Dependency{Version: "~2.0"}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(after), "# keep") || !strings.Contains(string(after), "# note") {
		t.Fatalf("unrelated comments lost:\n%s", after)
	}
	var x map[string]any
	if err := toml.Unmarshal(after, &x); err != nil {
		t.Fatal(err)
	}
	normal := x["dependencies"].(map[string]any)
	if normal["build"] != "~2.0" || normal["ordinary"] != "^1.0" {
		t.Fatalf("ordinary changed: %+v", normal)
	}
	if local, ok := x["dev-dependencies"].(map[string]any); ok && len(local) != 0 {
		t.Fatalf("development declaration survived promotion: %+v", local)
	}
}
