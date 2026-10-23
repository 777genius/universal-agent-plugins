package main

import (
	"os"
	"path/filepath"
	"testing"
)

// Cold-cache portable import uses the production transport/store/scope path;
// native Kimi and the inert trusted launcher are never executed.
func TestProjectKimiColdSavedImportPreservesPinnedAgreement(t *testing.T) {
	transport := newProjectJourneyTransport(t, "archive")
	original := newProjectJourney(t, transport)
	original.app = configureProjectKimiJourney(original.app, filepath.Join(original.home, ".kimi-code"), filepath.Join(original.home, "kimi"))
	if err := os.Mkdir(filepath.Join(original.a, ".git"), 0700); err != nil {
		t.Fatal(err)
	}
	original.run(t, original.a, "add", transport.endpoint, "--alias", "docs", "--target", "kimi")
	installation, _ := projectKimiOwned(t, original, original.a, projectJourneySkill)
	pair := original.assertPortablePair(t, original.a)
	exchange := t.TempDir()
	original.run(t, original.a, "deps", "export", exchange, "--yes")
	cold := newProjectJourney(t, transport)
	cold.app = configureProjectKimiJourney(cold.app, filepath.Join(cold.home, ".kimi-code"), filepath.Join(cold.home, "kimi"))
	if err := os.Mkdir(filepath.Join(cold.a, ".git"), 0700); err != nil {
		t.Fatal(err)
	}
	projectJourneyAbsent(t, cold.cache, filepath.Join(cold.a, ".uap"))
	cold.run(t, cold.a, "deps", "import", exchange, "--install", "--yes")
	restored, _ := projectKimiOwned(t, cold, cold.a, projectJourneySkill)
	if restored.Source.SourceBindingID != installation.Source.SourceBindingID || restored.Source.TreeDigest != installation.Source.TreeDigest || pair != cold.assertPortablePair(t, cold.a) {
		t.Fatal("Kimi cold restore changed exact source/portable agreement")
	}
	cold.run(t, cold.a, "remove", "docs", "--target", "kimi")
}
