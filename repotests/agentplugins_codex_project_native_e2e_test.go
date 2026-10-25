package pluginkitairepo_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// This source-only suite uses genuine Codex skills/list after production UAP
// project operations. It does not attest activation, establish native trust,
// initialize MCP, start a thread or issue a model request.
func TestAgentpluginsCodexNativeProjectComponentsLifecycle(t *testing.T) {
	if os.Getenv("AGENTPLUGINS_CODEX_PROJECT_NATIVE_E2E") != "1" {
		t.Skip("opt-in source project native client execution")
	}
	nativeRequireDisposable(t)
	if _, err := os.Lstat("/etc/codex"); !os.IsNotExist(err) {
		t.Fatal("system Codex config must be absent in disposable image")
	}
	f := newCodexProjectNativeFixture(t)
	stages := map[string]nativeStage{}
	for _, name := range []string{"project_install", "project_unchanged_add", "project_unchanged_update", "project_update", "project_missing_skill_repair", "project_remove", "project_foreign_preservation"} {
		stages[name] = nativeStage{Status: "not_evaluated", Reason: "not reached"}
	}
	stages["native_mcp_initialization"] = nativeStage{Status: "not_evaluated", Reason: "skills-only fixture; no trust mutation or MCP initialization"}
	stages["model_use"] = nativeStage{Status: "not_evaluated", Reason: "discovery-only RPC; no thread or model request"}
	evidence := f.evidence(t, stages)
	defer func() {
		f.writeEvidence(t, evidence)
		if err := nativeStageFailures(stages); err != nil {
			t.Error(err)
		}
	}()
	codexNativeProjectInstall(t, f, stages)
	codexNativeProjectUnchanged(t, f, stages)
	codexNativeProjectUpdate(t, f, stages)
	codexNativeProjectRepair(t, f, stages)
	codexNativeProjectRemove(t, f, stages)
	f.requireForeign(t)
	stages["project_foreign_preservation"] = nativeStage{Status: "passed", Reason: "user config/cache and both projects' foreign configuration and skills retain exact bytes"}
}

func codexNativeProjectPrepared(t *testing.T, result map[string]any) {
	t.Helper()
	outcome, err := commandsTargetOutcome(result)
	if err != nil {
		t.Fatal(err)
	}
	activation, _ := outcome["activation"].(map[string]any)
	if activation["activation"] != string(domain.ActivationPrepared) || activation["verification"] != string(domain.VerificationInstalled) || activation["activation_attested"] == true {
		t.Fatalf("project file outcome manufactured activation proof: %v", activation)
	}
}

func codexNativeProjectSkills(t *testing.T, f *codexProjectNativeFixture, label, command string) {
	t.Helper()
	if err := commandsCheckSkills(f.skills(t, label+"-project-A", f.base.Project), command); err != nil {
		t.Fatal(err)
	}
	if leaked := f.skills(t, label+"-project-B", f.otherProject); len(leaked) != 0 {
		t.Fatalf("project A skills leaked into project B: %v", leaked)
	}
}

func codexNativeProjectInstall(t *testing.T, f *codexProjectNativeFixture, stages map[string]nativeStage) {
	t.Helper()
	nativeAttempt(stages, "project_install", "project-add-installer.json")
	added := f.run(t, "project-add", "add", f.base.Package, "--no-save")
	codexNativeProjectPrepared(t, added)
	if err := commandsPlanSelection(added, "echo", "wizard"); err != nil {
		t.Fatal(err)
	}
	codexNativeProjectSkills(t, f, "skills-after-project-add", "echo")
	stages["project_install"] = nativeStage{Status: "passed", Reason: "UAP project add remains prepared; genuine Codex discovers exact project-A repository skill paths and rendered bytes, with no fixture skills in project B"}
}

