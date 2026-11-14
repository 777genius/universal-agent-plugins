package main

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"syscall"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// Real public CLI/state/filesystem composition and TLS Git/ZIP acquisition.
// Discovery is inert and every Devin/MCP process is rejected. These journeys
// prove Prepared file delivery, not native inventory, enablement or login.
func TestProjectDevinComposedSourceLifecycle(t *testing.T) {
	for _, kind := range []string{"git", "zip"} {
		t.Run(kind, func(t *testing.T) {
			transport := newProjectJourneyTransport(t, kind)
			devinCommitSource(t, transport, projectJourneySkill, "https://managed.test/v1", true)
			f := newDevinJourney(t, transport, true, false)
			userBefore := securityFixtureTree(t, f.devinRoot(""))
			for _, root := range []string{f.a, f.b} {
				f.run(t, root, "add", transport.endpoint, "--alias", "docs", "--target", "windsurf", "--yes")
				installation, _ := f.assertDevinOwned(t, root, projectJourneySkill, "https://managed.test/v1")
				f.rememberDevinData(t, root, installation)
			}
			bNative := securityFixtureTree(t, f.devinRoot(f.b))
			bState := projectJourneyRead(t, filepath.Join(f.namespace(f.b), "state-v2.json"))
			f.preserveDevin(t, f.devinRoot(f.b), filepath.Join(f.namespace(f.b), "state-v2.json"))
			t.Run("project", func(t *testing.T) { devinPreparedSourceLifecycle(t, f, f.a) })
			if !reflect.DeepEqual(userBefore, securityFixtureTree(t, f.devinRoot(""))) || !reflect.DeepEqual(bNative, securityFixtureTree(t, f.devinRoot(f.b))) || bState != projectJourneyRead(t, filepath.Join(f.namespace(f.b), "state-v2.json")) {
				t.Fatal("Devin project-A lifecycle changed original User or project B")
			}
			f.assertDevinOwned(t, f.b, projectJourneySkill, "https://managed.test/v1")
			t.Run("user", func(t *testing.T) {
				// Independent original User/A/B authority, never a reset of the
				// Project world's protected User profile.
				devinCommitSource(t, transport, projectJourneySkill, "https://managed.test/v1", true)
				user := newDevinJourney(t, transport, true, true)
				user.run(t, "", "add", transport.endpoint, "--alias", "docs", "--target", "windsurf", "--scope", "user", "--yes")
				installation, _ := user.assertDevinOwned(t, "", projectJourneySkill, "https://managed.test/v1")
				user.rememberDevinData(t, "", installation)
				devinPreparedSourceLifecycle(t, user, "")
			})
			if err := f.checkDevinProtection(t); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func devinPreparedSourceLifecycle(t *testing.T, f devinLifecycleJourney, project string) {
	t.Helper()
	original, binding := f.assertDevinOwned(t, project, projectJourneySkill, "https://managed.test/v1")
	f.run(t, project, "add", f.transport.endpoint, "--alias", "docs", "--target", "windsurf", "--yes")
	repeated, repeatBinding := f.assertDevinOwned(t, project, projectJourneySkill, "https://managed.test/v1")
	if !reflect.DeepEqual(original, repeated) || !reflect.DeepEqual(binding, repeatBinding) {
		t.Fatal("saved Devin repeat changed original installation/binding authority")
	}
	devinReconcileAndRepair(t, f, project, original, binding)
	devinCommitSource(t, f.transport, projectJourneyAdvancedSkill, "https://managed.test/v2", true)
	f.run(t, project, "update", "docs", "--target", "windsurf", "--yes")
	updated, selected := f.assertDevinOwned(t, project, projectJourneyAdvancedSkill, "https://managed.test/v2")
	devinAssertContinuity(t, original, updated, binding, selected, true)
	statePath := filepath.Join(f.namespace(project), "state-v2.json")
	beforeState, beforeNative := projectJourneyRead(t, statePath), securityFixtureTree(t, f.devinRoot(project))
	t.Logf("declared missing managed-package fault; scope=%q, process_calls=%d", selected.Scope, f.native.calls)
	devinQuarantineOwned(t, f, selected, selected.TargetLocator)
	if beforeState != projectJourneyRead(t, statePath) || !reflect.DeepEqual(beforeNative, securityFixtureTree(t, f.devinRoot(project))) || f.native.calls != 0 {
		t.Fatal("declared package-absence fault changed original receipts or native MCP/Skill projection")
	}
	f.run(t, project, "repair", updated.InstallationID, "--target", "windsurf", "--yes")
	repaired, repairedBinding := f.assertDevinOwned(t, project, projectJourneyAdvancedSkill, "https://managed.test/v2")
	devinAssertContinuity(t, updated, repaired, selected, repairedBinding, false)
	devinTransfer(t, f, project, repaired)
	devinRemoveRetainingData(t, f, project, repaired, repairedBinding)
}

func devinReconcileAndRepair(t *testing.T, f devinLifecycleJourney, project string, original domain.Installation, binding domain.ClientBinding) {
	t.Helper()
	statePath := filepath.Join(f.namespace(project), "state-v2.json")
	before := projectJourneyRead(t, statePath)
	f.run(t, project, "deps", "sync", "--locked", "--yes")
	if before != projectJourneyRead(t, statePath) {
		t.Fatal("Devin locked no-op changed exact ownership receipts")
	}
	native := securityFixtureTree(t, f.devinRoot(project))
	for _, args := range [][]string{{"list"}, {"info", original.InstallationID}, {"doctor", original.InstallationID}, {"outdated", original.InstallationID}, {"deps", "status"}, {"deps", "tree"}} {
		f.run(t, project, args...)
	}
	if before != projectJourneyRead(t, statePath) || !reflect.DeepEqual(native, securityFixtureTree(t, f.devinRoot(project))) {
		t.Fatal("Devin read commands changed native files or ownership receipts")
	}
	// Preserve the original missing-native-Skill repair case in the existing
	// Project fixture; the full managed-package fault follows the changed update.
	if project != "" {
		t.Logf("declared missing native-Skill fault; scope=%q, process_calls=%d", binding.Scope, f.native.calls)
		devinQuarantineOwned(t, f, binding, filepath.Join(f.devinRoot(project), "skills", "docs"))
		f.run(t, project, "repair", original.InstallationID, "--target", "windsurf", "--yes")
		repaired, selected := f.assertDevinOwned(t, project, projectJourneySkill, "https://managed.test/v1")
		devinAssertContinuity(t, original, repaired, binding, selected, false)
		f.inspectNoRecovery(t, project)
	}
}

func devinSourceContinues(original, current domain.SourceBinding, advance bool) bool {
	expected := original
	if advance {
		if current.TreeDigest == original.TreeDigest || current.ResolvedRevision == original.ResolvedRevision {
			return false
		}
		expected.TreeDigest, expected.ResolvedRevision = current.TreeDigest, current.ResolvedRevision
		if original.SourceKind == "archive" {
			expected.ArchiveSHA256 = current.ArchiveSHA256
			if current.ResolvedRevision != "sha256:"+current.ArchiveSHA256 {
				return false
			}
		}
	}
	return current == expected
}

func devinAssertContinuity(t *testing.T, original, current domain.Installation, before, after domain.ClientBinding, advance bool) {
	t.Helper()
	if original.InstallationID != current.InstallationID || original.DeclaredName != current.DeclaredName || original.CreatedAt != current.CreatedAt ||
		!devinSourceContinues(original.Source, current.Source, advance) || !reflect.DeepEqual(original.DataReceipts, current.DataReceipts) ||
		!reflect.DeepEqual(original.InstallPreferences, current.InstallPreferences) || before.ClientBindingID != after.ClientBindingID || before.ClientID != after.ClientID || before.Scope != after.Scope ||
		before.TargetLocator != after.TargetLocator || before.PhysicalArtifact != after.PhysicalArtifact || before.NativeProfileRoot != after.NativeProfileRoot || before.DataReceiptID != after.DataReceiptID ||
		before.InstallIntent != after.InstallIntent || before.Authentication != after.Authentication || !reflect.DeepEqual(before.SelectedDelivery, after.SelectedDelivery) {
		t.Fatal("Devin changed original source/binding/data authority")
	}
	if !advance && !reflect.DeepEqual(original.Package, current.Package) {
		t.Fatal("Devin repair changed the accepted package revision")
	}
}

func (f devinLifecycleJourney) rememberDevinData(t *testing.T, project string, installation domain.Installation) {
	t.Helper()
	// assertDevinOwned joined source, directory digest, native projection and
	// the production data owner marker before this original marker is authored.
	projectJourneyDataMarker(t, f.projectJourney, project, installation)
	for _, receipt := range installation.DataReceipts {
		f.preserveDevin(t, receipt.Locator)
	}
}

func devinQuarantineOwned(t *testing.T, f devinLifecycleJourney, binding domain.ClientBinding, source string) {
	t.Helper()
	owned := false
	for _, object := range binding.NativeObjects {
		if object.Path == source && object.ProtectionClass == "managed" && !object.UserModified &&
			(object.Kind == "managed_package_directory" || strings.HasSuffix(object.Kind, "_skill_directory")) {
			owned = true
		}
	}
	if !owned {
		t.Fatal("quarantine lacks exact owned directory receipt")
	}
	parent, err := os.OpenRoot(filepath.Dir(source))
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
	heldLeaf, err := parent.OpenRoot(filepath.Base(source))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := heldLeaf.Close(); err != nil {
			t.Error(err)
		}
	})
	original, err := heldLeaf.Stat(".")
	if err != nil || !original.IsDir() {
		t.Fatal("quarantine source is not an original ordinary directory", err)
	}
	// A sibling beneath the managed plugin root would itself remain a
	// same-manifest registry collision, correctly preventing repair.
	target := filepath.Join(f.home, "deliberately-quarantined-"+filepath.Base(source))
	destination, err := os.OpenRoot(f.home)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := destination.Close(); err != nil {
			t.Error(err)
		}
	})
	destinationIdentity, err := destination.Stat(".")
	if err != nil {
		t.Fatal(err)
	}
	projectJourneyAbsent(t, target)
	current, err := os.Lstat(f.home)
	if err != nil || !os.SameFile(destinationIdentity, current) {
		t.Fatal("quarantine destination parent changed", err)
	}
	current, err = os.Lstat(filepath.Dir(source))
	if err != nil || !os.SameFile(parentIdentity, current) {
		t.Fatal("quarantine parent changed", err)
	}
	current, err = parent.Lstat(filepath.Base(source))
	if err != nil || !os.SameFile(original, current) || current.Mode() != original.Mode() {
		t.Fatal("quarantine source changed", err)
	}
	if err := os.Rename(source, target); err != nil {
		t.Fatal(err)
	}
	current, err = os.Lstat(f.home)
	if err != nil || !os.SameFile(destinationIdentity, current) {
		t.Fatal("quarantine destination parent changed during move", err)
	}
	current, err = os.Lstat(filepath.Dir(source))
	if err != nil || !os.SameFile(parentIdentity, current) {
		t.Fatal("quarantine parent changed during move", err)
	}
	current, err = destination.Lstat(filepath.Base(target))
	if err != nil || !os.SameFile(original, current) || original.Mode() != current.Mode() {
		t.Fatal("quarantine lost original directory identity", err)
	}
	projectJourneyAbsent(t, source)
	f.preserveDevin(t, target)
}

