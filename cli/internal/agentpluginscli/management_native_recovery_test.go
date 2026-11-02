package agentpluginscli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/management"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// The real OpenCode projection/config adapters run here with a controlled host
// version probe. Separate SDK smoke proves the same path with the actual client.
func TestManagementRecoveryRequiresCompleteNativeProjectionAndReviewedRepair(t *testing.T) {
	repo := dependencyRepo(t)
	if err := os.MkdirAll(filepath.Join(repo, "skills", "second"), 0700); err != nil {
		t.Fatal(err)
	}
	dependencyWrite(t, filepath.Join(repo, "skills", "second", "SKILL.md"), "---\nname: second\ndescription: Second recovery fixture\n---\nKeep this Skill intact.\n")
	dependencyGit(t, repo, "add", ".")
	dependencyGit(t, repo, "commit", "-m", "test: second independently owned native Skill")
	sha := savedJourneyGit(t, repo, "rev-parse", "HEAD")
	fixture, _, _ := dependencySpecHarness(t, repo, dependencyClientSpec{id: domain.ClientOpenCode, executable: true, activation: "active"})
	detected, err := fixture.app.Detector.Detect(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	for _, client := range detected {
		if err := os.Chmod(client.ExecutablePath, 0700); err != nil {
			t.Fatal(err)
		}
	}
	backend := managementJourneyBackend(t, savedJourney{cli: fixture})
	managementApply(t, managementPrepare(t, backend, management.ChangeRequest{Action: management.Install, Source: "github:acme/demo@" + sha, TargetIds: []string{"profile-opencode"}}))
	profile := backend.config.Targets[0].ConfigRoot
	skill := filepath.Join(profile, "skills", "demo", "SKILL.md")
	second := filepath.Join(profile, "skills", "second", "SKILL.md")
	original, untouched := []byte(dependencyRead(t, skill)), []byte(dependencyRead(t, second))
	if coherent, _, err := backend.managementRecoveryCoherent(t.Context(), backend.app); err != nil || !coherent {
		t.Fatalf("fresh intact native environment was not coherent: %v, %v", coherent, err)
	}
	for _, surface := range []string{"one_of_two_skills", "native_config_entry"} {
		t.Run(surface, func(t *testing.T) {
			if surface == "one_of_two_skills" {
				if err := os.RemoveAll(filepath.Dir(skill)); err != nil {
					t.Fatal(err)
				}
			} else {
				dependencyWrite(t, filepath.Join(profile, "opencode.json"), `{"mcp":{},"theme":"system"}`)
			}
			view, err := backend.app.Installer.Inspect(t.Context())
			if err != nil || view.Recovery.Required || view.Installations[0].Bindings[0].ProjectionStatus != "" {
				t.Fatalf("fixture must lose only native projection, with no journal or private-package drift: %+v, %v", view, err)
			}
			stateBefore, err := backend.app.StateStore.Load()
			if err != nil {
				t.Fatal(err)
			}
			proof := backend.proveManagementRecovery(t.Context(), backend.app, management.Outcome{State: management.Succeeded, Result: management.OperationResult{Outcome: "completed"}})
			if proof.RecoverySettled || proof.State != management.Partial {
				t.Fatal("missing native projection falsely resolved accepted work", proof)
			}
			stateAfter, err := backend.app.StateStore.Load()
			if err != nil || !reflect.DeepEqual(stateBefore, stateAfter) {
				t.Fatal("read-only proof changed native receipts", err)
			}
			prepared := managementPrepare(t, backend, management.ChangeRequest{Action: management.Recover})
			if len(prepared.Review().Actions) != 1 || prepared.Review().Actions[0].Action != "repair" {
				t.Fatal("recovery did not show the required native repair before acceptance", prepared.Review())
			}
			if surface == "one_of_two_skills" {
				if _, err := os.Stat(skill); !os.IsNotExist(err) {
					t.Fatal("preparation silently repaired native files", err)
				}
			}
			outcome := managementApply(t, prepared)
			if !outcome.RecoverySettled || !outcome.Result.Mutated {
				t.Fatal("explicit native repair did not prove a settled environment", outcome)
			}
			if !bytes.Equal([]byte(dependencyRead(t, skill)), original) || !bytes.Equal([]byte(dependencyRead(t, second)), untouched) {
				t.Fatal("native repair did not restore the exact recorded Skill bytes")
			}
			var config map[string]any
			if err := json.Unmarshal([]byte(dependencyRead(t, filepath.Join(profile, "opencode.json"))), &config); err != nil {
				t.Fatal(err)
			}
			if config["mcp"].(map[string]any)["demo"] == nil {
				t.Fatal("native repair did not restore the MCP entry")
			}
			if surface == "native_config_entry" && config["theme"] != "system" {
				t.Fatal("native repair replaced unrelated configuration")
			}
		})
	}
}
