//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/conformance"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/packagesnapshot"
)

const copilotRawSkill = "benchmark-note"
const copilotRawPackage = "skill-4e68c33bfb91dc370d5c3745c0ba746428ff227ed3b3466f9c9d1552"

// This additional case uses the existing genuine pinned executable, scanner,
// original150s attempt and checked command captures. It never arms a provider.
func testCopilotNativeRawProjectSkills(t *testing.T) {
	t.Helper()
	t.Setenv("AGENTPLUGINS_PROJECT_NATIVE_OUTPUT", t.TempDir())
	f := newCopilotProjectAttempt(t)
	defer f.cancel()
	defer copilotRawLogEvidence(t, f)
	f.preparePackage(t)
	foreign := copilotRawForeign(t, f)
	before := copilotRawInventories(t, f, "before", nil)
	eventsExist, events, err := copilotRawEventSnapshot(f.base.Events)
	if err != nil {
		t.Fatal("raw initial MCP event snapshot refused", err)
	}
	argv := []string{f.installer, "skills", "add", nativeRawProjectSkillsSource, "--skill", copilotRawSkill, "--target", "copilot", "--scope", "project", "--project-root", f.base.Project, "--yes", "--no-ui", "--format", "json"}
	first := f.command(t, "raw-add", f.outside, f.env(), argv, 45*time.Second)
	copilotRawReady(t, first.Stdout, false)
	installed, binding := copilotRawProof(t, f, "add", "")
	if !reflect.DeepEqual(before, copilotRawInventories(t, f, "add", &binding)) {
		t.Fatal("add changed foreign catalogs")
	}
	stable := copilotRawSnapshot(t, f, binding)
	repeat := f.command(t, "raw-repeat", f.outside, f.env(), argv, 45*time.Second)
	copilotRawReady(t, repeat.Stdout, true)
	replay, replayBinding := copilotRawProof(t, f, "repeat", installed.InstallationID)
	afterRepeat := copilotRawSnapshot(t, f, binding)
	continuity, _ := json.Marshal(map[string]any{"before": stable, "after": afterRepeat})
	f.keep(t, "raw-repeat-snapshot.json", continuity)
	if !reflect.DeepEqual(installed, replay) || !reflect.DeepEqual(binding, replayBinding) || !reflect.DeepEqual(stable, afterRepeat) {
		t.Fatal("raw repeat changed IID/state/lock/owned package/registration")
	}
	if !reflect.DeepEqual(before, copilotRawInventories(t, f, "repeat", &binding)) {
		t.Fatal("repeat changed foreign catalogs")
	}
	removed := f.command(t, "raw-remove", f.outside, f.env(), []string{f.installer, "remove", installed.InstallationID, "--target", "copilot", "--scope", "project", "--project-root", f.base.Project, "--yes", "--no-ui", "--format", "json"}, 45*time.Second)
	value, err := projectNativeRuntimeJSON(removed.Stdout)
	doc, ok := value.(map[string]any)
	if err != nil || !ok || doc["schema_version"] != json.Number("1") || doc["command"] != "agentplugins remove" || doc["result"] != "success" {
		t.Fatal("raw removal DTO differs", err)
	}
	after := copilotRawInventories(t, f, "remove", nil)
	if !reflect.DeepEqual(before, after) {
		t.Fatal("raw removal changed foreign native inventories")
	}
	for path, want := range foreign {
		if nativeTreeDigest(t, path) != want {
			t.Fatal("raw lifecycle changed foreign tree", path)
		}
	}
	state := copilotRawRead(t, filepath.Join(f.base.Project, ".uap", "state-v2.json"))
	f.keep(t, "raw-remove-proof.json", state)
	var final domain.StateFileV2
	if err := json.Unmarshal(state, &final); err != nil {
		t.Fatal("raw retained state could not be decoded", err)
	}
	copilotRawRetainedData(t, f, final, installed, binding)
	if _, err := os.Lstat(binding.TargetLocator); !os.IsNotExist(err) {
		t.Fatal("raw removed package remains", err)
	}
	afterEventsExist, afterEvents, err := copilotRawEventSnapshot(f.base.Events)
	if err != nil || eventsExist != afterEventsExist || !bytes.Equal(events, afterEvents) {
		t.Fatal("ordinary catalog mutated MCP probe events", err)
	}
	f.check(t)
}

