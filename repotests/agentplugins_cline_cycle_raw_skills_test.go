//go:build linux

package pluginkitairepo_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packagedigest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

const clineRawSkill = "benchmark-note"
const clineRawPackage = "skill-4e68c33bfb91dc370d5c3745c0ba746428ff227ed3b3466f9c9d1552"
const clineRawSourceKey = "github-skill:777genius/universal-agent-plugins:benchmark-note:tests/fixtures/uap_apm/skills/benchmark-note"

// This is an additional ordinary public Skills journey, after the complete
// mixed-plugin cycle. It shares its genuine binary, private profile, original
// deadline and capture/cleanup mechanism; it never arms a model/provider turn.
func clineProjectRawSkills(t *testing.T, c *clineProjectCycle) {
	t.Helper()
	f, r := c.f, c.recipes["a"]
	root := r.Project
	native := filepath.Join(r.Project, ".cline", "skills", clineRawSkill)
	statePath := c.statePath(r)
	before := clineRawRead(t, statePath)
	old, err := clineCycleState(before)
	if err != nil || len(old.Installations) != 1 || !reflect.DeepEqual(old.Installations[0], r.Installation) {
		t.Fatal("raw Cline case requires the retained mixed installation", err)
	}
	foreignRoot := filepath.Join(filepath.Dir(native), "foreign-a")
	f.trees[foreignRoot] = clineRawTree(t, foreignRoot)
	baseline := clineRawTree(t, filepath.Dir(native))
	events, err := clineCycleEventBytes(f.base.Events)
	if err != nil {
		t.Fatal(err)
	}
	c.rawIdle(t)
	record := map[string]any{"source": nativeRawProjectSkillsSource, "skill_sha256": nativeRawProjectSkills[clineRawSkill], "commands": []any{}, "proofs": map[string]any{}, "inventories": []any{}, "probe_before": c.stream(t, events)}
	f.evidence["raw_skill_cycle"] = record
	c.rawInventories(t, record, "before", nil)
	delete(f.protected, statePath)
	delete(f.trees, filepath.Dir(native))
	add := []string{f.installer, "skills", "add", nativeRawProjectSkillsSource, "--skill", clineRawSkill, "--target", "cline", "--scope", "project", "--project-root", r.Project, "--yes", "--no-ui", "--format", "json"}
	first := c.rawCommand(t, record, "add", f.outside, add)
	if err := clineRawReady(first.Stdout, false); err != nil {
		t.Fatal(err)
	}
	i, binding := c.rawProof(t, record, "add", old.Installations[0], "", native)
	c.rawInventories(t, record, "add", map[string]string{filepath.Join(native, "SKILL.md"): string(clineRawRead(t, filepath.Join(native, "SKILL.md")))})
	stable := c.rawSnapshot(t, root, native, binding.TargetLocator)
	repeated := c.rawCommand(t, record, "repeat", f.outside, add)
	if err := clineRawReady(repeated.Stdout, true); err != nil {
		t.Fatal(err)
	}
	replay, replayBinding := c.rawProof(t, record, "repeat", old.Installations[0], i.InstallationID, native)
	after := c.rawSnapshot(t, root, native, binding.TargetLocator)
	record["repeat_before"], record["repeat_after"] = stable, after
	if !reflect.DeepEqual(i, replay) || !reflect.DeepEqual(binding, replayBinding) || !reflect.DeepEqual(stable, after) {
		t.Fatal("raw Cline repeat changed IID, binding, state, lock or owned payload")
	}
	c.rawInventories(t, record, "repeat", map[string]string{filepath.Join(native, "SKILL.md"): string(clineRawRead(t, filepath.Join(native, "SKILL.md")))})
	for _, name := range []string{"state-v2.json", "uap.toml", "uap.lock"} {
		path := filepath.Join(root, name)
		if name == "state-v2.json" {
			path = filepath.Join(root, ".uap", name)
		}
		delete(f.protected, path)
	}
	delete(f.trees, binding.TargetLocator)
	delete(f.trees, filepath.Dir(native))
	remove := []string{f.installer, "remove", i.InstallationID, "--target", "cline", "--scope", "project", "--project-root", r.Project, "--yes", "--no-ui", "--format", "json"}
	removed := c.rawCommand(t, record, "remove", f.outside, remove)
	value, err := projectNativeRuntimeJSON(removed.Stdout)
	doc, ok := value.(map[string]any)
	if err != nil || !ok || doc["schema_version"] != json.Number("1") || doc["command"] != "agentplugins remove" || doc["result"] != "success" {
		t.Fatal("raw Cline removal public DTO differs", err)
	}
	c.rawRemoved(t, record, old.Installations[0], i, native, binding.TargetLocator)
	c.rawInventories(t, record, "remove", nil)
	if !reflect.DeepEqual(baseline, clineRawTree(t, filepath.Dir(native))) {
		t.Fatal("raw Cline removal changed foreign native tree")
	}
	f.trees[filepath.Dir(native)] = baseline
	finalEvents, err := clineCycleEventBytes(f.base.Events)
	if err != nil || !bytes.Equal(events, finalEvents) {
		t.Fatal("raw Cline catalogs invoked the MCP probe", err)
	}
	c.rawIdle(t)
	record["probe_after"] = c.stream(t, finalEvents)
	f.check(t)
}