func devinTransfer(t *testing.T, f devinLifecycleJourney, project string, accepted domain.Installation) {
	t.Helper()
	root := project
	if root == "" {
		root = f.cache
	}
	pair := f.assertPortablePair(t, root)
	exchange := devinJourneyTemp(t)
	f.run(t, project, "deps", "export", exchange, "--yes")
	if pair != f.assertPortablePair(t, exchange) {
		t.Fatal("Devin export changed exact portable pins")
	}
	cold := newDevinJourney(t, f.transport, true, project == "")
	selected := cold.a
	if project == "" {
		selected = ""
	} else {
		cold.preserveDevin(t, cold.devinRoot(cold.b))
	}
	projectJourneyAbsent(t, cold.cache, filepath.Join(cold.a, ".uap"))
	cold.run(t, selected, "deps", "import", exchange, "--install", "--yes")
	restored, binding := cold.assertDevinOwned(t, selected, projectJourneyAdvancedSkill, "https://managed.test/v2")
	portableRoot := selected
	if portableRoot == "" {
		portableRoot = cold.cache
	}
	if restored.Source != accepted.Source || pair != cold.assertPortablePair(t, portableRoot) {
		t.Fatal("Devin cold import changed complete source identity or portable pins")
	}
	for _, receipt := range restored.DataReceipts {
		for _, original := range accepted.DataReceipts {
			if receipt.Locator == original.Locator {
				t.Fatal("cold Devin restore reused another world's data authority")
			}
		}
	}
	cold.rememberDevinData(t, selected, restored)
	cold.run(t, selected, "deps", "sync", "--locked", "--yes")
	repeated, repeatedBinding := cold.assertDevinOwned(t, selected, projectJourneyAdvancedSkill, "https://managed.test/v2")
	devinAssertContinuity(t, restored, repeated, binding, repeatedBinding, false)
	devinRemoveRetainingData(t, cold, selected, restored, repeatedBinding)
}