/*
copilotRawEventSnapshot distinguishes absent output from an empty native event
file. This no-provider journey must not create, replace or modify events.
The shared bounded owner/regular-file read rejects unsafe existing leaves.
*/
func copilotRawEventSnapshot(path string) (bool, []byte, error) {
	return copilotRawEventSnapshotRead(path, projectNativeRuntimeRead)
}

// Only a missing leaf observed before a held read is ordinary absence. A file
// disappearing during the nofollow read must remain a failure, not a no-op.
func copilotRawEventSnapshotRead(path string, read func(string, int64) ([]byte, error)) (bool, []byte, error) {
	if err := projectNativeRuntimeParents(path); err != nil {
		return false, nil, err
	}
	if _, err := os.Lstat(path); err != nil {
		if !os.IsNotExist(err) {
			return false, nil, err
		}
		if parentErr := projectNativeRuntimeParents(path); parentErr != nil {
			return false, nil, parentErr
		}
		return false, nil, nil
	}
	body, err := read(path, 1<<20)
	if err != nil {
		return false, nil, err
	}
	return true, body, nil
}

func TestCopilotRawEventSnapshotPreservesAbsenceAndRefusesAliases(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "events.jsonl")
	exists, body, err := copilotRawEventSnapshot(path)
	if err != nil || exists || len(body) != 0 {
		t.Fatal("uncreated event file must remain absent", err)
	}
	// Losing the parent is foreign fixture drift, not an absent event leaf.
	if _, _, err := copilotRawEventSnapshot(filepath.Join(root, "missing-parent", "events.jsonl")); !os.IsNotExist(err) {
		t.Fatal("missing event parent was accepted as an absent leaf", err)
	}
	if err := os.WriteFile(path, nil, 0600); err != nil {
		t.Fatal(err)
	}
	exists, body, err = copilotRawEventSnapshot(path)
	if err != nil || !exists || len(body) != 0 {
		t.Fatal("present empty event file must not resemble absence", err)
	}
	if err := os.WriteFile(path, []byte("event\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := copilotRawEventSnapshotRead(path, func(string, int64) ([]byte, error) {
		return nil, os.ErrNotExist
	}); err == nil {
		t.Fatal("event leaf disappearance during protected read was accepted as no events")
	}
	exists, body, err = copilotRawEventSnapshot(path)
	if err != nil || !exists || string(body) != "event\n" {
		t.Fatal("existing event bytes must remain observable", err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "foreign-events"), path); err != nil {
		t.Fatal(err)
	}
	if _, _, err := copilotRawEventSnapshot(path); err == nil {
		t.Fatal("event snapshot must refuse symlinked leaf")
	}
}

func copilotRawLogEvidence(t *testing.T, f *copilotProjectAttempt) {
	t.Helper()
	names := make([]string, 0, len(f.files))
	for name := range f.files {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		body := f.files[name]
		if !strings.HasPrefix(name, "raw-") {
			continue
		}
		// Each one-Skill DTO, exact proof and ordinary inventory must be complete
		// in the frontend artifact, not just an opaque or truncated success flag.
		if len(body) > 16<<10 {
			t.Errorf("raw Copilot evidence exceeds existing complete log bound: %s", name)
			continue
		}
		t.Logf("raw Copilot %s (%d bytes): %q", name, len(body), body)
	}
}

func copilotRawForeign(t *testing.T, f *copilotProjectAttempt) map[string]string {
	t.Helper()
	result := map[string]string{}
	for key, root := range map[string]string{"a": filepath.Join(f.base.Project, ".github", "skills"), "b": filepath.Join(f.other, ".github", "skills"), "user": filepath.Join(f.base.Home, ".copilot", "skills")} {
		name := "uap-raw-foreign-" + key
		projectNativeRuntimeWrite(t, filepath.Join(root, name, "SKILL.md"), []byte("---\nname: "+name+"\ndescription: Inert retained foreign Skill\n---\nForeign fixture.\n"), 0600)
		result[root] = nativeTreeDigest(t, root)
	}
	return result
}

