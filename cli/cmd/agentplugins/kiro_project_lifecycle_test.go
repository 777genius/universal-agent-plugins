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
				installed, _ := f.assertKiroOwned(t, root, projectJourneySkill, "https://managed.test/v1")
				f.rememberKiroData(t, root, installed)
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
	installation, binding := f.owned(t, f.a)
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
	repaired, selected := f.assertKiroOwned(t, f.a, projectJourneySkill, "https://managed.test/v1")
	if !kiroContinues(installation, repaired, binding, selected, false) {
		t.Fatal("Kiro native-Skill repair changed accepted source/binding/data authority")
	}
	f.inspectNoRecovery(t, f.a)
}

func kiroProjectUpdateAndTransfer(t *testing.T, f kiroProjectJourney) {
	t.Helper()
	previous, binding := f.owned(t, f.a)
	kiroProjectCommitSource(t, f.transport, projectJourneyAdvancedSkill, "https://managed.test/v2")
	f.run(t, f.a, "update", "docs", "--target", "kiro")
	updated, selected := f.assertKiroOwned(t, f.a, projectJourneyAdvancedSkill, "https://managed.test/v2")
	if !kiroContinues(previous, updated, binding, selected, true) {
		t.Fatal("Kiro update did not retain source/binding authority while advancing the pin")
	}
	kiroQuarantineOwnedPackage(t, f, selected)
	f.run(t, f.a, "repair", updated.InstallationID, "--target", "kiro")
	repaired, repairedBinding := f.assertKiroOwned(t, f.a, projectJourneyAdvancedSkill, "https://managed.test/v2")
	if !kiroContinues(updated, repaired, selected, repairedBinding, false) {
		t.Fatal("Kiro missing-package repair changed the accepted source or data authority")
	}
	updated = repaired
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
	restored, restoredBinding := cold.assertKiroOwned(t, cold.a, projectJourneyAdvancedSkill, "https://managed.test/v2")
	if !kiroColdIdentity(updated, restored) || pair != cold.assertPortablePair(t, cold.a) {
		t.Fatal("Kiro cold import changed source identity or portable pins")
	}
	cold.rememberKiroData(t, cold.a, restored)
	cold.run(t, cold.a, "deps", "sync", "--locked", "--yes")
	repeated, repeatedBinding := cold.assertKiroOwned(t, cold.a, projectJourneyAdvancedSkill, "https://managed.test/v2")
	if !kiroContinues(restored, repeated, restoredBinding, repeatedBinding, false) {
		t.Fatal("Kiro cold locked repeat changed original installation or data authority")
	}
	cold.inspectNoRecovery(t, cold.a)
	kiroProjectRemove(t, cold)
}

func kiroProjectRemove(t *testing.T, f kiroProjectJourney) {
	t.Helper()
	installed, binding := f.owned(t, f.a)
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
	kiroAssertRetained(t, f.state(t, f.a), installed)
}