func (c *clineProjectCycle) rawIdle(t *testing.T) {
	t.Helper()
	c.owner.mu.Lock()
	idle := c.owner.current == nil && c.owner.busy == 0 && !c.owner.failure && reflect.DeepEqual(c.owner.completed, clineCycleACP)
	c.owner.mu.Unlock()
	if !idle {
		t.Fatal("raw Cline catalogs require the completed unarmed provider")
	}
}

func (c *clineProjectCycle) rawCommand(t *testing.T, record map[string]any, stage, cwd string, argv []string) ports.CommandResult {
	t.Helper()
	c.f.check(t)
	c.rawIdle(t)
	result, expired, err := c.capture(cwd, argv)
	row := map[string]any{"stage": stage, "argv": argv, "cwd": cwd, "exit": result.ExitCode, "error": err != nil, "deadline": expired, "stdout": c.stream(t, result.Stdout), "stderr": c.stream(t, result.Stderr)}
	record["commands"] = append(record["commands"].([]any), row)
	c.f.check(t)
	c.rawIdle(t)
	if err != nil || expired || result.ExitCode != 0 {
		t.Fatal("raw Cline command capture/exit/closure", err)
	}
	return result
}

func clineRawReady(raw []byte, repeated bool) error {
	value, err := projectNativeRuntimeJSON(raw)
	doc, ok := value.(map[string]any)
	if err != nil || !ok || doc["schema_version"] != json.Number("1") || doc["command"] != "skills add" || doc["result"] != "success" || doc["status"] != "succeeded" || doc["readiness"] != "ready" {
		return fmt.Errorf("raw Cline public Ready absent")
	}
	if actions, exists := doc["next_actions"]; exists && !reflect.DeepEqual(actions, []any{}) {
		return fmt.Errorf("raw Cline public action remains")
	}
	items, ok := doc["items"].([]any)
	if !ok || len(items) != 1 {
		return fmt.Errorf("raw Cline selected item differs")
	}
	item, ok := items[0].(map[string]any)
	status := "succeeded"
	if repeated {
		status = "unchanged"
	}
	if !ok || item["alias"] != clineRawSkill || item["targetId"] != "cline" || item["status"] != status {
		return fmt.Errorf("raw Cline selected item/status differs")
	}
	return nil
}

func (c *clineProjectCycle) rawInventories(t *testing.T, record map[string]any, stage string, extra map[string]string) {
	t.Helper()
	for _, scope := range []string{"a", "b", "user"} {
		r := c.recipes[scope]
		argv := []string{c.f.client, "--auto-approve", "false", "--cwd", r.Project, "config", "skills", "--json"}
		result := c.rawCommand(t, record, stage+"-"+scope, r.Project, argv)
		record["inventories"] = append(record["inventories"].([]any), map[string]any{"stage": stage, "scope": scope, "stdout": c.stream(t, result.Stdout)})
		selected := map[string]string{}
		if scope == "a" {
			selected = extra
		}
		if len(result.Stderr) != 0 {
			t.Fatal("raw Cline native catalog diagnostics")
		}
		if err := c.skills(result.Stdout, r, nil, selected); err != nil {
			t.Fatal(err)
		}
	}
}

