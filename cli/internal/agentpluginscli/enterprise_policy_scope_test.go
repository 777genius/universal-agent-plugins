package agentpluginscli

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/enterprisepolicy"
)

func TestEnterprisePolicySelectedSkillKeepsUnrelatedDeniedArchive(t *testing.T) {
	j, source := removalArchiveJourney(t, map[string][]string{"keep": nil})
	j.run(t, "add", source+"/keep.zip", "--alias", "keep", "--target", "claude", "--format", "json")
	retained := savedJourneyBinding(t, j.state(t), "claude")
	retainedFiles := savedJourneyTree(t, retained.TargetLocator)
	policy := filepath.Join(t.TempDir(), "policy.yaml")
	dependencyWrite(t, policy, "schema_version: 2\nsources:\n  allowed_kinds: [local]\ninstall:\n  allowed_targets: [codex]\n")
	files := &enterprisepolicy.Files{Paths: []enterprisepolicy.Path{{Layer: "deployment", Path: policy, Required: true}}}
	j.cli.app.PolicyFiles = files
	j.cli.app = j.cli.app.withEnterprisePolicyGate(files)
	sourceParent, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(sourceParent, "review")
	if err := os.MkdirAll(root, 0700); err != nil {
		t.Fatal(err)
	}
	dependencyWrite(t, filepath.Join(root, "SKILL.md"), "---\nname: review\ndescription: Review local code\n---\nLocal instructions.\n")
	j.run(t, "skills", "add", root, "--target", "codex", "--yes", "--format", "json")
	requirements, locked := j.intent(t)
	if len(requirements.Dependencies) != 2 || len(locked.Packages) != 2 {
		t.Fatal("allowed selection did not preserve both declarations", requirements, locked)
	}
	if got := savedJourneyBinding(t, j.state(t), "claude"); !reflect.DeepEqual(retained, got) || !reflect.DeepEqual(retainedFiles, savedJourneyTree(t, got.TargetLocator)) {
		t.Fatal("allowed Skill selection changed an unrelated denied binding")
	}
	if got := savedJourneyBinding(t, j.state(t), "codex"); got.ClientID != "codex" {
		t.Fatal("allowed Skill was not installed")
	}
	before := j.state(t)
	if _, _, err := j.cli.execute(false, "repair", "keep", "--target", "claude", "--yes", "--format", "json"); err == nil || !reflect.DeepEqual(before, j.state(t)) {
		t.Fatalf("selected denied archive was grandfathered: %v", err)
	}
}
