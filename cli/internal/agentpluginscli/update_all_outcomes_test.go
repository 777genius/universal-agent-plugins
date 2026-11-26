package agentpluginscli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/commandext"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// Current rendering rules still require an apply-time comparison, even when
// they ultimately reproduce the installed bytes. Count that actual no-change
// outcome separately from a peer that really updates its package.
func TestUpdateAllCountsActualNoChangeAfterProjectionReview(t *testing.T) {
	fixture := newCLIFixture(t, []domain.DetectedClient{fixtureClient(t, domain.ClientCursor)})
	alpha := writeNamedCLIPlugin(t, "alpha", "1.0.0")
	writeCommandFixtureFile(t, filepath.Join(alpha, filepath.FromSlash(commandext.RelativeDirectory), "echo.md"), "---\ndescription: Echo the text\ndisable-model-invocation: false\n---\nReply with UAP-ECHO.\n")
	beta := writeNamedCLIPlugin(t, "beta", "1.0.0")
	for _, plugin := range []string{alpha, beta} {
		if stdout, _, err := fixture.execute(false, "add", plugin, "--target", "cursor"); err != nil {
			t.Fatalf("seed: %v\n%s", err, stdout)
		}
	}
	before, err := fixture.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	var installedCommand string
	for _, installation := range before.Installations {
		if installation.DeclaredName == "alpha" {
			for _, binding := range installation.Clients {
				installedCommand = filepath.Join(binding.TargetLocator, "skills", "alpha-echo", "SKILL.md")
			}
		}
	}
	fileBefore, err := os.Stat(installedCommand)
	if err != nil {
		t.Fatal(err)
	}
	writeNamedCLIPluginVersion(t, beta, "beta", "2.0.0")
	stdout, _, err := fixture.execute(false, "update", "--all", "--format", "json")
	if err != nil {
		t.Fatalf("public update: %v\n%s", err, stdout)
	}
	var envelope struct {
		Data updateAllResult `json:"data"`
	}
	if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
		t.Fatal(err)
	}
	result := envelope.Data
	if len(result.Installations) != 2 || result.Installations[0].Name != "alpha" {
		t.Fatalf("unexpected public result: %s", stdout)
	}
	current := result.Installations[0]
	if current.Plan == nil || len(current.Plan.Targets) != 1 {
		t.Fatalf("unchanged item never reached the real apply: %s", stdout)
	}
	actual := current.Plan.Targets[0].Output.Result
	if current.Plan.DryRun || !actual.NoChange || actual.Mutated {
		t.Fatalf("fixture did not produce an authoritative no-change apply: %+v", current.Plan)
	}
	after, err := fixture.store.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, installation := range after.Installations {
		if installation.DeclaredName == "alpha" {
			for _, original := range before.Installations {
				if original.InstallationID == installation.InstallationID && !reflect.DeepEqual(original.Clients, installation.Clients) {
					t.Fatal("unchanged target unexpectedly changed its durable binding")
				}
			}
		} else if installation.Package.Version != "2.0.0" {
			t.Fatal("changed peer did not update")
		}
	}
	fileAfter, err := os.Stat(installedCommand)
	if err != nil || !os.SameFile(fileBefore, fileAfter) || !fileBefore.ModTime().Equal(fileAfter.ModTime()) {
		t.Fatalf("unchanged command tree was replaced: %v", err)
	}
	writeNamedCLIPluginVersion(t, beta, "beta", "3.0.0")
	human, _, err := fixture.execute(false, "update", "--all")
	if err != nil {
		t.Fatalf("human update: %v\n%s", err, human)
	}
	if !strings.Contains(human, "alpha: skipped") || !strings.Contains(human, "updated=1 skipped=1") {
		t.Errorf("human summary counts a no-change apply as updated:\n%s", human)
	}
	if result.Updated != 1 || result.Skipped != 1 || current.Status != "skipped" {
		t.Fatalf("summary counts a no-change apply as updated: updated=%d skipped=%d alpha=%s\n%s", result.Updated, result.Skipped, current.Status, stdout)
	}
}