func codexNativeProjectUnchanged(t *testing.T, f *codexProjectNativeFixture, stages map[string]nativeStage) {
	t.Helper()
	for _, step := range []struct {
		stage, label string
		args         []string
	}{
		{"project_unchanged_add", "project-add-repeated", []string{"add", f.base.Package, "--no-save"}},
		{"project_unchanged_update", "project-update-unchanged", []string{"update", commandsPluginName}},
	} {
		nativeAttempt(stages, step.stage, step.label+"-installer.json")
		before := map[string]string{}
		for _, path := range []string{
			filepath.Join(f.base.Project, ".uap", "state-v2.json"),
			filepath.Join(f.base.Project, ".agents", "skills", commandsGuideSkill, "SKILL.md"),
			filepath.Join(f.base.Project, ".agents", "skills", commandsSkill("echo"), "SKILL.md"),
		} {
			before[path] = nativeSHA(t, path)
		}
		result := codexProjectAssertionEnvelope(f.run(t, step.label, step.args...))
		codexNativeProjectPrepared(t, result)
		commandsRequireMutated(t, step.label, result, false)
		outcome, err := commandsTargetOutcome(result)
		if err != nil {
			t.Fatal(err)
		}
		plan, _ := outcome["plan"].(map[string]any)
		if outcome["no_change"] != true || plan["client_id"] != "codex" || plan["scope"] != "project" {
			t.Fatalf("unchanged operation lost its exact project Codex result: %v", outcome)
		}
		if err := commandsPlanSelection(result, "echo", "wizard"); err != nil {
			t.Fatal(err)
		}
		requireUnchanged := func() {
			t.Helper()
			for path, digest := range before {
				if nativeSHA(t, path) != digest {
					t.Fatalf("%s changed stable project state or owned skill bytes: %s", step.label, path)
				}
			}
		}
		requireUnchanged()
		codexNativeProjectSkills(t, f, "skills-after-"+step.label, "echo")
		requireUnchanged()
		stages[step.stage] = nativeStage{Status: "passed", Reason: "public unchanged operation reports mutated=false and no_change=true; exact project state/owned skill bytes persist before and after fresh native A/B discovery", Artifacts: []string{step.label + "-installer.json"}}
	}
}

// A replay may use the public single-result envelope. Wrap only the assertion
// view; commandsRunInstaller retains the original JSON bytes as evidence.
func codexProjectAssertionEnvelope(result map[string]any) map[string]any {
	data, _ := result["data"].(map[string]any)
	if _, ok := data["targets"]; ok {
		return result
	}
	if outcome, ok := data["result"].(map[string]any); ok {
		return map[string]any{"data": map[string]any{"targets": []any{map[string]any{"output": map[string]any{"result": outcome}}}}}
	}
	return result
}

func codexNativeProjectUpdate(t *testing.T, f *codexProjectNativeFixture, stages map[string]nativeStage) {
	t.Helper()
	nativeAttempt(stages, "project_update", "project-update-installer.json")
	commandsSwapInvocation(t, f.base.Package)
	updated := f.run(t, "project-update", "update", commandsPluginName)
	codexNativeProjectPrepared(t, updated)
	commandsRequireMutated(t, "project update", updated, true)
	if err := commandsPlanSelection(updated, "wizard", "echo"); err != nil {
		t.Fatal(err)
	}
	codexNativeProjectSkills(t, f, "skills-after-project-update", "wizard")
	stages["project_update"] = nativeStage{Status: "passed", Reason: "UAP update replaces the command skill after invocation rules swap; fresh Codex discovery sees only the newly ordinary command with exact bytes"}
}

func codexNativeProjectRepair(t *testing.T, f *codexProjectNativeFixture, stages map[string]nativeStage) {
	t.Helper()
	nativeAttempt(stages, "project_missing_skill_repair", "project-repair-installer.json")
	missing := filepath.Join(f.base.Project, ".agents", "skills", commandsSkill("wizard"))
	nativeRequireContained(t, f.base.Project, missing)
	if err := os.RemoveAll(missing); err != nil {
		t.Fatal(err)
	}
	if found := f.skills(t, "skills-with-project-command-missing", f.base.Project); len(found) != 1 || found[commandsGuideSkill] == "" {
		t.Fatalf("native discovery did not observe the missing owned command skill: %v", found)
	}
	repaired := f.run(t, "project-repair", "repair", commandsPluginName)
	codexNativeProjectPrepared(t, repaired)
	codexNativeProjectSkills(t, f, "skills-after-project-repair", "wizard")
	stages["project_missing_skill_repair"] = nativeStage{Status: "passed", Reason: "fresh Codex observes the missing owned directory before repair and exact restored command skill after UAP repair"}
}

func codexNativeProjectRemove(t *testing.T, f *codexProjectNativeFixture, stages map[string]nativeStage) {
	t.Helper()
	nativeAttempt(stages, "project_remove", "project-remove-installer.json")
	f.run(t, "project-remove", "remove", commandsPluginName)
	for _, project := range []string{f.base.Project, f.otherProject} {
		label := "skills-after-project-remove-" + filepath.Base(project)
		if remaining := f.skills(t, label, project); len(remaining) != 0 {
			t.Fatalf("native Codex still discovers removed fixture skills: %v", remaining)
		}
	}
	stages["project_remove"] = nativeStage{Status: "passed", Reason: "fresh native Codex processes list no removed fixture skills in either project after UAP removal"}
}
