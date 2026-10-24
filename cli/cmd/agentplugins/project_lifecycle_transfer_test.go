package main

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestProjectComposedAgreementColdRestoreUsesPortablePins(t *testing.T) {
	for _, kind := range []string{"git", "zip"} {
		t.Run(kind, func(t *testing.T) {
			transport := newProjectJourneyTransport(t, kind)
			original := newProjectJourney(t, transport)
			original.run(t, original.a, "add", transport.endpoint, "--alias", "docs", "--target", "gemini")
			installation := original.assertOwnedContext(t, original.a)
			pair := original.assertPortablePair(t, original.a)
			exchange := t.TempDir()
			original.run(t, original.a, "deps", "export", exchange, "--yes")
			if pair != original.assertPortablePair(t, exchange) {
				t.Fatal("export changed the exact portable agreement")
			}
			before := securityFixtureTree(t, original.a)
			if _, err := original.execute(original.a, "deps", "export", original.a, "--yes"); err == nil || !strings.Contains(err.Error(), "different exchange directory") {
				t.Fatalf("same-pair export = %v", err)
			}
			if !reflect.DeepEqual(before, securityFixtureTree(t, original.a)) {
				t.Fatal("refused same-pair transfer wrote project state")
			}
			cold := newProjectJourney(t, transport)
			projectJourneyAbsent(t, cold.cache, filepath.Join(cold.a, ".uap"))
			cold.run(t, cold.a, "deps", "import", exchange, "--install", "--yes")
			restored := cold.assertOwnedContext(t, cold.a)
			if restored.Source.SourceBindingID != installation.Source.SourceBindingID || restored.Source.TreeDigest != installation.Source.TreeDigest || pair != cold.assertPortablePair(t, cold.a) {
				t.Fatal("cold project import changed the pinned source or agreement")
			}
			cold.inspectNoRecovery(t, cold.a)
			if kind == "zip" {
				projectJourneyColdChangedArchiveRefused(t, original, exchange, pair)
			}
		})
	}
}

func projectJourneyColdChangedArchiveRefused(t *testing.T, original projectJourney, exchange string, pair [2]string) {
	t.Helper()
	transport := original.transport
	projectJourneyWrite(t, filepath.Join(transport.repo, "skills", "docs", "SKILL.md"), "changed bytes after export\n")
	transport.mu.Lock()
	transport.archive = projectJourneyArchive(t, transport.repo)
	transport.mu.Unlock()
	refused := newProjectJourney(t, transport)
	before := securityFixtureTree(t, refused.nativeRoot(refused.a))
	if _, err := refused.execute(refused.a, "deps", "import", exchange, "--install", "--yes"); err == nil || !strings.Contains(strings.ToLower(err.Error()), "sha256") {
		t.Fatalf("changed pinned archive was not refused: %v", err)
	}
	projectJourneyAbsent(t, filepath.Join(refused.a, "uap.toml"), filepath.Join(refused.a, "uap.lock"), filepath.Join(refused.a, ".uap", "state-v2.json"), filepath.Join(refused.cache, "store"))
	if !reflect.DeepEqual(before, securityFixtureTree(t, refused.nativeRoot(refused.a))) || pair != original.assertPortablePair(t, exchange) {
		t.Fatal("refused cold archive restore changed native state or exported pins")
	}
}
