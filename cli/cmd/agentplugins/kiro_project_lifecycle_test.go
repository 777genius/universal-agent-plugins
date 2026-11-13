package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/kiro"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
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
			// Keep B's original file identities as well as bytes while A changes.
			f.preserve(t, f.kiroRoot(f.b), filepath.Join(f.namespace(f.b), "state-v2.json"))
			kiroProjectReconcileAndRepair(t, f)
			kiroProjectUpdateAndTransfer(t, f)
			kiroProjectRemove(t, f)
			if !reflect.DeepEqual(userBefore, securityFixtureTree(t, f.kiroRoot(""))) || !reflect.DeepEqual(bNative, securityFixtureTree(t, f.kiroRoot(f.b))) || bState != projectJourneyRead(t, filepath.Join(f.namespace(f.b), "state-v2.json")) {
				t.Fatal("Kiro project-A lifecycle changed user or project-B native configuration/state")
			}
			f.assertKiroOwned(t, f.b, projectJourneySkill, "https://managed.test/v1")
			t.Run("user", func(t *testing.T) { kiroUserPreparedSourceLifecycle(t, transport) })
			f.assertProtected(t)
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
	cold.preserve(t, cold.kiroRoot(cold.b))
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

// A Run-only rejecting runner exercises the public prepared file lifecycle.
// It does not certify native Kiro startup, login or MCP initialization.
func kiroUserPreparedSourceLifecycle(t *testing.T, transport *projectJourneyTransport) {
	t.Helper()
	skill := kiroUserCommitSource(t, transport, "original User pin")
	f := newKiroJourney(t, transport, true)
	f.run(t, "", "add", transport.endpoint, "--alias", "user-docs", "--target", "kiro", "--scope", "user", "--yes")
	original, binding := f.assertKiroUserOwned(t, skill)
	projectJourneyDataMarker(t, f.projectJourney, "", original)
	for _, receipt := range original.DataReceipts {
		f.preserve(t, filepath.Join(receipt.Locator, "keep.txt"))
	}
	f.run(t, "", "add", transport.endpoint, "--alias", "user-docs", "--target", "kiro", "--scope", "user", "--yes")
	repeated, repeatedBinding := f.assertKiroUserOwned(t, skill)
	if !reflect.DeepEqual(original, repeated) || !reflect.DeepEqual(binding, repeatedBinding) {
		t.Fatal("Kiro saved User repeat changed original installation or binding authority")
	}
	updated, selected, updatedSkill := kiroUserUpdateAndRepair(t, f, original, binding)
	kiroUserTransfer(t, f, updated, updatedSkill)
	f.run(t, "", "remove", "user-docs", "--target", "kiro", "--scope", "user", "--yes")
	projectJourneyAbsent(t, selected.TargetLocator, filepath.Join(f.kiroRoot(""), "skills", "uap-user-docs"))
	state := f.state(t, "")
	if len(state.Installations) != 1 || state.Installations[0].InstallationID != original.InstallationID || len(state.Installations[0].Clients) != 0 || !state.Installations[0].DataRetained ||
		!reflect.DeepEqual(state.Installations[0].DataReceipts, original.DataReceipts) || len(state.EnvironmentReceipts) != 0 {
		t.Fatal("Kiro User remove lost retained original data authority")
	}
	servers, _, _, _, err := kiro.ReadMCPConfig(filepath.Join(f.kiroRoot(""), "settings", "mcp.json"))
	if err != nil || servers["uap_user_probe"] != nil {
		t.Fatal("Kiro User remove retained selected MCP configuration")
	}
	f.assertProtected(t)
}

