package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// This is the production CLI/dependency/installer/native-filesystem composition
// with real loopback TLS Git and ZIP acquisition. Native Cursor discovery,
// enablement, MCP initialization, model use and runtime are not certified.
func TestProjectCursorComposedSourceLifecycle(t *testing.T) {
	for _, kind := range []string{"git", "zip"} {
		t.Run(kind, func(t *testing.T) {
			transport := newProjectJourneyTransport(t, kind)
			cursorProjectCommitSource(t, transport, projectJourneySkill, "https://managed.test/v1")
			f := newCursorProjectJourney(t, transport)
			userBefore := securityFixtureTree(t, f.cursorRoot(""))
			for _, root := range []string{f.a, f.b} {
				f.run(t, root, "add", transport.endpoint, "--alias", "docs", "--target", "cursor")
				f.assertCursorOwned(t, root, projectJourneySkill, "https://managed.test/v1")
			}
			bNative := securityFixtureTree(t, f.cursorRoot(f.b))
			bState := projectJourneyRead(t, filepath.Join(f.namespace(f.b), "state-v2.json"))
			cursorProjectReconcileAndRepair(t, f)
			cursorProjectUpdateAndTransfer(t, f)
			cursorProjectRemove(t, f)
			if !reflect.DeepEqual(userBefore, securityFixtureTree(t, f.cursorRoot(""))) || !reflect.DeepEqual(bNative, securityFixtureTree(t, f.cursorRoot(f.b))) || bState != projectJourneyRead(t, filepath.Join(f.namespace(f.b), "state-v2.json")) {
				t.Fatal("Cursor project-A lifecycle changed user or project-B native configuration/state")
			}
			f.assertCursorOwned(t, f.b, projectJourneySkill, "https://managed.test/v1")
		})
	}
}

func cursorProjectReconcileAndRepair(t *testing.T, f cursorProjectJourney) {
	t.Helper()
	statePath := filepath.Join(f.namespace(f.a), "state-v2.json")
	before := projectJourneyRead(t, statePath)
	f.run(t, f.a, "deps", "sync", "--locked", "--yes")
	if before != projectJourneyRead(t, statePath) {
		t.Fatal("Cursor locked no-op changed exact ownership receipts")
	}
	installation, _ := f.owned(t, f.a)
	native := securityFixtureTree(t, f.cursorRoot(f.a))
	for _, args := range [][]string{{"list"}, {"info", installation.InstallationID}, {"doctor", installation.InstallationID}, {"outdated", installation.InstallationID}, {"deps", "status"}, {"deps", "tree"}} {
		f.run(t, f.a, args...)
	}
	if before != projectJourneyRead(t, statePath) || !reflect.DeepEqual(native, securityFixtureTree(t, f.cursorRoot(f.a))) {
		t.Fatal("Cursor project read commands changed native files or ownership receipts")
	}
	if err := os.RemoveAll(filepath.Join(f.cursorRoot(f.a), "skills", "docs")); err != nil {
		t.Fatal(err)
	}
	f.run(t, f.a, "repair", installation.InstallationID, "--target", "cursor")
	f.assertCursorOwned(t, f.a, projectJourneySkill, "https://managed.test/v1")
	f.inspectNoRecovery(t, f.a)
}

func cursorProjectUpdateAndTransfer(t *testing.T, f cursorProjectJourney) {
	t.Helper()
	previous, binding := f.owned(t, f.a)
	cursorProjectCommitSource(t, f.transport, projectJourneyAdvancedSkill, "https://managed.test/v2")
	f.run(t, f.a, "update", "docs", "--target", "cursor")
	updated, selected := f.assertCursorOwned(t, f.a, projectJourneyAdvancedSkill, "https://managed.test/v2")
	if updated.Source.SourceBindingID != previous.Source.SourceBindingID || updated.Source.TreeDigest == previous.Source.TreeDigest || selected.ClientBindingID != binding.ClientBindingID || selected.TargetLocator != binding.TargetLocator {
		t.Fatal("Cursor update did not retain source/binding authority while advancing the pin")
	}
	pair := f.assertPortablePair(t, f.a)
	exchange := projectJourneyTemp(t)
	f.run(t, f.a, "deps", "export", exchange, "--yes")
	if pair != f.assertPortablePair(t, exchange) {
		t.Fatal("Cursor export changed exact portable pins")
	}
	cold := newCursorProjectJourney(t, f.transport)
	projectJourneyAbsent(t, cold.cache, filepath.Join(cold.a, ".uap"))
	cold.run(t, cold.a, "deps", "import", exchange, "--install", "--yes")
	restored, _ := cold.assertCursorOwned(t, cold.a, projectJourneyAdvancedSkill, "https://managed.test/v2")
	if restored.Source.SourceBindingID != updated.Source.SourceBindingID || restored.Source.TreeDigest != updated.Source.TreeDigest || pair != cold.assertPortablePair(t, cold.a) {
		t.Fatal("Cursor cold import changed source identity or portable pins")
	}
	cold.inspectNoRecovery(t, cold.a)
}

func cursorProjectRemove(t *testing.T, f cursorProjectJourney) {
	t.Helper()
	_, binding := f.owned(t, f.a)
	f.run(t, f.a, "remove", "docs", "--target", "cursor")
	projectJourneyAbsent(t, binding.TargetLocator, filepath.Join(f.cursorRoot(f.a), "skills", "docs"))
	settings := projectJourneyRead(t, filepath.Join(f.cursorRoot(f.a), "mcp.json"))
	if strings.Contains(settings, "managed.test") || !strings.Contains(settings, "foreign.test") || !strings.Contains(settings, "foreignPreference") {
		t.Fatalf("Cursor removal changed foreign MCP settings or retained its entry: %s", settings)
	}
	if projectJourneyRead(t, filepath.Join(f.cursorRoot(f.a), "skills", "foreign", "SKILL.md")) != "foreign skill\n" || projectJourneyRead(t, filepath.Join(f.cursorRoot(f.a), "hooks.json")) != "foreign hooks\n" {
		t.Fatal("Cursor removal changed foreign skill or hooks")
	}
	if len(f.state(t, f.a).EnvironmentReceipts) != 0 {
		t.Fatal("Cursor removal retained an environment ownership receipt")
	}
}