func copilotRawRead(t *testing.T, path string) []byte {
	t.Helper()
	body, err := projectNativeRuntimeRead(path, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func copilotRawReady(t *testing.T, body []byte, repeated bool) {
	t.Helper()
	value, err := projectNativeRuntimeJSON(body)
	doc, ok := value.(map[string]any)
	if err != nil || !ok || doc["schema_version"] != json.Number("1") || doc["command"] != "skills add" || doc["result"] != "success" || doc["status"] != "succeeded" || doc["readiness"] != "ready" {
		t.Fatal("actual public raw Skills Ready absent", err)
	}
	if actions, exists := doc["next_actions"]; exists && !reflect.DeepEqual(actions, []any{}) {
		t.Fatal("raw public manual action remains")
	}
	items, ok := doc["items"].([]any)
	if !ok || len(items) != 1 {
		t.Fatal("raw selected items differ")
	}
	item, ok := items[0].(map[string]any)
	status := "succeeded"
	if repeated {
		status = "unchanged"
	}
	if !ok || item["alias"] != copilotRawSkill || item["targetId"] != "copilot" || item["status"] != status {
		t.Fatal("raw selected item/status differs")
	}
}

func copilotRawProof(t *testing.T, f *copilotProjectAttempt, stage, iid string) (domain.Installation, domain.ClientBinding) {
	t.Helper()
	body := copilotRawRead(t, filepath.Join(f.base.Project, ".uap", "state-v2.json"))
	var state domain.StateFileV2
	if err := json.Unmarshal(body, &state); err != nil || len(state.Installations) != 1 {
		t.Fatal("raw exact one installation", err)
	}
	installation := state.Installations[0]
	if installation.InstallationID == "" || (iid != "" && installation.InstallationID != iid) || installation.DeclaredName != copilotRawPackage || len(installation.Clients) != 1 || !reflect.DeepEqual(installation.Package.Inventory, domain.ComponentInventory{Skills: []string{copilotRawSkill}}) {
		t.Fatal("raw selected installation differs")
	}
	source := installation.Source
	if source.Repository != "777genius/universal-agent-plugins" || source.PackageSubpath != "tests/fixtures/uap_apm/skills/benchmark-note" || source.ResolvedRevision != "b5d80336b53397be5bbb72c84f680e5885a6ac0d" || source.StableSourceKey != "github-skill:777genius/universal-agent-plugins:benchmark-note:tests/fixtures/uap_apm/skills/benchmark-note" {
		t.Fatal("raw immutable Git source differs")
	}
	var binding domain.ClientBinding
	for _, b := range installation.Clients {
		binding = b
	}
	physical := domain.ComputePhysicalArtifactID(copilotRawPackage, installation.InstallationID)
	active := filepath.Join(f.base.Project, ".uap", "managed", "clients", "copilot", physical)
	if binding.ClientID != "copilot" || binding.Scope != "project" || binding.NativeProfileRoot != f.base.Project || binding.TargetLocator != active || binding.PhysicalArtifact != physical || binding.ClientBindingID != domain.ComputeClientBindingID(installation.InstallationID, "copilot", "project", active) || binding.Activation != domain.ActivationActive || binding.Authentication != domain.AuthenticationNotRequired || binding.Policy != domain.PolicyAllowed || binding.Verification != domain.VerificationInstalled {
		t.Fatal("raw actual Active binding differs")
	}
	digest, err := (packagesnapshot.Builder{}).Digest(f.ctx, active)
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[string]int{}
	for _, object := range binding.NativeObjects {
		kinds[object.Kind]++
		if object.Kind == "managed_package_directory" && (object.Path != active || object.ManagedDigest != digest || object.ProtectionClass != "managed") {
			t.Fatal("raw managed package receipt differs")
		}
	}
	if len(binding.NativeObjects) != 2 || kinds["managed_package_directory"] != 1 || kinds["copilot_project_plugin_registration"] != 1 {
		t.Fatal("raw native delivery fabricated loose Skill receipts")
	}
	if nativeSHA(t, filepath.Join(active, "skills", copilotRawSkill, "SKILL.md")) != nativeRawProjectSkills[copilotRawSkill] || string(copilotRawRead(t, filepath.Join(active, "skills", copilotRawSkill, "references", "fixture.txt"))) != "Portable Agent Skills 1.0 fixture. This exact file must be preserved without transformation.\n" {
		t.Fatal("raw source Skill/resource bytes differ")
	}
	if _, err := os.Lstat(filepath.Join(f.base.Project, ".github", "skills", copilotRawSkill)); !os.IsNotExist(err) {
		t.Fatal("raw native plugin duplicated loose Skill", err)
	}
	f.keep(t, "raw-"+stage+"-proof.json", body)
	return installation, binding
}

func copilotRawSnapshot(t *testing.T, f *copilotProjectAttempt, binding domain.ClientBinding) map[string]string {
	t.Helper()
	result := map[string]string{"payload": nativeTreeDigest(t, binding.TargetLocator)}
	for _, path := range []string{filepath.Join(f.base.Project, ".uap", "state-v2.json"), filepath.Join(f.base.Project, "uap.toml"), filepath.Join(f.base.Project, "uap.lock"), filepath.Join(f.base.Project, ".github", "copilot", "settings.local.json")} {
		result[path] = nativeSHA(t, path)
	}
	return result
}

func copilotRawInventories(t *testing.T, f *copilotProjectAttempt, stage string, binding *domain.ClientBinding) map[string][]any {
	t.Helper()
	result := map[string][]any{}
	for scope, cwd := range map[string]string{"a": f.base.Project, "b": f.other, "user": f.outside} {
		command := f.command(t, "raw-"+stage+"-"+scope, cwd, f.env(), []string{f.client, "skill", "list", "--json"}, 30*time.Second)
		if len(command.Stderr) != 0 {
			t.Fatal("ordinary Skill catalog diagnostics")
		}
		value, err := projectNativeRuntimeJSON(command.Stdout)
		rows, ok := value.([]any)
		if err != nil || !ok {
			t.Fatal("ordinary Skill catalog array absent", err)
		}
		selected := 0
		kept := []any{}
		for _, value := range rows {
			row, ok := value.(map[string]any)
			if !ok {
				t.Fatal("ordinary Skill row invalid")
			}
			if row["name"] != copilotRawSkill {
				kept = append(kept, row)
				continue
			}
			selected++
			if scope != "a" || binding == nil {
				t.Fatal("raw Skill escaped selected project or removal")
			}
			path := filepath.Join(binding.TargetLocator, "skills", copilotRawSkill)
			skill, err := conformance.ParseSkill(f.ctx, copilotRawSkill, copilotRawRead(t, filepath.Join(path, "SKILL.md")))
			if err != nil || len(row) != 5 || row["description"] != skill.Description || row["source"] != "plugin" || row["path"] != path || row["enabled"] != true {
				t.Fatal("raw native Skill authority differs", err)
			}
		}
		if scope == "a" && binding != nil && selected != 1 {
			t.Fatal("raw Skill absent or ambiguous")
		}
		slices.SortFunc(kept, func(a, b any) int { aa, _ := json.Marshal(a); bb, _ := json.Marshal(b); return bytes.Compare(aa, bb) })
		result[scope] = kept
	}
	copilotRawPluginInventory(t, f, stage, binding)

	return result
}

func copilotRawPluginInventory(t *testing.T, f *copilotProjectAttempt, stage string, binding *domain.ClientBinding) {
	t.Helper()
	listed := f.command(t, "raw-"+stage+"-plugins", f.base.Project, f.env(), []string{f.client, "plugin", "list", "--json"}, 30*time.Second)
	if len(listed.Stderr) != 0 {
		t.Fatal("ordinary plugin catalog diagnostics")
	}
	value, err := projectNativeRuntimeJSON(listed.Stdout)
	rows, ok := value.([]any)
	if err != nil || !ok {
		t.Fatal("ordinary plugin catalog array absent", err)
	}
	found := 0
	for _, value := range rows {
		row, ok := value.(map[string]any)
		if !ok {
			t.Fatal("ordinary plugin row invalid")
		}
		if row["name"] != copilotRawPackage {
			continue
		}
		found++
		if binding == nil || row["marketplace"] != shared.ManagedMarketplaceName(binding.PhysicalArtifact) || row["source"] != "live" || row["installedFrom"] != binding.TargetLocator || row["enabled"] != true {
			t.Fatal("raw plugin identity/scope differs")
		}
	}
	if binding != nil && found != 1 || binding == nil && found != 0 {
		t.Fatal("raw plugin presence differs")
	}
}

// Ordinary final-binding removal retains historical source/package and owned
// DATA. It does not imply purge, even for a package containing only Skills.
func copilotRawRetainedData(t *testing.T, f *copilotProjectAttempt, state domain.StateFileV2, selected domain.Installation, binding domain.ClientBinding) {
	t.Helper()
	if len(state.Installations) != 1 {
		t.Fatal("raw removal lost retained historical installation")
	}
	retained := state.Installations[0]
	if retained.InstallationID != selected.InstallationID || len(retained.Clients) != 0 || !retained.DataRetained ||
		!reflect.DeepEqual(retained.Source, selected.Source) || !reflect.DeepEqual(retained.Package, selected.Package) || len(retained.DataReceipts) != 1 {
		t.Fatal("raw removal did not retain exact selected DATA history")
	}
	base := filepath.Join(f.base.Project, ".uap", "plugin-data")
	manager := providers.PluginDataManager{Base: base}
	for _, receipt := range retained.DataReceipts {
		if receipt.PhysicalBackend != binding.PhysicalArtifact || receipt.Scope != "project" || receipt.State != domain.DataReceiptOwned || receipt.Locator != filepath.Join(base, binding.PhysicalArtifact) {
			t.Fatal("raw retained DATA authority differs")
		}
		if err := manager.ValidateData(f.ctx, receipt); err != nil {
			t.Fatal("raw retained DATA ownership invalid", err)
		}
	}
}

func TestCopilotRawSnapshotUsesPublicProjectDependencyRoot(t *testing.T) {
	project := t.TempDir()
	active := filepath.Join(project, ".uap", "managed", "payload")
	if err := os.MkdirAll(active, 0700); err != nil {
		t.Fatal(err)
	}
	paths := []string{filepath.Join(project, ".uap", "state-v2.json"), filepath.Join(project, "uap.toml"), filepath.Join(project, "uap.lock"), filepath.Join(project, ".github", "copilot", "settings.local.json")}
	for _, path := range paths {
		projectNativeRuntimeWrite(t, path, []byte("inert held bytes\n"), 0600)
	}
	f := &copilotProjectAttempt{base: &nativeFixture{Project: project}}
	got := copilotRawSnapshot(t, f, domain.ClientBinding{TargetLocator: active})
	for _, path := range paths {
		if got[path] != nativeSHA(t, path) {
			t.Fatal("snapshot omitted actual project path", path)
		}
	}
	if _, err := os.Lstat(filepath.Join(project, ".uap", "uap.lock")); !os.IsNotExist(err) {
		t.Fatal("fixture masked invalid dependency-root assumption")
	}
}

func TestCopilotRawRemovalKeepsOrdinaryOwnedDataHistory(t *testing.T) {
	project := t.TempDir()
	ctx := context.Background()
	binding := domain.ClientBinding{PhysicalArtifact: "fixture-physical"}
	selected := domain.Installation{InstallationID: "fixture-installation", Clients: map[string]domain.ClientBinding{"binding": binding}}
	manager := providers.PluginDataManager{Base: filepath.Join(project, ".uap", "plugin-data")}
	receipt, _, err := manager.EnsureData(ctx, selected.InstallationID, binding.PhysicalArtifact, "project")
	if err != nil {
		t.Fatal(err)
	}
	retained := selected
	retained.Clients = map[string]domain.ClientBinding{}
	retained.DataRetained = true
	retained.DataReceipts = map[string]domain.DataReceipt{receipt.DataReceiptID: receipt}
	f := &copilotProjectAttempt{base: &nativeFixture{Project: project}, ctx: ctx}
	copilotRawRetainedData(t, f, domain.StateFileV2{Installations: []domain.Installation{retained}}, selected, binding)
}
