package agentpluginscli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/management"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

// These are application integration tests: real Git, source loading, graph,
// persistence and installer transactions, with controlled client/scanner seams.
func managementJourneyBackend(t *testing.T, j savedJourney) *ManagementBackend {
	t.Helper()
	detected, err := j.cli.app.Detector.Detect(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	cfg := management.Startup{StateRoot: filepath.Dir(j.cli.app.ManagedRoot), DependenciesRoot: j.cli.app.DependenciesRoot, Scope: management.User, Targets: []management.StartupTarget{}}
	for _, target := range detected {
		if err := os.WriteFile(target.ExecutablePath, []byte("fixture executable"), 0700); err != nil {
			t.Fatal(err)
		}
		cfg.Targets = append(cfg.Targets, management.StartupTarget{Id: "profile-" + string(target.ClientID), Client: string(target.ClientID), ConfigRoot: target.ConfigRoot, Executable: target.ExecutablePath})
	}
	backend, err := NewManagementBackend(j.cli.app, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(backend.Close)
	return backend
}
func managementPrepare(t *testing.T, b *ManagementBackend, request management.ChangeRequest) management.Prepared {
	t.Helper()
	prepared, err := b.Prepare(t.Context(), request, newSkillCatalogID())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(prepared.Close)
	if !prepared.Review().Confirmable {
		t.Fatal(prepared.Review())
	}
	return prepared
}
func managementApply(t *testing.T, prepared management.Prepared) management.Outcome {
	t.Helper()
	if err := prepared.Check(t.Context()); err != nil {
		t.Fatal(err)
	}
	out, err := prepared.Apply(t.Context(), nil)
	if err != nil || out.State != management.Succeeded {
		t.Fatalf("apply: %+v, %v", out, err)
	}
	return out
}
func managementInstallRequest(source string) management.ChangeRequest {
	return management.ChangeRequest{Action: management.Install, Source: source, TargetIds: []string{"profile-codex"}}
}
func TestManagementRetainsPreparationAndInstallerIdentity(t *testing.T) {
	repo, sha := savedJourneyRepository(t)
	j := newSavedJourney(t, repo)
	backend := managementJourneyBackend(t, j)
	prepared := managementPrepare(t, backend, managementInstallRequest("github:acme/demo@"+sha))
	manifest, lockPath := j.cli.app.dependencyPaths()
	if _, err := os.Stat(manifest); !os.IsNotExist(err) {
		t.Fatal("review published intent", err)
	}
	ids := prepared.OperationIDs()
	if len(ids) != 1 {
		t.Fatal("missing accepted installer identity", ids)
	}
	result := managementApply(t, prepared)
	if !result.Result.Mutated || len(result.Result.Items) != 1 {
		t.Fatal(result)
	}
	binding := savedJourneyBinding(t, j.state(t), "codex")
	found := false
	for _, receipt := range binding.Receipts {
		found = found || slices.Contains(ids, receipt.OperationID)
	}
	if !found {
		t.Fatal("native receipt lost accepted operation correlation", ids, binding.Receipts)
	}
	view, err := backend.Inspect(t.Context())
	if err != nil || len(view.Entries) != 1 || len(view.Entries[0].Bindings) != 1 || len(view.Entries[0].Bindings[0].ManagedPaths) == 0 {
		t.Fatal(view, err)
	}
	oldLock, err := os.ReadFile(lockPath)
	if err != nil {
		t.Fatal(err)
	}
	pinned := managementPrepare(t, backend, management.ChangeRequest{Action: management.Update, All: true})
	managementApply(t, pinned)
	newLock, _ := os.ReadFile(lockPath)
	if !reflect.DeepEqual(oldLock, newLock) {
		t.Fatal("bulk update changed an exact commit")
	}
	dependencyWrite(t, filepath.Join(binding.TargetLocator, "drift.txt"), "damaged projection")
	managementApply(t, managementPrepare(t, backend, management.ChangeRequest{Action: management.Repair, Aliases: []string{"demo"}}))
	if _, err := os.Stat(filepath.Join(binding.TargetLocator, "drift.txt")); !os.IsNotExist(err) {
		t.Fatal("repair retained projection drift", err)
	}
	managementApply(t, managementPrepare(t, backend, management.ChangeRequest{Action: management.Remove, Aliases: []string{"demo"}, TargetIds: []string{"profile-codex"}, ExternalUninstalled: true}))
	if len(j.state(t).EnvironmentReceipts) != 0 {
		t.Fatal("remove retained live environment receipts")
	}
}
func TestManagementStaleReviewAndCancellationDoNotPublish(t *testing.T) {
	repo, sha := savedJourneyRepository(t)
	j := newSavedJourney(t, repo)
	backend := managementJourneyBackend(t, j)
	request := managementInstallRequest("github:acme/demo@" + sha)
	cancelled := managementPrepare(t, backend, request)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	out, err := cancelled.Apply(ctx, nil)
	if !errors.Is(err, context.Canceled) || out.State != management.Cancelled || out.Result.Mutated {
		t.Fatal(out, err)
	}
	stale := managementPrepare(t, backend, request)
	manifest, lock := j.cli.app.dependencyPaths()
	if err := os.MkdirAll(filepath.Dir(manifest), 0700); err != nil {
		t.Fatal(err)
	}
	dependencyWrite(t, manifest, "# external owner edit\n")
	if err := stale.Check(t.Context()); err == nil {
		t.Fatal("changed manifest retained authority")
	}
	out, err = stale.Apply(t.Context(), nil)
	if err == nil || out.Result.Mutated || j.runner.calls != 0 {
		t.Fatal("stale application performed effects", out, err)
	}
	if _, err := os.Stat(lock); !os.IsNotExist(err) {
		t.Fatal("stale application published lock", err)
	}
	if got := dependencyRead(t, manifest); got != "# external owner edit\n" {
		t.Fatal("stale application overwrote owner edit", got)
	}
}
func TestManagementRejectsForeignCapturedProfile(t *testing.T) {
	repo, sha := savedJourneyRepository(t)
	j := newSavedJourney(t, repo)
	backend := managementJourneyBackend(t, j)
	managementApply(t, managementPrepare(t, backend, managementInstallRequest("github:acme/demo@"+sha)))
	cfg := backend.config
	cfg.Targets = append([]management.StartupTarget{}, cfg.Targets...)
	for i := range cfg.Targets {
		if cfg.Targets[i].Client == "codex" {
			cfg.Targets[i].ConfigRoot = t.TempDir()
		}
	}
	foreign, err := NewManagementBackend(j.cli.app, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer foreign.Close()
	observed, err := foreign.Inspect(t.Context())
	if err != nil || len(observed.Entries) != 1 {
		t.Fatal(observed, err)
	}
	binding := observed.Entries[0].Bindings[0]
	if binding.Status != "outside_context" || binding.Readiness != nil || observed.Entries[0].UpdateStatus != management.EnvironmentEntryUpdateStatusBlocked {
		t.Fatal("old profile appeared ready in a new context", observed)
	}
	_, err = foreign.Prepare(t.Context(), management.ChangeRequest{Action: management.Recover}, "different-context")
	if err == nil {
		t.Fatal("recovery admitted a binding from another physical profile")
	}
}
func TestManagementNonterminalNativeRecoveryDoesNotSettle(t *testing.T) {
	repo, _ := savedJourneyRepository(t)
	backend := managementJourneyBackend(t, newSavedJourney(t, repo))
	prepared := managementRecovery{backend: backend}
	out := prepared.recoveryOutcome(t.Context(), installer.Result{Outcome: installer.OutcomeIncomplete}, nil)
	if out.RecoverySettled || out.State == management.Succeeded {
		t.Fatal("nil error erased a nonterminal native outcome", out)
	}
}