func clineRawRead(t *testing.T, path string) []byte {
	t.Helper()
	raw, err := projectNativeRuntimeRead(path, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func clineRawTree(t *testing.T, path string) map[string]string {
	t.Helper()
	tree, err := clineAttemptTree(path)
	if err != nil {
		t.Fatal(err)
	}
	return tree
}

func (c *clineProjectCycle) rawSnapshot(t *testing.T, root, native, managed string) map[string]any {
	t.Helper()
	result := map[string]any{"native": clineRawTree(t, native), "managed": clineRawTree(t, managed)}
	for _, name := range []string{"state-v2.json", "uap.toml", "uap.lock"} {
		path := filepath.Join(root, name)
		if name == "state-v2.json" {
			path = filepath.Join(root, ".uap", name)
		}
		raw := clineRawRead(t, path)
		c.f.protected[path] = clineAttemptSHA(raw)
		result[name] = c.stream(t, raw)
	}
	return result
}

func (c *clineProjectCycle) rawProof(t *testing.T, record map[string]any, stage string, old domain.Installation, iid, native string) (domain.Installation, domain.ClientBinding) {
	t.Helper()
	raw := clineRawRead(t, c.statePath(c.recipes["a"]))
	state, err := clineCycleState(raw)
	if err != nil || len(state.Installations) != 2 {
		t.Fatal("raw Cline exact two installations required", err)
	}
	var i domain.Installation
	retained := false
	for _, candidate := range state.Installations {
		if candidate.InstallationID == old.InstallationID {
			retained = reflect.DeepEqual(candidate, old)
		} else {
			i = candidate
		}
	}
	if !retained || i.InstallationID == "" || (iid != "" && i.InstallationID != iid) || i.DeclaredName != clineRawPackage || len(i.Clients) != 1 || i.DataRetained {
		t.Fatal("raw Cline selected/retained installation differs")
	}
	source := i.Source
	if source.StableSourceKey != clineRawSourceKey || source.SourceBindingID != "src_"+clineAttemptSHA([]byte(clineRawSourceKey))[:24] || source.Repository != "777genius/universal-agent-plugins" || source.PackageSubpath != "tests/fixtures/uap_apm/skills/benchmark-note" || source.ResolvedRevision != "b5d80336b53397be5bbb72c84f680e5885a6ac0d" || !reflect.DeepEqual(i.Package.Inventory, domain.ComponentInventory{Skills: []string{clineRawSkill}}) {
		t.Fatal("raw Cline immutable source/selected package differs")
	}
	var binding domain.ClientBinding
	for _, b := range i.Clients {
		binding = b
	}
	physical := domain.ComputePhysicalArtifactID(clineRawPackage, i.InstallationID)
	active := filepath.Join(c.recipes["a"].Project, ".uap", "managed", "clients", "cline", physical)
	if binding.ClientID != "cline" || binding.Scope != "project" || binding.NativeProfileRoot != c.recipes["a"].Project || binding.TargetLocator != active || binding.PhysicalArtifact != physical || binding.ClientBindingID != domain.ComputeClientBindingID(i.InstallationID, "cline", "project", active) || binding.Activation != domain.ActivationActive || binding.Authentication != domain.AuthenticationNotRequired || binding.Policy != domain.PolicyAllowed || binding.Materialization != domain.MaterializationMaterialized || binding.Verification != domain.VerificationInstalled {
		t.Fatal("raw Cline exact active binding differs")
	}
	paths := map[string][]byte{}
	for _, relative := range []string{"SKILL.md", "references/fixture.txt"} {
		body := clineRawRead(t, filepath.Join(native, relative))
		if !bytes.Equal(body, clineRawRead(t, filepath.Join(active, "skills", clineRawSkill, relative))) {
			t.Fatal("raw Cline managed/native bytes differ")
		}
		paths[relative] = body
	}
	if clineAttemptSHA(paths["SKILL.md"]) != nativeRawProjectSkills[clineRawSkill] || string(paths["references/fixture.txt"]) != "Portable Agent Skills 1.0 fixture. This exact file must be preserved without transformation.\n" {
		t.Fatal("raw Cline delivered bytes differ from pinned Git source")
	}
	entries, err := clineAttemptPayload(active)
	if err != nil {
		t.Fatal(err)
	}
	digest, err := projectNativeRuntimeInstalledDigest(c.f.ctx, entries)
	if err != nil {
		t.Fatal(err)
	}
	sourceDigest, err := packagedigest.DigestCaptured(c.f.ctx, entries)
	manifest := clineRawRead(t, filepath.Join(active, "plugin.json"))
	wantManifest, _ := json.Marshal(map[string]any{"$schema": domain.PluginSchemaV1, "name": clineRawPackage})
	if err != nil || sourceDigest != i.Source.TreeDigest || !bytes.Equal(manifest, append(wantManifest, '\n')) || i.Package.ManifestDigest != "sha256:"+clineAttemptSHA(manifest) {
		t.Fatal("raw Cline managed package/source digest differs", err)
	}
	objects := map[string]domain.NativeObjectOwnership{}
	for _, obj := range binding.NativeObjects {
		if _, exists := objects[obj.Kind]; exists {
			t.Fatal("raw Cline duplicate native ownership")
		}
		objects[obj.Kind] = obj
	}
	wantPackage := domain.NativeObjectOwnership{ObjectID: "package:cline:" + physical, Kind: "managed_package_directory", LogicalName: clineRawPackage, Path: active, ManagedDigest: digest, ProtectionClass: "managed"}
	if len(objects) != 2 || !reflect.DeepEqual(objects["managed_package_directory"], wantPackage) {
		t.Fatal("raw Cline managed ownership differs")
	}
	owned := objects["cline_project_skill_directory"]
	nativeDigest, err := shared.DigestSkillDirectory(native)
	want := domain.NativeObjectOwnership{ObjectID: "cline_project_skill_directory:" + clineRawSkill, Kind: "cline_project_skill_directory", LogicalName: clineRawSkill, Path: native, SourceRelative: "skills/" + clineRawSkill, ManagedDigest: nativeDigest, ProtectionClass: "managed"}
	if err != nil || !reflect.DeepEqual(owned, want) {
		t.Fatal("raw Cline exact Skill ownership differs", err)
	}
	c.f.protected[c.statePath(c.recipes["a"])] = clineAttemptSHA(raw)
	c.f.trees[active] = clineRawTree(t, active)
	c.f.trees[filepath.Dir(native)] = clineRawTree(t, filepath.Dir(native))
	record["proofs"].(map[string]any)[stage] = map[string]any{"state": c.stream(t, raw), "skill": c.stream(t, paths["SKILL.md"]), "resource": c.stream(t, paths["references/fixture.txt"]), "binding": binding}
	return i, binding
}

func (c *clineProjectCycle) rawRemoved(t *testing.T, record map[string]any, old, selected domain.Installation, native, active string) {
	t.Helper()
	raw := clineRawRead(t, c.statePath(c.recipes["a"]))
	state, err := clineCycleState(raw)
	if err != nil || len(state.Installations) != 2 {
		t.Fatal("raw Cline removal state differs", err)
	}
	for _, i := range state.Installations {
		if i.InstallationID == old.InstallationID {
			if !reflect.DeepEqual(i, old) {
				t.Fatal("raw Cline removal changed retained mixed installation")
			}
		} else if i.InstallationID != selected.InstallationID || len(i.Clients) != 0 || !i.DataRetained || !reflect.DeepEqual(i.Source, selected.Source) || !reflect.DeepEqual(i.Package, selected.Package) {
			t.Fatal("raw Cline removal did not retain only selected data")
		}
	}
	for _, path := range []string{native, active} {
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			t.Fatal("raw Cline removed owned path remains", err)
		}
		c.f.absent[path] = true
	}
	c.f.protected[c.statePath(c.recipes["a"])] = clineAttemptSHA(raw)
	record["proofs"].(map[string]any)["remove"] = map[string]any{"state": c.stream(t, raw)}
}

func TestClineRawSnapshotUsesPublicProjectDependencyRoot(t *testing.T) {
	project, native, managed := t.TempDir(), t.TempDir(), t.TempDir()
	for _, path := range []string{filepath.Join(project, ".uap", "state-v2.json"), filepath.Join(project, "uap.toml"), filepath.Join(project, "uap.lock")} {
		projectNativeRuntimeWrite(t, path, []byte("inert held bytes\n"), 0600)
	}
	c := &clineProjectCycle{f: &clineProjectAttempt{key: "inert-fixture-key", protected: map[string]string{}}}
	got := c.rawSnapshot(t, project, native, managed)
	if len(got) != 5 {
		t.Fatal("raw snapshot omitted held state or payload")
	}
	for _, path := range []string{filepath.Join(project, ".uap", "state-v2.json"), filepath.Join(project, "uap.toml"), filepath.Join(project, "uap.lock")} {
		if c.f.protected[path] != clineAttemptSHA(clineRawRead(t, path)) {
			t.Fatal("raw snapshot protected wrong project path", path)
		}
	}
	if _, err := os.Lstat(filepath.Join(project, ".uap", "uap.toml")); !os.IsNotExist(err) {
		t.Fatal("fixture masked invalid dependency root")
	}
}