// A Run-only rejecting runner exercises the public prepared file lifecycle.
// It does not certify native Kiro startup, login or MCP initialization.
func kiroUserPreparedSourceLifecycle(t *testing.T, transport *projectJourneyTransport) {
	t.Helper()
	skill := kiroUserCommitSource(t, transport, "original User pin")
	f := newKiroJourney(t, transport, true)
	f.run(t, "", "add", transport.endpoint, "--alias", "user-docs", "--target", "kiro", "--scope", "user", "--yes")
	original, binding := f.assertKiroUserOwned(t, skill)
	f.rememberKiroData(t, "", original)
	f.run(t, "", "add", transport.endpoint, "--alias", "user-docs", "--target", "kiro", "--scope", "user", "--yes")
	repeated, repeatedBinding := f.assertKiroUserOwned(t, skill)
	if !reflect.DeepEqual(original, repeated) || !reflect.DeepEqual(binding, repeatedBinding) {
		t.Fatal("Kiro saved User repeat changed original installation or binding authority")
	}
	updated, selected, updatedSkill := kiroUserUpdateAndRepair(t, f, original, binding)
	kiroUserTransfer(t, f, updated, updatedSkill)
	f.run(t, "", "remove", "user-docs", "--target", "kiro", "--scope", "user", "--yes")
	projectJourneyAbsent(t, selected.TargetLocator, filepath.Join(f.kiroRoot(""), "skills", "uap-user-docs"))
	kiroAssertRetained(t, f.state(t, ""), updated)
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
	if !kiroContinues(original, updated, binding, selected, true) {
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
	if !kiroContinues(updated, repaired, selected, repairedBinding, false) {
		t.Fatal("Kiro User repair changed the last accepted revision or original data authority")
	}
	kiroQuarantineOwnedPackage(t, f, repairedBinding)
	f.run(t, "", "repair", repaired.InstallationID, "--target", "kiro", "--scope", "user", "--yes")
	full, fullBinding := f.assertKiroUserOwned(t, skill)
	if !kiroContinues(repaired, full, repairedBinding, fullBinding, false) {
		t.Fatal("Kiro User missing-package repair changed original source/binding/data authority")
	}
	return full, fullBinding, skill
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
	restored, binding := cold.assertKiroUserOwned(t, skill)
	if !kiroColdIdentity(updated, restored) || pair != cold.assertPortablePair(t, cold.cache) {
		t.Fatal("Kiro cold User import changed selected source or agreement pins")
	}
	cold.rememberKiroData(t, "", restored)
	cold.run(t, "", "deps", "sync", "--locked", "--scope", "user", "--yes")
	repeated, repeatedBinding := cold.assertKiroUserOwned(t, skill)
	if !kiroContinues(restored, repeated, binding, repeatedBinding, false) {
		t.Fatal("Kiro cold User repeat changed its original source/binding/data authority")
	}
	cold.run(t, "", "remove", "user-docs", "--target", "kiro", "--scope", "user", "--yes")
	projectJourneyAbsent(t, repeatedBinding.TargetLocator, filepath.Join(cold.kiroRoot(""), "skills", "uap-user-docs"))
	kiroAssertRetained(t, cold.state(t, ""), repeated)
	cold.assertProtected(t)
}

func kiroContinues(original, current domain.Installation, before, after domain.ClientBinding, advance bool) bool {
	expected := original
	if len(after.Receipts) < len(before.Receipts) || !reflect.DeepEqual(before.Receipts, after.Receipts[:len(before.Receipts)]) {
		return false
	}
	if advance {
		if current.Source.TreeDigest == original.Source.TreeDigest || current.Source.ResolvedRevision == original.Source.ResolvedRevision ||
			before.PackageRevision == nil || after.PackageRevision == nil || len(before.NativeObjects) != len(after.NativeObjects) {
			return false
		}
		expected.Source.TreeDigest, expected.Source.ResolvedRevision = current.Source.TreeDigest, current.Source.ResolvedRevision
		if original.Source.SourceKind == "archive" {
			expected.Source.ArchiveSHA256 = current.Source.ArchiveSHA256
			if current.Source.ResolvedRevision != "sha256:"+current.Source.ArchiveSHA256 {
				return false
			}
		}
		revision := *before.PackageRevision
		revision.TreeDigest, revision.ResolvedRevision = current.Source.TreeDigest, current.Source.ResolvedRevision
		before.PackageRevision = &revision
		before.NativeObjects = append([]domain.NativeObjectOwnership(nil), before.NativeObjects...)
		for i := range before.NativeObjects {
			// assertKiroOwned joins these digests to the actual installed files.
			// The update changes authored content, never an object's identity.
			before.NativeObjects[i].ManagedDigest = after.NativeObjects[i].ManagedDigest
		}
	}
	// Only per-operation timestamps/group and appended history may advance.
	// The fixture's manifest stays identical across its authored source update.
	expected.UpdatedAt, expected.OperationGroupID, expected.Clients = current.UpdatedAt, current.OperationGroupID, current.Clients
	before.UpdatedAt, before.Receipts = after.UpdatedAt, after.Receipts
	return reflect.DeepEqual(expected, current) && reflect.DeepEqual(before, after)
}

func kiroColdIdentity(original, restored domain.Installation) bool {
	if restored.InstallationID == original.InstallationID || restored.Source != original.Source || !reflect.DeepEqual(restored.Package, original.Package) ||
		len(original.DataReceipts) != 1 || len(restored.DataReceipts) != 1 {
		return false
	}
	for _, receipt := range restored.DataReceipts {
		for _, previous := range original.DataReceipts {
			if receipt.Locator == previous.Locator || receipt.DataReceiptID == previous.DataReceiptID {
				return false
			}
		}
	}
	return true
}

func kiroAssertRetained(t *testing.T, state domain.StateFileV2, original domain.Installation) {
	t.Helper()
	if len(state.Installations) != 1 || len(state.Installations[0].Clients) != 0 || !state.Installations[0].DataRetained || len(state.EnvironmentReceipts) != 0 {
		t.Fatal("Kiro remove lost its retained installation or left an active binding")
	}
	current := state.Installations[0]
	for _, binding := range original.Clients {
		if binding.Scope == string(domain.ScopeUser) {
			if binding.InstallIntent != domain.InstallIntentPrepare {
				t.Fatal("Kiro User fixture lost its original explicit preparation decision")
			}
			// Removing a prepared binding retains exactly this reinstall choice.
			original.InstallPreferences = []domain.InstallPreference{{ClientID: domain.ClientKiro, Scope: domain.ScopeUser, InstallIntent: domain.InstallIntentPrepare}}
		}
	}
	original.UpdatedAt, original.OperationGroupID = current.UpdatedAt, current.OperationGroupID
	original.Clients, original.DataRetained = current.Clients, true
	if !reflect.DeepEqual(original, current) {
		t.Fatal("Kiro remove changed the accepted source/package or retained data authority")
	}
}

func kiroQuarantineOwnedPackage(t *testing.T, f kiroProjectJourney, binding domain.ClientBinding) {
	t.Helper()
	digest := ""
	for _, object := range binding.NativeObjects {
		if object.Kind == "managed_package_directory" && object.Path == binding.TargetLocator && object.ProtectionClass == "managed" && !object.UserModified {
			digest = object.ManagedDigest
		}
	}
	if digest == "" {
		t.Fatal("missing-package fault requires the exact verified owned receipt")
	}
	if err := f.app.Lifecycle.Stager.Verify(t.Context(), binding.TargetLocator, digest); err != nil {
		t.Fatal(err)
	}
	destinationParent := projectJourneyTemp(t)
	destination := filepath.Join(destinationParent, "kiro-owned-package")
	parent, err := os.OpenRoot(filepath.Dir(binding.TargetLocator))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := parent.Close(); err != nil {
			t.Error(err)
		}
	})
	parentIdentity, err := parent.Stat(".")
	if err != nil {
		t.Fatal(err)
	}
	// Parent.OpenRoot uses a rename-compatible directory handle on Windows.
	// Keep the original leaf open; closing it would discard the authority.
	held, err := parent.OpenRoot(filepath.Base(binding.TargetLocator))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := held.Close(); err != nil {
			t.Error(err)
		}
	})
	original, err := held.Stat(".")
	if err != nil || !original.IsDir() {
		t.Fatal("Kiro owned package handle is not a directory", err)
	}
	to, err := os.OpenRoot(destinationParent)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := to.Close(); err != nil {
			t.Error(err)
		}
	})
	destinationIdentity, err := to.Stat(".")
	if err != nil {
		t.Fatal(err)
	}
	named, err := parent.Lstat(filepath.Base(binding.TargetLocator))
	if err != nil || !os.SameFile(original, named) || named.Mode()&os.ModeSymlink != 0 {
		t.Fatal("Kiro owned package changed before declared missing-package fault", err)
	}
	for path, identity := range map[string]os.FileInfo{filepath.Dir(binding.TargetLocator): parentIdentity, destinationParent: destinationIdentity} {
		current, err := os.Lstat(path)
		if err != nil || !os.SameFile(identity, current) || identity.Mode() != current.Mode() {
			t.Fatal("Kiro quarantine parent changed before its declared fault", err)
		}
	}
	if err := os.Rename(binding.TargetLocator, destination); err != nil {
		t.Fatal(err)
	}
	for path, identity := range map[string]os.FileInfo{filepath.Dir(binding.TargetLocator): parentIdentity, destinationParent: destinationIdentity} {
		current, err := os.Lstat(path)
		if err != nil || !os.SameFile(identity, current) || identity.Mode() != current.Mode() {
			t.Fatal("Kiro quarantine parent changed during its declared fault", err)
		}
	}
	moved, err := to.Lstat(filepath.Base(destination))
	if err != nil || !os.SameFile(original, moved) || original.Mode() != moved.Mode() {
		t.Fatal("Kiro fault did not retain the original owned package", err)
	}
	projectJourneyAbsent(t, binding.TargetLocator)
	f.preserve(t, destination)
}