func kiroUserUpdateAndRepair(t *testing.T, f kiroProjectJourney, original domain.Installation, binding domain.ClientBinding) (domain.Installation, domain.ClientBinding, string) {
	t.Helper()
	skill := kiroUserCommitSource(t, f.transport, "updated User pin")
	f.run(t, "", "update", "user-docs", "--target", "kiro", "--scope", "user", "--yes")
	updated, selected := f.assertKiroUserOwned(t, skill)
	expectedSource := original.Source
	expectedSource.ResolvedRevision, expectedSource.TreeDigest = updated.Source.ResolvedRevision, updated.Source.TreeDigest
	if original.Source.SourceKind == "archive" {
		expectedSource.ArchiveSHA256 = updated.Source.ArchiveSHA256
		if updated.Source.ResolvedRevision != "sha256:"+updated.Source.ArchiveSHA256 {
			t.Fatal("Kiro User archive revision does not join its exact archive digest")
		}
	}
	if updated.InstallationID != original.InstallationID || updated.Source != expectedSource || updated.Source.ResolvedRevision == original.Source.ResolvedRevision || updated.Source.TreeDigest == original.Source.TreeDigest ||
		selected.ClientBindingID != binding.ClientBindingID || selected.TargetLocator != binding.TargetLocator || !reflect.DeepEqual(updated.DataReceipts, original.DataReceipts) {
		t.Fatal("Kiro User update changed source/binding/data authority instead of advancing its pin")
	}
	// The exact receipt-verified owned Skill is quarantined as a declared test
	// fault. No original peer/policy tree is deleted or rebaselined.
	source := filepath.Join(f.kiroRoot(""), "skills", "uap-user-docs")
	if err := os.Rename(source, filepath.Join(f.home, "owned-skill-quarantine")); err != nil {
		t.Fatal(err)
	}
	f.run(t, "", "repair", updated.InstallationID, "--target", "kiro", "--scope", "user", "--yes")
	repaired, repairedBinding := f.assertKiroUserOwned(t, skill)
	if repaired.InstallationID != updated.InstallationID || !reflect.DeepEqual(repaired.Source, updated.Source) || !reflect.DeepEqual(repaired.Package, updated.Package) ||
		!reflect.DeepEqual(repaired.DataReceipts, original.DataReceipts) || repairedBinding.ClientBindingID != selected.ClientBindingID || repairedBinding.TargetLocator != selected.TargetLocator {
		t.Fatal("Kiro User repair changed the last accepted revision or original data authority")
	}
	return repaired, repairedBinding, skill
}

func kiroUserTransfer(t *testing.T, f kiroProjectJourney, updated domain.Installation, skill string) {
	t.Helper()
	pair := f.assertPortablePair(t, f.cache)
	exchange := projectJourneyTemp(t)
	f.run(t, "", "deps", "export", exchange, "--scope", "user", "--yes")
	if pair != f.assertPortablePair(t, exchange) {
		t.Fatal("Kiro User export changed portable pins")
	}
	cold := newKiroJourney(t, f.transport, true)
	projectJourneyAbsent(t, cold.cache)
	cold.run(t, "", "deps", "import", exchange, "--install", "--scope", "user", "--yes")
	restored, _ := cold.assertKiroUserOwned(t, skill)
	if restored.Source != updated.Source || pair != cold.assertPortablePair(t, cold.cache) {
		t.Fatal("Kiro cold User import changed selected source or agreement pins")
	}
	for _, receipt := range restored.DataReceipts {
		for _, original := range updated.DataReceipts {
			if receipt.Locator == original.Locator {
				t.Fatal("Kiro cold User import reused another world's data authority")
			}
		}
	}
	cold.assertProtected(t)
}

func kiroUserCommitSource(t *testing.T, transport *projectJourneyTransport, content string) string {
	t.Helper()
	oldSkill := filepath.Join(transport.repo, "skills", "docs")
	if _, err := os.Stat(oldSkill); err == nil {
		if err := os.Rename(oldSkill, filepath.Join(transport.repo, "skills", "uap-user-docs")); err != nil {
			t.Fatal(err)
		}
	} else if !os.IsNotExist(err) {
		t.Fatal(err)
	}
	skill := "---\nname: uap-user-docs\ndescription: Inert User fixture\n---\n" + content + "\n"
	projectJourneyWrite(t, filepath.Join(transport.repo, "skills", "uap-user-docs", "SKILL.md"), skill)
	projectJourneyWrite(t, filepath.Join(transport.repo, "mcp.json"), `{"$schema":"https://agent-plugins.org/schemas/1.0.0/mcp.schema.json","mcpServers":{"uap_user_probe":{"type":"stdio","command":"go","args":["version"],"env":{"DATA":"${PLUGIN_DATA}"}}}}`)
	projectJourneyGit(t, transport.repo, "add", ".")
	projectJourneyGit(t, transport.repo, "commit", "-m", "select inert User components")
	projectJourneyGit(t, transport.repo, "push", "--quiet", transport.remote, "main")
	archive, archiveErr := exec.CommandContext(t.Context(), "git", "-C", transport.repo, "archive", "--format=zip", "--prefix=provider-wrapper/", "HEAD").Output()
	if archiveErr != nil || len(archive) > 1<<20 {
		t.Fatal("create bounded owned User archive", archiveErr)
	}
	transport.mu.Lock()
	transport.archive = archive
	transport.mu.Unlock()
	return skill
}
