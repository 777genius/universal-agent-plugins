package pluginkitairepo_test

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/statev2"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/opencode"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

const openCodeReadySkill = "uap-opencode-ready-guide"

// Current-source V1 only. The public outcome must become Ready from the same
// owned installation independently observed by fresh native no-model commands.
func TestAgentpluginsOpenCodeNativeProjectSkillReadyLifecycle(t *testing.T) {
	if os.Getenv("AGENTPLUGINS_OPENCODE_PROJECT_NATIVE_E2E") != "1" {
		t.Skip("opt-in current-source project native Skill readiness")
	}
	nativeRequireDisposable(t)
	if os.Getenv("AGENTPLUGINS_OPENCODE_VERSION") != "1.18.34" {
		t.Fatal("pure Skill Ready qualification requires the exact reviewed V1 ABI")
	}
	f := newOpenCodeProjectFixture(t)
	defer f.finish(t)
	f.table = openCodeReadySkillCommandTable()
	f.setup(t)
	source := filepath.Join(f.work, "source-ready-skill")
	nativeWrite(t, filepath.Join(source, "plugin.json"), []byte(`{"$schema":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","name":"uap-commands-fixture","version":"1.0.0"}`), 0600)
	nativeWrite(t, filepath.Join(source, "skills", openCodeReadySkill, "SKILL.md"), []byte("---\nname: "+openCodeReadySkill+"\ndescription: Inert project readiness fixture\n---\nRead the accompanying reference for the project guide.\n"), 0600)
	nativeWrite(t, filepath.Join(source, "skills", openCodeReadySkill, "reference.txt"), []byte("Owned project guide reference.\n"), 0600)
	foreignRoots := []string{filepath.Join(f.A, "opencode.jsonc"), filepath.Join(f.A, ".uap", "data", "caller-owned"),
		filepath.Join(f.B, "opencode.jsonc"), filepath.Join(f.B, ".uap", "data", "caller-owned"),
		filepath.Join(f.work, "xdg-config", "opencode", "opencode.jsonc")}
	foreign := f.snapshot(t, foreignRoots...)
	openCodeReadySkillObserve(t, f, "before-A", f.A, source, false)
	openCodeReadySkillObserve(t, f, "before-B", f.B, source, false)
	nativeAttempt(f.stages, "positive_project_discovery", "add-A.stdout.log")
	added := openCodeReadySkillRun(t, f, "add-A", "add", source, "--no-save")
	commandsRequireMutated(t, "pure Skill add", added, true)
	installation, binding := openCodeReadySkillBinding(t, f, source)
	openCodeReadySkillObserve(t, f, "added-A", f.A, source, true)
	openCodeReadySkillObserve(t, f, "added-B", f.B, source, false)
	f.stages["positive_project_discovery"] = nativeStage{Status: "passed", Reason: "public project add is Ready, active, installed, no-auth and policy-allowed; exact owned full Skill bytes and fresh native A/B catalogs agree"}
	stableRoots := []string{filepath.Join(f.A, ".uap", "state-v2.json"), binding.TargetLocator, filepath.Join(f.A, ".opencode", "skills", openCodeReadySkill)}
	stable := f.snapshot(t, stableRoots...)
	repeated := openCodeReadySkillRun(t, f, "repeat-add-A", "add", source, "--no-save")
	commandsRequireMutated(t, "pure Skill repeat", repeated, false)
	outcome, err := commandsTargetOutcome(repeated)
	if err != nil || outcome["no_change"] != true {
		t.Fatalf("pure Skill repeat lost its public no-change proof: %v %v", outcome, err)
	}
	if again, repeatedBinding := openCodeReadySkillBinding(t, f, source); !reflect.DeepEqual(installation, again) || !reflect.DeepEqual(binding, repeatedBinding) {
		t.Fatal("repeat changed the exact owned native binding")
	}
	openCodeReadySkillObserve(t, f, "repeated-A", f.A, source, true)
	openCodeReadySkillObserve(t, f, "repeated-B", f.B, source, false)
	if !reflect.DeepEqual(stable, f.snapshot(t, stableRoots...)) {
		t.Fatal("repeat or native observation changed owned state or Skill bytes")
	}
	f.stages["unchanged_lifecycle"] = nativeStage{Status: "passed", Reason: "public repeat remains Ready with mutated=false/no_change=true; exact receipt and source/native resource digests stay unchanged across fresh A/B observation"}
	openCodeReadySkillRemove(t, f, source, installation, binding)
	if !reflect.DeepEqual(foreign, f.snapshot(t, foreignRoots...)) {
		t.Fatal("pure Skill lifecycle changed foreign profile/project config or caller-owned data")
	}
	f.keepJSON(t, "pure-skill-ownership.json", map[string]any{"installation_id": installation.InstallationID, "binding": binding, "stable": stable, "foreign": foreign})
	f.stages["foreign_preservation"] = nativeStage{Status: "passed", Reason: "selected config, user config, sibling config and caller-owned persistent data remain byte-identical"}
	if len(f.used) != len(f.table) {
		t.Fatalf("finite pure Skill command table incomplete: used%d want%d", len(f.used), len(f.table))
	}
	f.evidence["scenario"] = "pure-skill-public-ready"
	f.evidence["activation"] = string(domain.ActivationActive)
	f.evidence["authentication"] = string(domain.AuthenticationNotRequired)
	f.evidence["pure_skill_public_ready_qualified"] = true
	f.evidence["native_discovery_qualified"] = true
}

