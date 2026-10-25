package pluginkitairepo_test

import (
	"os"
	"path/filepath"
	"testing"
)

// This current-source gate proves prepared file lifecycle with real pinned
// Claude suppression. It does not grant trust or certify loaded project skills.
func TestAgentpluginsClaudeNativeProjectPreparedLifecycle(t *testing.T) {
	if os.Getenv("AGENTPLUGINS_CLAUDE_PROJECT_NATIVE_E2E") != "1" {
		t.Skip("opt-in source project native client execution")
	}
	nativeRequireDisposable(t)
	f := newClaudeProjectNativeFixture(t)
	stages := claudeProjectPreparedStages()
	evidence := claudeProjectNativeIdentity(t, f)
	evidence["project_contract"] = "untrusted_prepared_v1"
	evidence["native_surface"] = "fresh bounded native project lists prove suppression; owned file lifecycle is prepared; no trust/model/MCP initialization"
	defer f.writeEvidence(t, evidence, stages)
	copyTree(t, commandsFixtureSource(t), f.base.PackageRoot)
	c := commandsNativeClient{target: "claude", pkg: f.base.PackageRoot, statePath: filepath.Join(f.project, ".uap", "state-v2.json"), run: f.run}

	nativeAttempt(stages, "install", "add-installer.json")
	added := c.succeed(t, "add", "add", c.pkg)
	requireClaudePreparedResult(t, added)
	if err := commandsPlanSelection(claudePreparedEnvelope(added), "echo", "wizard"); err != nil {
		t.Fatal(err)
	}
	managed := f.preparedPath(t, "binding-after-add")
	f.requirePreparedSkills(t, managed, "echo")
	f.requireSuppression(t, "after-add", true)
	stages["install"] = nativeStage{Status: "passed", Reason: "production initial add installed owned files as Prepared+Installed; native project scan remains suppressed", Artifacts: []string{"add-installer.json", "binding-after-add.json"}}
	stages["prepared_command_projection"] = nativeStage{Status: "passed", Reason: "owned projected files contain exact rendered ordinary command bytes; this is filesystem evidence"}
	stages["prepared_explicit_command_withheld"] = nativeStage{Status: "passed", Reason: "plan and owned projected files withhold the explicit-only command; no native loaded-skill claim"}

	nativeAttempt(stages, "unchanged_add", "add-repeated-installer.json")
	before := nativeSHA(t, c.statePath)
	repeated := c.succeed(t, "add-repeated", "add", c.pkg)
	requireClaudePreparedResult(t, repeated)
	requireClaudePreparedMutation(t, "repeated blocked add", repeated, false)
	if nativeSHA(t, c.statePath) != before {
		t.Fatal("repeated prepared add rewrote state")
	}
	f.requireSuppression(t, "after-repeated-add", true)
	stages["unchanged_add"] = nativeStage{Status: "passed", Reason: "repeated add reobserved real suppression without file/state mutation"}

	nativeAttempt(stages, "unchanged_update", "update-unchanged-installer.json")
	unchanged := c.succeed(t, "update-unchanged", "update", commandsPluginName)
	requireClaudePreparedResult(t, unchanged)
	requireClaudePreparedMutation(t, "unchanged prepared update", unchanged, false)
	if nativeSHA(t, c.statePath) != before {
		t.Fatal("unchanged prepared update rewrote state")
	}
	f.requirePreparedSkills(t, managed, "echo")
	stages["unchanged_update"] = nativeStage{Status: "passed", Reason: "unchanged prepared update retained exact state and projected bytes"}

	nativeAttempt(stages, "invocation_change_update", "update-invocation-change-installer.json")
	commandsSwapInvocation(t, c.pkg)
	changed := c.succeed(t, "update-invocation-change", "update", commandsPluginName)
	requireClaudePreparedResult(t, changed)
	requireClaudePreparedMutation(t, "changed prepared update", changed, true)
	if err := commandsPlanSelection(claudePreparedEnvelope(changed), "wizard", "echo"); err != nil {
		t.Fatal(err)
	}
	if next := f.preparedPath(t, "binding-after-update"); next != managed {
		t.Fatal("update changed owned binding path")
	}
	f.requirePreparedSkills(t, managed, "wizard")
	f.requireSuppression(t, "after-update", true)
	stages["invocation_change_update"] = nativeStage{Status: "passed", Reason: "production update changed owned command projection while native discovery stayed blocked"}

	nativeAttempt(stages, "intact_repair", "repair-intact-installer.json")
	requireClaudePreparedResult(t, c.succeed(t, "repair-intact", "repair", commandsPluginName))
	f.requirePreparedSkills(t, managed, "wizard")
	stages["intact_repair"] = nativeStage{Status: "passed", Reason: "intact repair preserved prepared files and explicit native blocker"}

	nativeAttempt(stages, "absent_repair", "repair-absent-installer.json")
	nativeRequireContained(t, f.project, managed)
	if err := os.RemoveAll(managed); err != nil {
		t.Fatal(err)
	}
	f.requireSuppression(t, "after-owned-directory-loss", false)
	requireClaudePreparedResult(t, c.succeed(t, "repair-absent", "repair", commandsPluginName))
	if next := f.preparedPath(t, "binding-after-repair"); next != managed {
		t.Fatal("repair changed recorded owned binding")
	}
	f.requirePreparedSkills(t, managed, "wizard")
	f.requireSuppression(t, "after-repair", true)
	stages["absent_repair"] = nativeStage{Status: "passed", Reason: "production repair rebuilt missing exact-owned directory; fresh native list again suppressed it"}

	nativeAttempt(stages, "remove", "remove-installer.json")
	c.succeed(t, "remove", "remove", commandsPluginName)
	if _, err := os.Stat(managed); !os.IsNotExist(err) {
		t.Fatalf("owned directory survived remove: %v", err)
	}
	f.requireSuppression(t, "after-remove", false)
	stages["remove"] = nativeStage{Status: "passed", Reason: "production inactive removal deleted only owned project files; fresh native list no longer suppresses a managed directory"}
	f.requireForeign(t)
	stages["project_scope_preservation"] = nativeStage{Status: "passed", Reason: "foreign user/cache/project A/project B hashes remained exact; fresh project-B native lists never exposed A"}
	stages["native_suppression_boundary"] = nativeStage{Status: "passed", Reason: "bounded raw pinned native A/B lists retained the untrusted project suppression boundary"}
}
