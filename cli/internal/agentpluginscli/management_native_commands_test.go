package agentpluginscli

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/management"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

// The real SDK smoke uses the same package: a declared Skill plus a portable
// command rendered as a second native Skill, with an explicit-only command
// withheld. Exercise ordinary repair as well as subsequent fresh recovery.
func managementCommandsBackend(t *testing.T) *ManagementBackend {
	t.Helper()
	repo := t.TempDir()
	fixtureRoot := filepath.Join("..", "..", "..", "repotests", "testdata", "agentplugins_commands_fixture")
	if err := os.CopyFS(repo, os.DirFS(fixtureRoot)); err != nil {
		t.Fatal(err)
	}
	dependencyGit(t, repo, "init", "-b", "main")
	dependencyGit(t, repo, "add", ".")
	dependencyGit(t, repo, "commit", "-m", "test: exact command projection fixture")
	sha := savedJourneyGit(t, repo, "rev-parse", "HEAD")
	fixture, _, _ := dependencySpecHarness(t, repo, dependencyClientSpec{id: domain.ClientOpenCode, executable: true})
	backend := managementJourneyBackend(t, savedJourney{cli: fixture})
	managementApply(t, managementPrepare(t, backend, management.ChangeRequest{Action: management.Install, Source: "github:acme/demo@" + sha, Alias: "demo", TargetIds: []string{"profile-opencode"}}))
	return backend
}

func TestManagementRepairPreservesEditedSkillBesideRenderedCommand(t *testing.T) {
	backend := managementCommandsBackend(t)
	configRoot := backend.config.Targets[0].ConfigRoot
	// Real `opencode debug skill` creates this unrelated config after install.
	// The Skills-only stored projection still names its original JSON choice.
	configPath := filepath.Join(configRoot, "opencode.jsonc")
	config := "{\n  \"$schema\": \"https://opencode.ai/config.json\"\n}\n"
	dependencyWrite(t, configPath, config)
	managementApply(t, managementPrepare(t, backend, management.ChangeRequest{Action: management.Repair, Aliases: []string{"demo"}}))
	root := filepath.Join(configRoot, "skills")
	skillDir, commandDir := filepath.Join(root, "fixture-guide"), filepath.Join(root, "uap-commands-fixture-echo")
	skillFile := filepath.Join(skillDir, "SKILL.md")
	original, command := dependencyRead(t, skillFile), dependencyRead(t, filepath.Join(commandDir, "SKILL.md"))
	edited := original + "\nUnreviewed local fixture edit.\n"
	dependencyWrite(t, skillFile, edited)
	prepared := managementPrepare(t, backend, management.ChangeRequest{Action: management.Repair, Aliases: []string{"demo"}})
	out, err := prepared.Apply(t.Context(), nil)
	if !errors.Is(err, installer.ErrManagedSkillChanged) || out.State != management.Partial || out.RecoverySettled {
		t.Fatalf("ordinary repair lost the typed changed-content refusal: %+v; cause: %T: %v", out, err, err)
	}
	if dependencyRead(t, skillFile) != edited || dependencyRead(t, filepath.Join(commandDir, "SKILL.md")) != command {
		t.Fatal("refused repair changed existing native Skill bytes")
	}
	if !slices.ContainsFunc(out.Diagnostics, func(d management.Diagnostic) bool {
		return d.Code == "OWNED_SKILL_CHANGED" && d.Alias == "demo" && d.TargetId == "profile-opencode"
	}) || !slices.ContainsFunc(out.Result.NextActions, func(a management.NextAction) bool { return a.Kind == "preserve_changes" }) {
		t.Fatal("refused repair omitted bounded manual recovery guidance", out)
	}
	preserved := filepath.Join(t.TempDir(), "owner-preserved-guide")
	if err := os.Rename(skillDir, preserved); err != nil {
		t.Fatal(err)
	}
	recovered := managementApply(t, managementPrepare(t, backend, management.ChangeRequest{Action: management.Recover}))
	if !recovered.RecoverySettled || dependencyRead(t, skillFile) != original || dependencyRead(t, filepath.Join(preserved, "SKILL.md")) != edited || dependencyRead(t, configPath) != config {
		t.Fatal("fresh recovery failed to converge after explicit preservation", recovered)
	}
}
