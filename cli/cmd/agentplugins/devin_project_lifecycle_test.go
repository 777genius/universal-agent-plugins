package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// This is the production CLI/dependency/installer/native-filesystem composition
// with real loopback TLS Git and ZIP acquisition. Native Devin discovery,
// enablement, MCP initialization, model use and runtime are not certified.
func TestProjectDevinComposedSourceLifecycle(t *testing.T) {
	for _, kind := range []string{"git", "zip"} {
		t.Run(kind, func(t *testing.T) {
			transport := newProjectJourneyTransport(t, kind)
			devinProjectCommitSource(t, transport, projectJourneySkill, "https://managed.test/v1")
			f := newDevinProjectJourney(t, transport)
			userBefore := securityFixtureTree(t, f.devinRoot(""))
			for _, root := range []string{f.a, f.b} {
				f.run(t, root, "add", transport.endpoint, "--alias", "docs", "--target", "windsurf")
				f.assertDevinOwned(t, root, projectJourneySkill, "https://managed.test/v1")
			}
			bNative := securityFixtureTree(t, f.devinRoot(f.b))
			bState := projectJourneyRead(t, filepath.Join(f.namespace(f.b), "state-v2.json"))
			devinProjectReconcileAndRepair(t, f)
			devinProjectUpdateAndTransfer(t, f)
			devinProjectRemove(t, f)
			if !reflect.DeepEqual(userBefore, securityFixtureTree(t, f.devinRoot(""))) || !reflect.DeepEqual(bNative, securityFixtureTree(t, f.devinRoot(f.b))) || bState != projectJourneyRead(t, filepath.Join(f.namespace(f.b), "state-v2.json")) {
				t.Fatal("Devin project-A lifecycle changed user or project-B native configuration/state")
			}
			f.assertDevinOwned(t, f.b, projectJourneySkill, "https://managed.test/v1")
		})
	}
}

func devinProjectReconcileAndRepair(t *testing.T, f devinProjectJourney) {
	t.Helper()
	statePath := filepath.Join(f.namespace(f.a), "state-v2.json")
	before := projectJourneyRead(t, statePath)
	f.run(t, f.a, "deps", "sync", "--locked", "--yes")
	if before != projectJourneyRead(t, statePath) {
		t.Fatal("Devin locked no-op changed exact ownership receipts")
	}
	installation, _ := f.owned(t, f.a)
	native := securityFixtureTree(t, f.devinRoot(f.a))
	for _, args := range [][]string{{"list"}, {"info", installation.InstallationID}, {"doctor", installation.InstallationID}, {"outdated", installation.InstallationID}, {"deps", "status"}, {"deps", "tree"}} {
		f.run(t, f.a, args...)
	}
	if before != projectJourneyRead(t, statePath) || !reflect.DeepEqual(native, securityFixtureTree(t, f.devinRoot(f.a))) {
		t.Fatal("Devin project read commands changed native files or ownership receipts")
	}
	if err := os.RemoveAll(filepath.Join(f.devinRoot(f.a), "skills", "docs")); err != nil {
		t.Fatal(err)
	}
	f.run(t, f.a, "repair", installation.InstallationID, "--target", "windsurf")
	f.assertDevinOwned(t, f.a, projectJourneySkill, "https://managed.test/v1")
	f.inspectNoRecovery(t, f.a)
}

func devinProjectUpdateAndTransfer(t *testing.T, f devinProjectJourney) {
	t.Helper()
	previous, binding := f.owned(t, f.a)
	devinProjectCommitSource(t, f.transport, projectJourneyAdvancedSkill, "https://managed.test/v2")
	f.run(t, f.a, "update", "docs", "--target", "windsurf")
	updated, selected := f.assertDevinOwned(t, f.a, projectJourneyAdvancedSkill, "https://managed.test/v2")
	if updated.Source.SourceBindingID != previous.Source.SourceBindingID || updated.Source.TreeDigest == previous.Source.TreeDigest || selected.ClientBindingID != binding.ClientBindingID || selected.TargetLocator != binding.TargetLocator {
		t.Fatal("Devin update did not retain source/binding authority while advancing the pin")
	}
	pair := f.assertPortablePair(t, f.a)
	exchange := t.TempDir()
	f.run(t, f.a, "deps", "export", exchange, "--yes")
	if pair != f.assertPortablePair(t, exchange) {
		t.Fatal("Devin export changed exact portable pins")
	}
	cold := newDevinProjectJourney(t, f.transport)
	projectJourneyAbsent(t, cold.cache, filepath.Join(cold.a, ".uap"))
	cold.run(t, cold.a, "deps", "import", exchange, "--install", "--yes")
	restored, _ := cold.assertDevinOwned(t, cold.a, projectJourneyAdvancedSkill, "https://managed.test/v2")
	if restored.Source.SourceBindingID != updated.Source.SourceBindingID || restored.Source.TreeDigest != updated.Source.TreeDigest || pair != cold.assertPortablePair(t, cold.a) {
		t.Fatal("Devin cold import changed source identity or portable pins")
	}
	cold.inspectNoRecovery(t, cold.a)
}

func devinProjectRemove(t *testing.T, f devinProjectJourney) {
	t.Helper()
	_, binding := f.owned(t, f.a)
	f.run(t, f.a, "remove", "docs", "--target", "windsurf")
	projectJourneyAbsent(t, binding.TargetLocator, filepath.Join(f.devinRoot(f.a), "skills", "docs"))
	settings := projectJourneyRead(t, filepath.Join(f.devinRoot(f.a), "mcp_config.json"))
	if strings.Contains(settings, "managed.test") || !strings.Contains(settings, "foreign.test") || !strings.Contains(settings, "foreignPreference") {
		t.Fatalf("Devin removal changed foreign MCP settings or retained its entry: %s", settings)
	}
	if projectJourneyRead(t, filepath.Join(f.devinRoot(f.a), "skills", "foreign", "SKILL.md")) != "foreign skill\n" || projectJourneyRead(t, filepath.Join(f.devinRoot(f.a), "hooks.json")) != "foreign hooks\n" {
		t.Fatal("Devin removal changed foreign skill or hooks")
	}
	if len(f.state(t, f.a).EnvironmentReceipts) != 0 {
		t.Fatal("Devin removal retained an environment ownership receipt")
	}
}
