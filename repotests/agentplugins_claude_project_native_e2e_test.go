package pluginkitairepo_test

import (
	"os"
	"path/filepath"
	"testing"
)

// The historical selector now qualifies automatic project installation with
// the pinned real Claude CLI. Confirmed installation grants only exact project trust; no authentication or model call is needed.
func TestAgentpluginsClaudeNativeProjectPreparedLifecycle(t *testing.T) {
	if os.Getenv("AGENTPLUGINS_CLAUDE_PROJECT_NATIVE_E2E") != "1" {
		t.Skip("opt-in source project native client execution")
	}
	nativeRequireDisposable(t)
	f := newClaudeProjectNativeFixture(t)
	stages := map[string]nativeStage{}
	for _, name := range []string{"install", "unchanged_add", "unchanged_update", "invocation_change_update", "intact_repair", "absent_repair", "remove", "project_scope_preservation", "native_skill_discovery", "command_skill_discovery", "explicit_command_withheld"} {
		stages[name] = nativeStage{Status: "not_evaluated", Reason: "not reached"}
	}
	evidence := claudeProjectNativeIdentity(t, f)
	evidence["project_contract"] = "project_marketplace_installation_v1"
	evidence["native_surface"] = "explicit scoped native marketplace install, enabled A/disabled B inventory and exact native Skill details; exact selected-project trust; no model/authentication or MCP initialization"
	defer f.writeEvidence(t, evidence, stages)
	claudeProjectInitializeUntrusted(t, f)
	copyTree(t, commandsFixtureSource(t), f.base.PackageRoot)
	c := commandsNativeClient{target: "claude", pkg: f.base.PackageRoot, statePath: filepath.Join(f.project, ".uap", "state-v2.json"), run: f.run}

	nativeAttempt(stages, "install", "add-installer.json")
	added := c.succeed(t, "add", "add", c.pkg)
	requireClaudeProjectActive(t, added)
	requireClaudeSelectedProjectTrust(t, f)
	if err := commandsPlanSelection(claudePreparedEnvelope(added), "echo", "wizard"); err != nil {
		t.Fatal(err)
	}
	managed := f.projectManagedPath(t, "binding-after-add")
	f.requireProjectSkills(t, managed, "echo")
	claudeProjectMarketplaceObservation(t, f, "after-add", managed, "echo", true)
	claudeProjectInitializedSkills(t, f, "after-add", []string{commandsPluginName + ":" + commandsGuideSkill, commandsPluginName + ":" + commandsSkill("echo")}, true)
	stages["install"] = nativeStage{Status: "passed", Reason: "production add installed and activated the exact scoped native plugin with exact selected-project trust", Artifacts: []string{"add-installer.json", "binding-after-add.json"}}
	stages["native_skill_discovery"] = nativeStage{Status: "passed", Reason: "native plugin details expose exactly the installed selected Skills and their verified cached bytes"}
	stages["command_skill_discovery"] = nativeStage{Status: "passed", Reason: "native plugin details expose the exact ordinary command rendered as a Skill"}
	stages["explicit_command_withheld"] = nativeStage{Status: "passed", Reason: "the plan, owned files and native Skill inventory all withhold the explicit-only command"}

	nativeAttempt(stages, "unchanged_add", "add-repeated-installer.json")
	before := nativeSHA(t, c.statePath)
	repeated := c.succeed(t, "add-repeated", "add", c.pkg)
	requireClaudeProjectActive(t, repeated)
	requireClaudePreparedMutation(t, "repeated native add", repeated, false)
	if nativeSHA(t, c.statePath) != before {
		t.Fatal("repeated native add rewrote state")
	}
	claudeProjectMarketplaceObservation(t, f, "after-repeated-add", managed, "echo", true)
	stages["unchanged_add"] = nativeStage{Status: "passed", Reason: "repeated add reobserved the enabled native plugin without file/state mutation"}

	nativeAttempt(stages, "unchanged_update", "update-unchanged-installer.json")
	unchanged := c.succeed(t, "update-unchanged", "update", commandsPluginName)
	requireClaudeProjectActive(t, unchanged)
	requireClaudePreparedMutation(t, "unchanged native update", unchanged, false)
	if nativeSHA(t, c.statePath) != before {
		t.Fatal("unchanged native update rewrote state")
	}
	f.requireProjectSkills(t, managed, "echo")
	stages["unchanged_update"] = nativeStage{Status: "passed", Reason: "unchanged native update retained exact state and projected bytes"}

	nativeAttempt(stages, "invocation_change_update", "update-invocation-change-installer.json")
	commandsSwapInvocation(t, c.pkg)
	changed := c.succeed(t, "update-invocation-change", "update", commandsPluginName)
	requireClaudeProjectActive(t, changed)
	requireClaudePreparedMutation(t, "changed native update", changed, true)
	if err := commandsPlanSelection(claudePreparedEnvelope(changed), "wizard", "echo"); err != nil {
		t.Fatal(err)
	}
	if next := f.projectManagedPath(t, "binding-after-update"); next != managed {
		t.Fatal("update changed owned binding path")
	}
	f.requireProjectSkills(t, managed, "wizard")
	claudeProjectMarketplaceObservation(t, f, "after-update", managed, "wizard", true)
	claudeProjectInitializedSkills(t, f, "after-update", []string{commandsPluginName + ":" + commandsGuideSkill, commandsPluginName + ":" + commandsSkill("wizard")}, true)
	stages["invocation_change_update"] = nativeStage{Status: "passed", Reason: "production update changed the ordinary command and refreshed same-version native cached bytes"}

	nativeAttempt(stages, "intact_repair", "repair-intact-installer.json")
	requireClaudeProjectActive(t, c.succeed(t, "repair-intact", "repair", commandsPluginName))
	f.requireProjectSkills(t, managed, "wizard")
	stages["intact_repair"] = nativeStage{Status: "passed", Reason: "intact repair reverified native activation and preserved owned files"}

	nativeAttempt(stages, "absent_repair", "repair-absent-installer.json")
	nativeRequireContained(t, f.project, managed)
	if err := os.RemoveAll(managed); err != nil {
		t.Fatal(err)
	}
	requireClaudeProjectActive(t, c.succeed(t, "repair-absent", "repair", commandsPluginName))
	if next := f.projectManagedPath(t, "binding-after-repair"); next != managed {
		t.Fatal("repair changed recorded owned binding")
	}
	f.requireProjectSkills(t, managed, "wizard")
	claudeProjectMarketplaceObservation(t, f, "after-repair", managed, "wizard", true)
	stages["absent_repair"] = nativeStage{Status: "passed", Reason: "production repair rebuilt the missing exact-owned directory and verified native discovery"}

	nativeAttempt(stages, "remove", "remove-installer.json")
	c.succeed(t, "remove", "remove", commandsPluginName)
	if _, err := os.Stat(managed); !os.IsNotExist(err) {
		t.Fatalf("owned directory survived remove: %v", err)
	}
	claudeProjectMarketplaceObservation(t, f, "after-remove", managed, "", false)
	claudeProjectInitializedSkills(t, f, "after-remove", []string{commandsPluginName + ":" + commandsGuideSkill, commandsPluginName + ":" + commandsSkill("wizard")}, false)
	requireClaudeSelectedProjectTrust(t, f)
	stages["remove"] = nativeStage{Status: "passed", Reason: "production removal withdrew the exact project registration and owned directory; native A/B inventories no longer contain it"}
	f.requireForeign(t)
	stages["project_scope_preservation"] = nativeStage{Status: "passed", Reason: "foreign user/cache/B bytes and A settings keys preserved; B disabled the A registration and refused its details"}
	claudeNativeProjectRawSkills(t, f, stages)
}

func requireClaudeSelectedProjectTrust(t *testing.T, f *claudeProjectNativeFixture) {
	t.Helper()
	profile, err := readClaudeTrustedProfile(filepath.Join(f.base.ConfigDir, ".claude.json"))
	content, parseErr := claudeUntrustedProfileContent(profile, filepath.ToSlash(f.project))
	projects, _ := content["projects"].(map[string]any)
	selected, _ := projects[filepath.ToSlash(f.project)].(map[string]any)
	if err != nil || parseErr != nil || selected["hasTrustDialogAccepted"] != true {
		t.Fatal("confirmed installation's exact project trust was not preserved", err, parseErr)
	}
}