func openCodeReadySkillCommandTable() map[string]string {
	table := map[string]string{"git-A": "git", "git-B": "git", "git-user": "git", "git-json": "git", "version": "native",
		"add-A": "uap", "repeat-add-A": "uap", "remove-A": "uap"}
	for _, stage := range []string{"before-A", "before-B", "added-A", "added-B", "repeated-A", "repeated-B", "removed-A", "removed-B"} {
		table[stage+"-config"], table[stage+"-skill"] = "native", "native"
	}
	return table
}

func openCodeReadySkillRun(t *testing.T, f *openCodeProjectFixture, label string, args ...string) map[string]any {
	t.Helper()
	args = append(args, "--scope", "project", "--project-root", f.A, "--target", "opencode", "--yes", "--format", "json")
	captured := f.command(t, "uap", label, f.installer, f.A, f.env(), args)
	result, err := openCodeProjectConfig(captured.Stdout)
	if err != nil || captured.Exit != 0 || result["result"] != "success" || !openCodeProjectProgressForActivation(label, captured.Stderr, result, "active") {
		t.Fatalf("public pure Skill operation failed its exact output contract: %v; see %s evidence", err, label)
	}
	if args[0] == "remove" {
		return result
	}
	wrapped, err := openCodeProjectAssertionEnvelopeForActivation(result, "active")
	if err != nil {
		t.Fatal(err)
	}
	codexNativeProjectSkillsReady(t, wrapped)
	return wrapped
}

func openCodeReadySkillBinding(t *testing.T, f *openCodeProjectFixture, source string) (domain.Installation, domain.ClientBinding) {
	t.Helper()
	state, err := (statev2.Store{Path: filepath.Join(f.A, ".uap", "state-v2.json"), ProjectRoot: f.A}).Load()
	if err != nil || len(state.Installations) != 1 || len(state.Installations[0].Clients) != 1 {
		t.Fatalf("pure Skill project state: %v", err)
	}
	for _, binding := range state.Installations[0].Clients {
		if binding.ClientID != "opencode" || binding.Scope != "project" || binding.NativeProfileRoot != f.A ||
			binding.Activation != domain.ActivationActive || binding.Verification != domain.VerificationInstalled ||
			binding.Authentication != domain.AuthenticationNotRequired || binding.Policy != domain.PolicyAllowed || len(binding.NativeObjects) != 2 {
			t.Fatalf("public Ready did not persist the exact native outcome: %+v", binding)
		}
		objects := opencode.OpenCodeObjects(binding.NativeObjects)
		packages := 0
		for _, receipt := range binding.NativeObjects {
			if receipt.Kind == "managed_package_directory" && receipt.Path == binding.TargetLocator && receipt.ProtectionClass == "managed" && !receipt.UserModified {
				packages++
			}
		}
		if len(objects) != 1 || packages != 1 {
			t.Fatal("persisted pure Skill package/native receipt composition differs")
		}
		object := objects[0]
		skillRoot := filepath.Join(f.A, ".opencode", "skills", openCodeReadySkill)
		if object.Kind != "opencode_global_skill_directory" || object.ObjectID != "opencode-skill:"+openCodeReadySkill ||
			object.LogicalName != openCodeReadySkill || object.Path != skillRoot || object.SourceRelative != "skills/"+openCodeReadySkill ||
			object.ProtectionClass != "managed" || object.UserModified || object.ManagedDigest == "" {
			t.Fatalf("pure Skill receipt does not own the exact selected directory: %+v", object)
		}
		for _, root := range []string{filepath.Join(source, "skills", openCodeReadySkill), filepath.Join(binding.TargetLocator, "skills", openCodeReadySkill), skillRoot} {
			digest, err := shared.DigestSkillDirectory(root)
			if err != nil || digest != object.ManagedDigest {
				t.Fatalf("complete source/owned/native Skill bytes differ: %s %v", root, err)
			}
		}
		return state.Installations[0], binding
	}
	t.Fatal("pure Skill native binding missing")
	return domain.Installation{}, domain.ClientBinding{}
}

