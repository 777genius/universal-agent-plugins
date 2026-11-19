package pluginkitairepo_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// This existing immutable Anthropic fixture deliberately shares its name with
// Codex's built-in Skill. The browser's docx/frontend-design batch stays separate.
const nativeAnthropicSkillCreatorBlob = "65b3a402dbd09b8e83f9d637c6b553875189085c"

func codexNativeProjectSystemSkill(t *testing.T, f *codexProjectNativeFixture, stages map[string]nativeStage) {
	t.Helper()
	nativeAttempt(stages, "project_system_skill_install", "project-system-skill-add-installer.json")
	codexNativeSystemSkillInventory(t, f, "project-system-skill-before", false)
	builtin := filepath.Join(f.base.CodexHome, "skills", ".system", "skill-creator")
	builtinBefore := nativeTreeDigest(t, builtin)
	fixture := &browserProjectFixture{codex: f, pins: map[string]string{"skill-creator": nativeAnthropicSkillCreatorBlob}}
	wanted := map[string]bool{"skill-creator/codex": true}
	state := filepath.Join(f.base.Project, ".uap", "state-v2.json")
	var original map[string]browserProjectBinding
	var beforeState string
	var beforeTrees map[string]string
	for _, step := range []struct{ stage, label, status string }{
		{"project_system_skill_install", "project-system-skill-add", "succeeded"},
		{"project_system_skill_repeat", "project-system-skill-repeat", "unchanged"},
	} {
		nativeAttempt(stages, step.stage, step.label+"-installer.json")
		result, err := commandsRunInstaller(t, f.base.Root, f.base.Project, f.base.env(filepath.Dir(f.client)), f.installer, step.label,
			"skills", "add", nativeAnthropicSource, "--skill", "skill-creator", "--target", "codex", "--scope", "project", "--project-root", f.base.Project, "--yes", "--no-ui")
		codexNativeSystemSkillReady(t, result, err, step.status)
		bindings := fixture.state(t, wanted)
		codexNativeSystemSkillInventory(t, f, step.label, true)
		if nativeTreeDigest(t, builtin) != builtinBefore {
			t.Fatal("selected project Skill changed the genuine built-in Skill")
		}
		if original == nil {
			original, beforeState, beforeTrees = bindings, nativeSHA(t, state), browserProjectTrees(t, bindings)
		} else if nativeSHA(t, state) != beforeState || !reflect.DeepEqual(bindings, original) ||
			!reflect.DeepEqual(browserProjectTrees(t, bindings), beforeTrees) {
			t.Fatal("repeated Skill add changed saved identity, state or owned bytes")
		}
		stages[step.stage] = nativeStage{Status: "passed", Reason: "pinned Anthropic skill-creator reports Ready; fresh native repo/system rows coexist, project B excludes the owned Skill, and built-in/foreign bytes are unchanged"}
	}
	nativeAttempt(stages, "project_system_skill_remove", "project-system-skill-remove-installer.json")
	item := original["skill-creator/codex"]
	f.run(t, "project-system-skill-remove", "remove", item.installation)
	fixture.state(t, map[string]bool{})
	listed := f.run(t, "project-system-skill-removed-list", "list")
	data, _ := listed["data"].(map[string]any)
	installations, ok := data["installations"].([]any)
	if !ok || len(installations) != 0 {
		t.Fatal("removed Skill remains in the public selected project inventory")
	}
	for _, path := range []string{item.binding.TargetLocator, item.payload} {
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			t.Fatalf("removed owned Skill path remains: %s: %v", path, err)
		}
	}
	codexNativeSystemSkillInventory(t, f, "project-system-skill-removed", false)
	if nativeTreeDigest(t, builtin) != builtinBefore {
		t.Fatal("removing the selected Skill changed the genuine built-in Skill")
	}
	stages["project_system_skill_remove"] = nativeStage{Status: "passed", Reason: "public removal by saved installation ID clears its record and owned paths; fresh A/B discovery retains only the unchanged built-in and foreign Skills"}
}

func codexNativeSystemSkillReady(t *testing.T, result map[string]any, runErr error, status string) {
	t.Helper()
	items, _ := result["items"].([]any)
	if runErr != nil || result["result"] != "success" || result["status"] != "succeeded" || result["readiness"] != "ready" || len(items) != 1 ||
		(result["next_actions"] != nil && !reflect.DeepEqual(result["next_actions"], []any{})) ||
		(result["next_action"] != nil && result["next_action"] != "") {
		t.Fatalf("selected Skill did not finish public Ready without setup actions: %v %+v", runErr, result)
	}
	item, _ := items[0].(map[string]any)
	if item["alias"] != "skill-creator" || item["targetId"] != "codex" || item["status"] != status {
		t.Fatalf("unexpected selected Skill operation: %+v", item)
	}
}

func codexNativeSystemSkillInventory(t *testing.T, f *codexProjectNativeFixture, label string, present bool) {
	t.Helper()
	for _, project := range []string{f.base.Project, f.otherProject} {
		listing := f.skillListing(t, label+"-"+filepath.Base(project), project)
		if err := codexSystemSkillInventory(listing, project, f.base.CodexHome, present && project == f.base.Project); err != nil {
			t.Fatal(err)
		}
	}
	f.requireForeign(t)
}

