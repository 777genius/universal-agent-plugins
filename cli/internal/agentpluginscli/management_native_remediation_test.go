package agentpluginscli

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/management"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

func managementNativeRemediationFixture(t *testing.T, id domain.ClientID) (*ManagementBackend, cliFixture, *dependencyFixtureRunner) {
	t.Helper()
	repo := dependencyRepo(t)
	sha := savedJourneyGit(t, repo, "rev-parse", "HEAD")
	fixture, _, runner := dependencySpecHarness(t, repo, dependencyClientSpec{id: id, executable: true})
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
	managementApply(t, managementPrepare(t, backend, management.ChangeRequest{Action: management.Install, Source: "github:acme/demo@" + sha, TargetIds: []string{"profile-" + string(id)}}))
	return backend, fixture, runner
}

func TestManagementRecoveryPreservesEditedNativeSkillUntilExplicitResolution(t *testing.T) {
	backend, _, _ := managementNativeRemediationFixture(t, domain.ClientOpenCode)
	skillDir := filepath.Join(backend.config.Targets[0].ConfigRoot, "skills", "demo")
	skillFile := filepath.Join(skillDir, "SKILL.md")
	original := dependencyRead(t, skillFile)
	changed := "---\nname: demo\ndescription: Owner's edited Skill\n---\nRetain these owner-authored instructions.\n"
	dependencyWrite(t, skillFile, changed)
	prepared := managementPrepare(t, backend, management.ChangeRequest{Action: management.Recover})
	out, err := prepared.Apply(t.Context(), nil)
	if !errors.Is(err, installer.ErrManagedSkillChanged) || out.State != management.Partial || out.RecoverySettled {
		t.Fatal("edited owned content was not a typed unresolved conflict", out, err)
	}
	if got := dependencyRead(t, skillFile); got != changed {
		t.Fatal("recovery replaced the owner's changed bytes", got)
	}
	view, inspectErr := backend.app.Installer.Inspect(t.Context())
	if inspectErr != nil || view.Recovery.Required {
		t.Fatal("known unchanged native refusal left an unrecoverable attempt marker", view.Recovery, inspectErr)
	}
	if !slices.ContainsFunc(out.Diagnostics, func(d management.Diagnostic) bool {
		return d.Code == "OWNED_SKILL_CHANGED" && d.Alias == "demo" && d.TargetId == "profile-opencode"
	}) || !slices.ContainsFunc(out.Result.NextActions, func(a management.NextAction) bool { return a.Kind == "preserve_changes" }) {
		t.Fatal("typed refusal omitted actionable bounded guidance", out)
	}
	for _, action := range out.Result.NextActions {
		if action.Alias == "demo" && (action.Kind == "activate" || action.Kind == "manual") {
			t.Fatal("changed-content conflict retained misleading activation guidance", action)
		}
	}
	preserved := filepath.Join(t.TempDir(), "owner-preserved-demo")
	if err := os.Rename(skillDir, preserved); err != nil {
		t.Fatal(err)
	}
	settled := managementApply(t, managementPrepare(t, backend, management.ChangeRequest{Action: management.Recover}))
	if !settled.RecoverySettled || dependencyRead(t, skillFile) != original || dependencyRead(t, filepath.Join(preserved, "SKILL.md")) != changed {
		t.Fatal("fresh recovery failed to restore the recorded Skill while retaining the explicit owner backup", settled)
	}
}

func TestManagementRecoveryPreservesKnownManualReadiness(t *testing.T) {
	backend, fixture, runner := managementNativeRemediationFixture(t, domain.ClientCursor)
	before, err := backend.Inspect(t.Context())
	if err != nil || len(before.Entries) != 1 || before.Entries[0].Bindings[0].Readiness == nil {
		t.Fatal(before, err)
	}
	readiness := before.Entries[0].Bindings[0].Readiness
	if readiness.Activation != string(domain.ActivationManual) {
		t.Fatal("fixture must require real manual native activation", readiness)
	}
	stateBefore, mutations := dependencyRead(t, fixture.store.Path), runner.mutations
	out := managementApply(t, managementPrepare(t, backend, management.ChangeRequest{Action: management.Recover}))
	if !out.RecoverySettled || out.Result.Mutated || !slices.ContainsFunc(out.Result.NextActions, func(a management.NextAction) bool {
		return a.Kind == "readiness" && strings.Contains(a.Reason, "activation_manual")
	}) {
		t.Fatal("materialization proof hid manual readiness or retried native effects", out)
	}
	after, err := backend.Inspect(t.Context())
	if err != nil || !reflect.DeepEqual(readiness, after.Entries[0].Bindings[0].Readiness) || runner.mutations != mutations || dependencyRead(t, fixture.store.Path) != stateBefore {
		t.Fatal("recovery attested readiness or rewrote stable receipts", after, err)
	}
}
