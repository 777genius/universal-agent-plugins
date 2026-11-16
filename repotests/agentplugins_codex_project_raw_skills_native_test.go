package pluginkitairepo_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// The actual public Git transport supplies two inert Agent Skills; no native
// command or trust setting is injected into the installation under test.
const nativeRawProjectSkillsSource = "https://github.com/777genius/universal-agent-plugins/tree/b5d80336b53397be5bbb72c84f680e5885a6ac0d/tests/fixtures/uap_apm/skills"

var nativeRawProjectSkills = map[string]string{
	"benchmark-note":      "53e39c4f1dcaa4c2ed1196654678ede091b71fb837f77a1beb07ba556abc1198",
	"benchmark-secondary": "e2090662c57975dee9fd7bb046df87a7764380661d87690e2c539a1a955ed777",
}

func codexNativeProjectRawSkills(t *testing.T, f *codexProjectNativeFixture, stages map[string]nativeStage) {
	t.Helper()
	for _, stage := range []string{"project_raw_skills_install", "project_raw_skills_repeat"} {
		nativeAttempt(stages, stage, stage+"-installer.json")
		before := ""
		state := filepath.Join(f.base.Project, ".uap", "state-v2.json")
		if stage == "project_raw_skills_repeat" {
			before = nativeSHA(t, state)
		}
		result, err := commandsRunInstaller(t, f.base.Root, f.base.Project, f.base.env(filepath.Dir(f.client)), f.installer, stage,
			"skills", "add", nativeRawProjectSkillsSource, "--skill", "benchmark-note", "--skill", "benchmark-secondary", "--target", "codex", "--scope", "project", "--project-root", f.base.Project, "--yes", "--no-ui")
		items, _ := result["items"].([]any)
		if err != nil || result["status"] != "succeeded" || result["readiness"] != "ready" || len(items) != 2 {
			t.Fatalf("two raw Skills did not become automatically ready: %v %+v", err, result)
		}
		codexNativeProjectRawState(t, state)
		codexNativeProjectRawInventory(t, f, stage, true)
		if before != "" && nativeSHA(t, state) != before {
			t.Fatal("unchanged raw Skills rewrote the saved project state")
		}
		f.requireForeign(t)
		stages[stage] = nativeStage{Status: "passed", Reason: "ordinary public Git Skills batch reports ready for both bindings; exact enabled repository paths, pinned bytes and foreign files independently verified"}
	}
	nativeAttempt(stages, "project_raw_skills_remove", "project-raw-remove-installer.json")
	for name := range nativeRawProjectSkills {
		f.run(t, "project-raw-remove-"+name, "remove", name)
	}
	codexNativeProjectRawInventory(t, f, "project-raw-removed", false)
	stages["project_raw_skills_remove"] = nativeStage{Status: "passed", Reason: "ordinary project removal removes both raw Skills from fresh native discovery without changing foreign skills or user/project-B configuration"}
}

func codexNativeProjectRawState(t *testing.T, path string) {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var state domain.StateFileV2
	if err := json.Unmarshal(body, &state); err != nil {
		t.Fatal(err)
	}
	found := 0
	for _, installation := range state.Installations {
		for _, binding := range installation.Clients {
			if binding.ClientID != "codex" {
				continue
			}
			if binding.Scope != "project" || binding.Activation != domain.ActivationActive || binding.Verification != domain.VerificationInstalled {
				t.Fatalf("raw Skill binding lacks automatic native proof: %+v", binding)
			}
			found++
		}
	}
	if found != 2 {
		t.Fatalf("expected two active raw Skill bindings, found %d", found)
	}
}

func codexNativeProjectRawInventory(t *testing.T, f *codexProjectNativeFixture, label string, present bool) {
	t.Helper()
	r := startNativeRPC(t, f.base, f.client, label)
	defer r.close()
	r.must(t, "initialize", map[string]any{"clientInfo": map[string]string{"name": "uap_raw_project_check", "version": "1"}, "capabilities": map[string]bool{"experimentalApi": true}})
	if err := json.NewEncoder(r.in).Encode(map[string]any{"method": "initialized"}); err != nil {
		t.Fatal(err)
	}
	var result struct {
		Data []struct {
			CWD    string
			Errors []any
			Skills []struct {
				Name, Path, Scope, PluginID string
				Enabled                     bool
			}
		}
	}
	if err := json.Unmarshal(r.must(t, "skills/list", map[string]any{"cwds": []string{f.base.Project}, "forceReload": true}), &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Data) != 1 || !openCodeSamePath(result.Data[0].CWD, f.base.Project) || len(result.Data[0].Errors) != 0 {
		t.Fatal("raw project native inventory has wrong context or discovery errors")
	}
	found := map[string]bool{}
	for _, skill := range result.Data[0].Skills {
		digest, selected := nativeRawProjectSkills[skill.Name]
		if !selected {
			continue
		}
		path := filepath.Join(f.base.Project, ".agents", "skills", skill.Name, "SKILL.md")
		if !present || found[skill.Name] || !skill.Enabled || skill.PluginID != "" || skill.Scope != "repo" || !openCodeSamePath(skill.Path, path) || nativeSHA(t, path) != digest {
			t.Fatalf("unexpected raw project Skill discovery: %+v", skill)
		}
		found[skill.Name] = true
	}
	if present && len(found) != len(nativeRawProjectSkills) {
		t.Fatal("native Codex did not discover both selected raw project Skills")
	}
}
