package agentpluginscli

import (
	"os"
	"reflect"
	"strings"
	"testing"
)

// An unrelated saved agreement must not prevent removing an imperative install.
// Live environment receipts must still prevent that imperative fallback.
func TestLifecycleRoutingWithUnrelatedInvalidLock(t *testing.T) {
	for _, missingManifest := range []bool{false, true} {
		for _, byID := range []bool{false, true} {
			name := "stale lock"
			if missingManifest {
				name = "missing manifest"
			}
			if byID {
				name += " by ID"
			}
			t.Run(name, func(t *testing.T) {
				repo, sha := savedJourneyRepository(t)
				journey := newSavedJourney(t, repo)
				journey.run(t, "add", "github:acme/demo@"+sha, "--no-save", "--target", "codex", "--format", "json")
				state := journey.state(t)
				if len(state.Installations) != 1 || len(state.EnvironmentReceipts) != 0 {
					t.Fatalf("fixture is not an imperative installation: %+v", state)
				}
				selector := state.Installations[0].DeclaredName
				if byID {
					selector = state.Installations[0].InstallationID
				}
				mp, lp := journey.cli.app.dependencyPaths()
				if !missingManifest {
					dependencyWrite(t, mp, "schema_version = 1\n[targets]\n")
				}
				dependencyWrite(t, lp, `{"lock_version":1,"manifest_digest":"sha256:`+strings.Repeat("f", 64)+`","packages":[]}`)
				before := reconciliationFiles(t, journey.cli, "")
				journey.run(t, "remove", selector, "--target", "codex", "--external-uninstalled", "--format", "json")
				after := reconciliationFiles(t, journey.cli, "")
				if before[mp] != after[mp] || before[lp] != after[lp] {
					t.Fatal("imperative removal changed unrelated dependency files")
				}
				for _, installation := range journey.state(t).Installations {
					if len(installation.Clients) != 0 {
						t.Fatal("imperative removal left a live client binding")
					}
				}
			})
		}
	}

	t.Run("live saved receipt cannot fall back", func(t *testing.T) {
		fixture, source, _ := reconciliationHarness(t)
		state, err := fixture.store.Load()
		if err != nil {
			t.Fatal(err)
		}
		mp, _ := fixture.app.dependencyPaths()
		if err := os.Remove(mp); err != nil {
			t.Fatal(err)
		}
		before := reconciliationFiles(t, fixture, "")
		downloads, lookups := source.downloads, source.lookups
		for _, selector := range []string{"demo", state.Installations[0].DeclaredName, state.Installations[0].InstallationID} {
			_, _, err := fixture.execute(false, "remove", selector, "--target", "codex", "--external-uninstalled", "--format", "json")
			if err == nil {
				t.Fatalf("saved owner %s fell back to imperative removal", selector)
			}
		}
		if !reflect.DeepEqual(before, reconciliationFiles(t, fixture, "")) || downloads != source.downloads || lookups != source.lookups {
			t.Fatal("saved-owner refusal mutated state or acquired source")
		}
	})
}
