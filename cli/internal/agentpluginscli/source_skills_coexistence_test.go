package agentpluginscli

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/skillsource"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

// Both accepted entrypoints share the actual saved store and transaction while
// retaining different identities, provenance and cold-repair acquisition paths.
func TestSourcesAndStandaloneSkillsShareSavedColdRepair(t *testing.T) {
	transport := newDirectTransportFixture(t, "zip", false)
	journey, repo, original := skillsFullFlowFixture(t)
	configureSkillArchiveSource(t, &journey, transport)
	journey.run(t, "skills", "add", "acme/skills", "--path", "engineering/review", "--target", "codex", "--no-ui", "--yes", "--format", "json")
	journey.run(t, "add", transport.server.URL+"/package.zip", "--target", "codex", "--format", "json")
	requirements, locked := journey.intent(t)
	if len(locked.Packages) != 2 {
		t.Fatalf("coexisting entrypoints lost a pin: %+v", locked)
	}
	var skill environment.LockedPackage
	families := map[string]bool{}
	for _, pin := range locked.Packages {
		if pin.Source.Git != nil && pin.Source.Git.Kind == environment.SkillKind {
			skill, families["skill"] = pin, true
		} else if pin.Source.Archive != nil {
			families["archive"] = true
		} else {
			t.Fatalf("unexpected coexisting source: %+v", pin)
		}
	}
	if len(families) != 2 || skill.SourceKey == "" || !strings.HasPrefix(skill.SourceKey, "github-skill:") {
		t.Fatal("source namespaces or Skill provenance collapsed")
	}
	if err := os.WriteFile(filepath.Join(repo, "engineering", "review", "SKILL.md"), []byte(strings.ReplaceAll(string(original), "fixture-v1", "fixture-v2")), 0600); err != nil {
		t.Fatal(err)
	}
	savedJourneyGit(t, repo, "add", ".")
	savedJourneyGit(t, repo, "commit", "-m", "advance Skill source after combined old pins")
	fresh := newSavedJourney(t, repo)
	configureSkillArchiveSource(t, &fresh, transport)
	copySavedSourceIntent(t, journey, fresh)
	fresh.run(t, "deps", "sync", "--locked", "--yes", "--format", "json")
	repairCombinedSourceInstallations(t, fresh)
	gotRequirements, gotLock := fresh.intent(t)
	if !reflect.DeepEqual(requirements, gotRequirements) || !reflect.DeepEqual(locked, gotLock) {
		t.Fatal("cold repair changed combined saved intent or exact pins")
	}
	assertCombinedPinnedSkill(t, fresh, skill, original)
}

func repairCombinedSourceInstallations(t *testing.T, fresh savedJourney) {
	t.Helper()
	for _, installation := range fresh.state(t).Installations {
		binding := installation.Clients[string(domain.ClientCodex)]
		if binding.ClientBindingID == "" {
			for _, candidate := range installation.Clients {
				if candidate.ClientID == string(domain.ClientCodex) {
					binding = candidate
				}
			}
		}
		if binding.PackageRevision == nil {
			t.Fatal("coexisting binding lost recorded revision")
		}
		if err := os.WriteFile(filepath.Join(binding.TargetLocator, "combined-drift.txt"), []byte("explicit repair"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.RemoveAll(fresh.cli.app.EnvironmentStoreRoot); err != nil {
			t.Fatal(err)
		}
		fresh.run(t, "repair", installation.InstallationID, "--target", "codex", "--format", "json")
	}
}

func assertCombinedPinnedSkill(t *testing.T, fresh savedJourney, skill environment.LockedPackage, original []byte) {
	t.Helper()
	stored, err := fresh.cli.app.dependencyStore().Get(context.Background(), skill.DigestAlgorithm, skill.TreeDigest)
	if err != nil {
		// The final archive repair may have removed the preceding Skill store.
		loaded, loadErr := materializeLockedPackage(context.Background(), fresh.cli.app, skill, true, nil)
		if loadErr != nil {
			t.Fatal(loadErr)
		}
		if loaded.cleanup != nil {
			_ = loaded.cleanup()
		}
		stored, err = fresh.cli.app.dependencyStore().Get(context.Background(), skill.DigestAlgorithm, skill.TreeDigest)
	}
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(stored.Root, "skills", "review", "SKILL.md"))
	if err != nil || string(body) != string(original) {
		t.Fatalf("combined cold replay followed moving Skill ref: %q %v", body, err)
	}
}

func configureSkillArchiveSource(t *testing.T, journey *savedJourney, transport *directTransportFixture) {
	t.Helper()
	acquirer := journey.source.Acquirer
	acquirer.HTTPClient = transport.server.Client()
	journey.cli.app.SourceAcquirer = skillsource.Acquirer{Acquirer: acquirer}
}

func copySavedSourceIntent(t *testing.T, from, to savedJourney) {
	t.Helper()
	manifest, lock := from.cli.app.dependencyPaths()
	newManifest, newLock := to.cli.app.dependencyPaths()
	for source, destination := range map[string]string{manifest: newManifest, lock: newLock} {
		body, err := os.ReadFile(source)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Dir(destination), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(destination, body, 0600); err != nil {
			t.Fatal(err)
		}
	}
}