func devinRemoveRetainingData(t *testing.T, f devinLifecycleJourney, project string, original domain.Installation, binding domain.ClientBinding) {
	t.Helper()
	f.run(t, project, "remove", "docs", "--target", "windsurf", "--yes")
	projectJourneyAbsent(t, binding.TargetLocator, filepath.Join(f.devinRoot(project), "skills", "docs"))
	settings := projectJourneyRead(t, filepath.Join(f.devinRoot(project), "mcp_config.json"))
	if strings.Contains(settings, "managed.test") || strings.Contains(settings, `"docs_data"`) || !strings.Contains(settings, "foreign.test") || !strings.Contains(settings, "foreignPreference") {
		t.Fatal("Devin removal changed foreign MCP settings or retained selected entries")
	}
	state := f.state(t, project)
	if len(state.Installations) != 1 || state.Installations[0].InstallationID != original.InstallationID || state.Installations[0].Source != original.Source ||
		len(state.Installations[0].Clients) != 0 || !state.Installations[0].DataRetained || !reflect.DeepEqual(state.Installations[0].DataReceipts, original.DataReceipts) || len(state.EnvironmentReceipts) != 0 {
		t.Fatal("Devin removal lost original retained-data authority")
	}
	if err := f.checkDevinProtection(t); err != nil {
		t.Fatal(err)
	}
}