func openCodeReadySkillObserve(t *testing.T, f *openCodeProjectFixture, label, project, source string, present bool) {
	t.Helper()
	config := f.config(t, label, project, nil)
	servers, err := openCodeProjectServers(config)
	if err != nil || len(servers) != 0 {
		t.Fatalf("pure Skill proof unexpectedly includes MCP configuration: %v %v", servers, err)
	}
	found := f.skills(t, label, project, false)
	if !present {
		if _, exists := found[openCodeReadySkill]; exists {
			t.Fatal("fresh native catalog exposes an absent or removed selected Skill")
		}
		return
	}
	content, err := openCodeProjectSkillContent(nativeReadProject(t, filepath.Join(source, "skills", openCodeReadySkill, "SKILL.md")))
	if err != nil {
		t.Fatal(err)
	}
	expected := map[string]openCodeProjectSkill{openCodeReadySkill: {Name: openCodeReadySkill, Location: filepath.Join(project, ".opencode", "skills", openCodeReadySkill, "SKILL.md"), Content: content}}
	if err := openCodeProjectCheckSkills(found, expected, nil); err != nil {
		t.Fatal(err)
	}
}

func openCodeReadySkillRemove(t *testing.T, f *openCodeProjectFixture, source string, before domain.Installation, binding domain.ClientBinding) {
	t.Helper()
	nativeAttempt(f.stages, "remove", "remove-A.stdout.log")
	openCodeReadySkillRun(t, f, "remove-A", "remove", commandsPluginName)
	skillRoot := filepath.Join(f.A, ".opencode", "skills", openCodeReadySkill)
	for _, path := range []string{skillRoot, binding.TargetLocator} {
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			t.Fatalf("removed owned installation path remains: %s %v", path, err)
		}
	}
	state, err := (statev2.Store{Path: filepath.Join(f.A, ".uap", "state-v2.json"), ProjectRoot: f.A}).Load()
	if err != nil || len(state.Installations) != 1 {
		t.Fatalf("ordinary removal lost its retained installation: %v", err)
	}
	after := state.Installations[0]
	if after.InstallationID != before.InstallationID || after.DeclaredName != before.DeclaredName ||
		!reflect.DeepEqual(after.Source, before.Source) || !reflect.DeepEqual(after.Package, before.Package) ||
		len(after.Clients) != 0 || !after.DataRetained || len(after.DataReceipts) == 0 {
		t.Fatalf("ordinary removal did not retain exact identity/data while clearing active bindings: %+v", after)
	}
	openCodeReadySkillObserve(t, f, "removed-A", f.A, source, false)
	openCodeReadySkillObserve(t, f, "removed-B", f.B, source, false)
	f.stages["remove"] = nativeStage{Status: "passed", Reason: "ordinary public remove clears active bindings and owned managed/native paths, retains exact installation identity and data, and fresh A/B catalogs no longer expose the Skill"}
}
