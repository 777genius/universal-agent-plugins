package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// This is the production CLI/dependency/installer/native-filesystem composition
// with real loopback TLS Git and ZIP acquisition. Native Kiro discovery,
// enablement, MCP initialization, model use and runtime are not certified.
func TestProjectKiroComposedSourceLifecycle(t *testing.T) {
	for _, kind := range []string{"git", "zip"} {
		t.Run(kind, func(t *testing.T) {
			transport := newProjectJourneyTransport(t, kind)
			kiroProjectCommitSource(t, transport, projectJourneySkill, "https://managed.test/v1")
			f := newKiroProjectJourney(t, transport)
			userBefore := securityFixtureTree(t, f.kiroRoot(""))
			for _, root := range []string{f.a, f.b} {
				f.run(t, root, "add", transport.endpoint, "--alias", "docs", "--target", "kiro")
				f.assertKiroOwned(t, root, projectJourneySkill, "https://managed.test/v1")
			}
			bNative := securityFixtureTree(t, f.kiroRoot(f.b))
			bState := projectJourneyRead(t, filepath.Join(f.namespace(f.b), "state-v2.json"))
			kiroProjectReconcileAndRepair(t, f)
			kiroProjectUpdateAndTransfer(t, f)
			kiroProjectRemove(t, f)
			if !reflect.DeepEqual(userBefore, securityFixtureTree(t, f.kiroRoot(""))) || !reflect.DeepEqual(bNative, securityFixtureTree(t, f.kiroRoot(f.b))) || bState != projectJourneyRead(t, filepath.Join(f.namespace(f.b), "state-v2.json")) {
				t.Fatal("Kiro project-A lifecycle changed user or project-B native configuration/state")
			}
			f.assertKiroOwned(t, f.b, projectJourneySkill, "https://managed.test/v1")
		})
	}
}

func kiroProjectReconcileAndRepair(t *testing.T, f kiroProjectJourney) {
	t.Helper()
	statePath := filepath.Join(f.namespace(f.a), "state-v2.json")
	before := projectJourneyRead(t, statePath)
	f.run(t, f.a, "deps", "sync", "--locked", "--yes")
	if before != projectJourneyRead(t, statePath) {
		t.Fatal("Kiro locked no-op changed exact ownership receipts")
	}
	installation, _ := f.owned(t, f.a)
	native := securityFixtureTree(t, f.kiroRoot(f.a))
	for _, args := range [][]string{{"list"}, {"info", installation.InstallationID}, {"doctor", installation.InstallationID}, {"outdated", installation.InstallationID}, {"deps", "status"}, {"deps", "tree"}} {
		f.run(t, f.a, args...)
	}
	if before != projectJourneyRead(t, statePath) || !reflect.DeepEqual(native, securityFixtureTree(t, f.kiroRoot(f.a))) {
		t.Fatal("Kiro project read commands changed native files or ownership receipts")
	}
	if err := os.RemoveAll(filepath.Join(f.kiroRoot(f.a), "skills", "docs")); err != nil {
		t.Fatal(err)
	}
	f.run(t, f.a, "repair", installation.InstallationID, "--target", "kiro")
	f.assertKiroOwned(t, f.a, projectJourneySkill, "https://managed.test/v1")
	f.inspectNoRecovery(t, f.a)
}

func kiroProjectUpdateAndTransfer(t *testing.T, f kiroProjectJourney) {
	t.Helper()
	previous, binding := f.owned(t, f.a)
	kiroProjectCommitSource(t, f.transport, projectJourneyAdvancedSkill, "https://managed.test/v2")
	f.run(t, f.a, "update", "docs", "--target", "kiro")
	updated, selected := f.assertKiroOwned(t, f.a, projectJourneyAdvancedSkill, "https://managed.test/v2")
	if updated.Source.SourceBindingID != previous.Source.SourceBindingID || updated.Source.TreeDigest == previous.Source.TreeDigest || selected.ClientBindingID != binding.ClientBindingID || selected.TargetLocator != binding.TargetLocator {
		t.Fatal("Kiro update did not retain source/binding authority while advancing the pin")
	}
	pair := f.assertPortablePair(t, f.a)
	exchange := projectJourneyTemp(t)
	f.run(t, f.a, "deps", "export", exchange, "--yes")
	if pair != f.assertPortablePair(t, exchange) {
		t.Fatal("Kiro export changed exact portable pins")
	}
	cold := newKiroProjectJourney(t, f.transport)
	projectJourneyAbsent(t, cold.cache, filepath.Join(cold.a, ".uap"))
	cold.run(t, cold.a, "deps", "import", exchange, "--install", "--yes")
	restored, _ := cold.assertKiroOwned(t, cold.a, projectJourneyAdvancedSkill, "https://managed.test/v2")
	if restored.Source.SourceBindingID != updated.Source.SourceBindingID || restored.Source.TreeDigest != updated.Source.TreeDigest || pair != cold.assertPortablePair(t, cold.a) {
		t.Fatal("Kiro cold import changed source identity or portable pins")
	}
	cold.inspectNoRecovery(t, cold.a)
}

func kiroProjectRemove(t *testing.T, f kiroProjectJourney) {
	t.Helper()
	_, binding := f.owned(t, f.a)
	f.run(t, f.a, "remove", "docs", "--target", "kiro")
	projectJourneyAbsent(t, binding.TargetLocator, filepath.Join(f.kiroRoot(f.a), "skills", "docs"))
	settings := projectJourneyRead(t, filepath.Join(f.kiroRoot(f.a), "settings", "mcp.json"))
	if strings.Contains(settings, "managed.test") || !strings.Contains(settings, "foreign.test") || !strings.Contains(settings, "foreignPreference") {
		t.Fatalf("Kiro removal changed foreign MCP settings or retained its entry: %s", settings)
	}
	if projectJourneyRead(t, filepath.Join(f.kiroRoot(f.a), "skills", "foreign", "SKILL.md")) != "foreign skill\n" || projectJourneyRead(t, filepath.Join(f.kiroRoot(f.a), "hooks.json")) != "foreign hooks\n" {
		t.Fatal("Kiro removal changed foreign skill or hooks")
	}
	if len(f.state(t, f.a).EnvironmentReceipts) != 0 {
		t.Fatal("Kiro removal retained an environment ownership receipt")
	}
}