func TestProjectDevinContinuityRejectsDetachedSourceAndSameByteReplacement(t *testing.T) {
	original := domain.SourceBinding{SourceBindingID: "original-id", TreeDigest: "original-tree", CanonicalSource: "original-canonical", RequestedSource: "original-request"}
	changed := original
	changed.CanonicalSource = "foreign-canonical"
	if original.SourceBindingID != changed.SourceBindingID || original.TreeDigest != changed.TreeDigest {
		t.Fatal("counterexample no longer passes the previous partial source oracle")
	}
	if devinSourceContinues(original, changed, false) {
		t.Fatal("full source guard accepted detached canonical authority")
	}
	root := devinJourneyTemp(t)
	path := filepath.Join(root, "keep.txt")
	projectJourneyWrite(t, path, "retained original bytes\n")
	f := devinLifecycleJourney{native: &devinNativeRejectCounter{}, protected: map[string]devinProtectedTree{}, foreign: map[string]map[string]any{}}
	f.preserveDevin(t, path)
	before := securityFixtureTree(t, path)
	if err := os.Rename(path, filepath.Join(root, "original.txt")); err != nil {
		if runtime.GOOS != "windows" || !errors.Is(err, syscall.Errno(32)) || f.checkDevinProtection(t) != nil {
			t.Fatal("unexpected marker replacement refusal", err)
		}
		return // The original held Windows file forbids deletion/replacement.
	}
	projectJourneyWrite(t, path, "retained original bytes\n")
	if !reflect.DeepEqual(before, securityFixtureTree(t, path)) {
		t.Fatal("counterexample no longer passes the previous byte-only oracle")
	}
	if err := f.checkDevinProtection(t); err == nil || !strings.Contains(err.Error(), "identity changed") || f.native.calls != 0 {
		t.Fatal("original marker identity guard accepted a same-byte replacement", err)
	}
}
