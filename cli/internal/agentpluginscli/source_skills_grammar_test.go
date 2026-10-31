package agentpluginscli

import (
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/manifestedit"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

func TestSourceTransportAndSkillIntentEditRoundTrip(t *testing.T) {
	input := []byte("schema_version=1\n[dependencies.skill]\nkind='skill'\nskill='pdf'\ngit='acme/tools'\npath='skills/pdf'\nref='main'\n[dependencies.git]\ngit='https://git.example/tools.git'\npath='plugins/pdf'\nref='main'\n[dependencies.archive]\narchive='https://packages.example/tools.zip'\npath='plugins/pdf'\n[targets]\ncodex=['skill','git','archive']\n")
	wanted, err := readDependencyManifestBytes("uap.toml", input)
	if err != nil {
		t.Fatal(err)
	}
	empty := environment.Requirements{SchemaVersion: 1, Dependencies: map[string]environment.Requirement{}, Targets: map[string][]string{}}
	changes := manifestedit.Changes{Dependencies: dependencyIntentEdits(empty, wanted), Targets: []manifestedit.TargetEdit{{Name: "codex", Aliases: wanted.Targets["codex"]}}}
	body, err := manifestedit.Apply([]byte("schema_version=1\n"), changes)
	if err != nil {
		t.Fatal(err)
	}
	actual, err := readDependencyManifestBytes("uap.toml", body)
	if err != nil || !reflect.DeepEqual(actual, wanted) {
		t.Fatalf("combined grammar lost source fields: %s %v", body, err)
	}
	keys := map[string]bool{}
	for _, requirement := range actual.Dependencies {
		key, err := requirement.SourceKey()
		if err != nil || keys[key] {
			t.Fatalf("source namespaces collapsed: %q %v", key, err)
		}
		keys[key] = true
	}
}

func TestStandaloneSkillFieldsRefuseOtherTransportFamilies(t *testing.T) {
	for _, source := range []string{"git='https://git.example/tools.git'", "archive='https://packages.example/tools.zip'", "registry='uap'\ndistribution='acme/pdf'\nrelease=1", "version='1.2.3'"} {
		body := []byte("schema_version=1\n[dependencies.pdf]\nkind='skill'\nskill='pdf'\n" + source + "\n")
		if _, err := readDependencyManifestBytes("uap.toml", body); err == nil {
			t.Fatalf("standalone Skill fields accepted on %s", source)
		}
	}
}