// Unlike the existing command-Skill parser, this case requires both same-named
// rows. Neither an unrelated path nor the built-in can satisfy the owned row.
func codexSystemSkillInventory(listing []byte, project, profile string, present bool) error {
	var decoded struct {
		Data []struct {
			CWD    string
			Errors []any
			Skills []struct {
				Name, Path, Scope, PluginID string
				Enabled                     bool
			}
		}
	}
	if err := json.Unmarshal(listing, &decoded); err != nil {
		return err
	}
	if len(decoded.Data) != 1 || !openCodeSamePath(decoded.Data[0].CWD, project) || decoded.Data[0].Errors == nil || len(decoded.Data[0].Errors) != 0 {
		return fmt.Errorf("same-name Skill inventory has wrong cwd or discovery errors")
	}
	owned := filepath.Join(project, ".agents", "skills", "skill-creator", "SKILL.md")
	builtin := filepath.Join(profile, "skills", ".system", "skill-creator", "SKILL.md")
	foreign := filepath.Join(project, ".agents", "skills", "foreign-codex-project", "SKILL.md")
	seen := map[string]bool{}
	for _, skill := range decoded.Data[0].Skills {
		var key string
		scope := "repo"
		switch {
		case skill.Name == "skill-creator" && openCodeSamePath(skill.Path, builtin):
			key, scope = "builtin", "system"
		case skill.Name == "skill-creator" && present && openCodeSamePath(skill.Path, owned):
			key = "owned"
		case skill.Name == "skill-creator":
			return fmt.Errorf("unexpected same-name Skill path: %s", skill.Path)
		case skill.Name == "foreign-codex-project" && openCodeSamePath(skill.Path, foreign):
			key = "foreign"
		default:
			continue
		}
		if seen[key] || !skill.Enabled || skill.Scope != scope || skill.PluginID != "" {
			return fmt.Errorf("invalid or duplicate %s Skill row", key)
		}
		seen[key] = true
	}
	if !seen["builtin"] || !seen["foreign"] || seen["owned"] != present {
		return fmt.Errorf("missing exact owned, built-in or foreign Skill row: %v", seen)
	}
	return nil
}

func TestCodexSystemSkillInventory(t *testing.T) {
	project, profile := filepath.Join(t.TempDir(), "project"), filepath.Join(t.TempDir(), "codex")
	for _, name := range []string{"installed", "removed", "reversed", "wrong-cwd", "errors", "missing-errors", "null-errors", "missing-owned", "missing-builtin", "missing-foreign", "disabled-owned", "disabled-builtin", "plugin-owned", "plugin-builtin", "scope-owned", "scope-builtin", "path-owned", "path-builtin", "duplicate-owned", "duplicate-builtin", "duplicate-foreign", "removed-owned-leak"} {
		t.Run(name, func(t *testing.T) {
			owned := map[string]any{"name": "skill-creator", "path": filepath.Join(project, ".agents", "skills", "skill-creator", "SKILL.md"), "scope": "repo", "enabled": true}
			builtin := map[string]any{"name": "skill-creator", "path": filepath.Join(profile, "skills", ".system", "skill-creator", "SKILL.md"), "scope": "system", "enabled": true}
			foreign := map[string]any{"name": "foreign-codex-project", "path": filepath.Join(project, ".agents", "skills", "foreign-codex-project", "SKILL.md"), "scope": "repo", "enabled": true}
			group := map[string]any{"cwd": project, "errors": []any{}, "skills": []any{owned, builtin, foreign}}
			present := name != "removed" && name != "removed-owned-leak"
			switch name {
			case "removed", "missing-owned":
				group["skills"] = []any{builtin, foreign}
			case "reversed":
				group["skills"] = []any{builtin, owned, foreign}
			case "wrong-cwd":
				group["cwd"] = project + "-other"
			case "errors":
				group["errors"] = []any{"discovery failed"}
			case "missing-errors":
				delete(group, "errors")
			case "null-errors":
				group["errors"] = nil
			case "missing-builtin":
				group["skills"] = []any{owned, foreign}
			case "missing-foreign":
				group["skills"] = []any{owned, builtin}
			case "disabled-owned":
				owned["enabled"] = false
			case "disabled-builtin":
				builtin["enabled"] = false
			case "plugin-owned":
				owned["pluginId"] = "foreign@marketplace"
			case "plugin-builtin":
				builtin["pluginId"] = "foreign@marketplace"
			case "scope-owned":
				owned["scope"] = "system"
			case "scope-builtin":
				builtin["scope"] = "repo"
			case "path-owned":
				owned["path"] = filepath.Join(project+"-other", ".agents", "skills", "skill-creator", "SKILL.md")
			case "path-builtin":
				builtin["path"] = filepath.Join(profile+"-other", "skills", ".system", "skill-creator", "SKILL.md")
			case "duplicate-owned":
				group["skills"] = []any{owned, builtin, foreign, owned}
			case "duplicate-builtin":
				group["skills"] = []any{owned, builtin, foreign, builtin}
			case "duplicate-foreign":
				group["skills"] = []any{owned, builtin, foreign, foreign}
			}
			body, err := json.Marshal(map[string]any{"data": []any{group}})
			if err != nil {
				t.Fatal(err)
			}
			err = codexSystemSkillInventory(body, project, profile, present)
			wantOK := name == "installed" || name == "removed" || name == "reversed"
			if (err == nil) != wantOK {
				t.Fatalf("same-name inventory %s: %v", name, err)
			}
		})
	}
}
