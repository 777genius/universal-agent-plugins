package pluginkitairepo_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/claude"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// Reuse the same public immutable raw-Skills fixture as the Codex native gate.
// The installer selects both Skills; independent no-model native initialization
// proves that both become available in A and neither becomes available in B.
func claudeNativeProjectRawSkills(t *testing.T, f *claudeProjectNativeFixture, stages map[string]nativeStage) {
	t.Helper()
	var expected []string
	for _, stage := range []string{"project_raw_skills_install", "project_raw_skills_repeat"} {
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
		expected = claudeNativeRawProjectState(t, f, statePath)
		claudeProjectInitializedSkills(t, f, stage, expected, true)
		if before != "" && nativeSHA(t, statePath) != before {
			t.Fatal("unchanged raw Claude Skills rewrote saved project state")
		}
		stages[stage] = nativeStage{Status: "passed", Reason: "public Git batch reports ready for both Skills; exact owned and native-cache bytes match pins, no-token A initialization loads each once, B loads neither"}
	}
	nativeAttempt(stages, "project_raw_skills_remove", "project-raw-remove-installer.json")
	for name := range nativeRawProjectSkills {
		result, err := f.run(t, "project-raw-remove-"+name, "remove", name, "--target", "claude")
		if err != nil || result["result"] != "success" {
			t.Fatalf("raw Claude Skill removal failed: %v %+v", err, result)
		}
	}
	claudeProjectInitializedSkills(t, f, "project-raw-removed", expected, false)
	f.requireForeign(t)
	stages["project_raw_skills_remove"] = nativeStage{Status: "passed", Reason: "ordinary removal withdraws both selected raw Skills from fresh credential-free native A/B initialization and preserves foreign configuration"}
}

func claudeNativeRawProjectState(t *testing.T, f *claudeProjectNativeFixture, path string) []string {
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
				cacheSkill := filepath.Join(f.base.ConfigDir, "plugins", "cache", marketplace, installation.DeclaredName, "0.0.0", ".agentplugins-runtime", "skills", alias, "SKILL.md")
				if nativeSHA(t, projectSkill) != digest || nativeSHA(t, cacheSkill) != digest {
					t.Fatal("raw Claude project/native cache Skill bytes differ from the immutable source")
				}
				expected = append(expected, installation.DeclaredName+":"+alias)
				found = true
			}
		}
		if !found {
			t.Fatal("selected raw Claude Skill has no retained receipt")
		}
	}
	return expected
}
