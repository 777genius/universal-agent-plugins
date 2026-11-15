package pluginkitairepo_test

import (
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/claude"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

type claudeRawNativeBinding struct {
	installationID, target, cache string
}

// Reuse the same public immutable raw-Skills fixture as the Codex native gate.
// The installer selects both Skills; independent no-model native initialization
// proves that both become available in A and neither becomes available in B.
func claudeNativeProjectRawSkills(t *testing.T, f *claudeProjectNativeFixture, stages map[string]nativeStage) {
	t.Helper()
	var expected []string
	var original map[string]claudeRawNativeBinding
	for _, stage := range []string{"project_raw_skills_install", "project_raw_skills_repeat", "project_raw_skills_reinstall"} {
		if stage == "project_raw_skills_reinstall" {
			claudeNativeRawProjectRemove(t, f, stages, "project_raw_skills_remove", expected, original)
		}
		nativeAttempt(stages, stage, stage+"-installer.json")
		statePath := filepath.Join(f.project, ".uap", "state-v2.json")
		before := ""
		if stage == "project_raw_skills_repeat" {
			before = nativeSHA(t, statePath)
		}
		result, err := f.run(t, stage, "skills", "add", nativeRawProjectSkillsSource,
			"--skill", "benchmark-note", "--skill", "benchmark-secondary", "--target", "claude", "--yes", "--no-ui")
		items, _ := result["items"].([]any)
		if err != nil || result["status"] != "succeeded" || result["readiness"] != "ready" || len(items) != 2 {
			t.Fatalf("two raw Claude Skills did not become ready automatically: %v %+v", err, result)
		}
		var bindings map[string]claudeRawNativeBinding
		expected, bindings = claudeNativeRawProjectState(t, f, statePath)
		if original == nil {
			original = bindings
		} else if !maps.Equal(original, bindings) {
			t.Fatal("raw Claude Skill repeat/reinstall changed its retained installation identity or owned paths")
		}
		claudeProjectInitializedSkills(t, f, stage, expected, true)
		if before != "" && nativeSHA(t, statePath) != before {
			t.Fatal("unchanged raw Claude Skills rewrote saved project state")
		}
		stages[stage] = nativeStage{Status: "passed", Reason: "public Git batch reports ready for both Skills; exact owned and native-cache bytes match pins, no-token A initialization loads each once, B loads neither"}
	}
	claudeNativeRawProjectRemove(t, f, stages, "project_raw_skills_final_remove", expected, original)
}

func claudeNativeRawProjectRemove(t *testing.T, f *claudeProjectNativeFixture, stages map[string]nativeStage, stage string, expected []string, bindings map[string]claudeRawNativeBinding) {
	t.Helper()
	nativeAttempt(stages, stage, stage+"-installer.json")
	for name, binding := range bindings {
		result, err := f.run(t, stage+"-"+name, "remove", name, "--target", "claude")
		if err != nil || result["result"] != "success" {
			t.Fatalf("raw Claude Skill removal failed: %v %+v", err, result)
		}
		for _, path := range []string{binding.target, binding.cache} {
			if _, err := os.Lstat(path); !os.IsNotExist(err) {
				t.Fatalf("owned raw Claude Skill path survived removal: %s: %v", path, err)
			}
		}
	}
	claudeProjectInitializedSkills(t, f, stage, expected, false)
	f.requireForeign(t)
	stages[stage] = nativeStage{Status: "passed", Reason: "ordinary removal withdraws both selected raw Skills from fresh credential-free native A/B initialization, removes only their owned paths and caches, and preserves foreign configuration"}
}

func claudeNativeRawProjectState(t *testing.T, f *claudeProjectNativeFixture, path string) ([]string, map[string]claudeRawNativeBinding) {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var state domain.StateFileV2
	if err := json.Unmarshal(body, &state); err != nil {
		t.Fatal(err)
	}
	var expected []string
	bindings := map[string]claudeRawNativeBinding{}
	for alias, digest := range nativeRawProjectSkills {
		found := false
		for _, receipt := range state.EnvironmentReceipts {
			if receipt.Alias != alias || receipt.ClientID != "claude" {
				continue
			}
			for _, installation := range state.Installations {
				if installation.InstallationID != receipt.InstallationID {
					continue
				}
				binding, ok := installation.Clients[receipt.ClientBindingID]
				if !ok || found || binding.ClientID != "claude" || binding.Scope != "project" || binding.Activation != domain.ActivationActive ||
					binding.Authentication != domain.AuthenticationNotRequired || binding.Verification != domain.VerificationInstalled {
					t.Fatal("raw Claude Skill has no unique ready owned project binding")
				}
				nativeRequireContained(t, f.project, binding.TargetLocator)
				projectSkill := filepath.Join(binding.TargetLocator, ".agentplugins-plugin", ".agentplugins-runtime", "skills", alias, "SKILL.md")
				marketplace := claude.ProjectMarketplaceName(binding.PhysicalArtifact, f.project)
				cache := filepath.Join(f.base.ConfigDir, "plugins", "cache", marketplace, installation.DeclaredName, "0.0.0")
				cacheSkill := filepath.Join(cache, ".agentplugins-runtime", "skills", alias, "SKILL.md")
				if nativeSHA(t, projectSkill) != digest || nativeSHA(t, cacheSkill) != digest {
					t.Fatal("raw Claude project/native cache Skill bytes differ from the immutable source")
				}
				expected = append(expected, installation.DeclaredName+":"+alias)
				bindings[alias] = claudeRawNativeBinding{installationID: installation.InstallationID, target: binding.TargetLocator, cache: cache}
				found = true
			}
		}
		if !found {
			t.Fatal("selected raw Claude Skill has no retained receipt")
		}
	}
	return expected, bindings
}