func TestKiroContinuityRejectsDetachedAuthority(t *testing.T) {
	before := domain.ClientBinding{ClientBindingID: "binding", ClientID: "kiro", Scope: "project", DataReceiptID: "data",
		Receipts: []domain.MutationReceipt{{OperationID: "original", Phase: "committed"}}}
	original := domain.Installation{InstallationID: "original", Source: domain.SourceBinding{SourceBindingID: "source", CanonicalSource: "original-source"},
		DataReceipts: map[string]domain.DataReceipt{"data": {DataReceiptID: "data", Locator: "original-data", OwnershipDigest: "owner"}}}
	for _, change := range []string{"unchanged", "appended-receipt", "source", "installation", "binding", "data", "receipt-history"} {
		t.Run(change, func(t *testing.T) {
			current, after := original, before
			switch change {
			case "appended-receipt":
				after.Receipts = append(append([]domain.MutationReceipt(nil), before.Receipts...), domain.MutationReceipt{OperationID: "new", Phase: "committed"})
			case "source":
				current.Source.CanonicalSource = "unrelated"
			case "installation":
				current.InstallationID = "replacement"
			case "binding":
				after.ProfileNamespace = "different-namespace"
			case "data":
				receipt := original.DataReceipts["data"]
				receipt.OwnershipDigest = "replacement-owner"
				current.DataReceipts = map[string]domain.DataReceipt{"data": receipt}
			case "receipt-history":
				after.Receipts = []domain.MutationReceipt{{OperationID: "replacement", Phase: "committed"}}
			}
			if kiroContinues(original, current, before, after, false) != (change == "unchanged" || change == "appended-receipt") {
				t.Fatal("Kiro continuity accepted detached original authority")
			}
		})
	}
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
