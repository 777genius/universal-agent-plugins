package agentpluginscli

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/cli/internal/agentpluginscli/management"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/dirswap"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/installer"
)

func TestManagementRemovalAttestationBlocksAcceptanceAndPublication(t *testing.T) {
	repo, sha := savedJourneyRepository(t)
	j := newSavedJourney(t, repo)
	backend := managementJourneyBackend(t, j)
	managementApply(t, managementPrepare(t, backend, managementInstallRequest("github:acme/demo@"+sha)))
	manifest, lock := j.cli.app.dependencyPaths()
	beforeManifest, beforeLock := dependencyRead(t, manifest), dependencyRead(t, lock)
	beforeState, calls := j.state(t), j.runner.calls
	request := management.ChangeRequest{Action: management.Remove, Aliases: []string{"demo"}, TargetIds: []string{"profile-codex"}}
	prepared, err := backend.Prepare(t.Context(), request, "blocked-native-removal")
	if err != nil {
		t.Fatal(err)
	}
	defer prepared.Close()
	assertManagementRemovalConflict(t, prepared.Review())
	if err = prepared.Check(t.Context()); err == nil {
		t.Fatal("a blocked review passed its authoritative check")
	}
	out, err := prepared.Apply(t.Context(), nil)
	if err == nil || out.Result.Mutated {
		t.Fatal("direct application bypassed the missing attestation", out, err)
	}
	server := managementRemovalServer(t, backend)
	var review management.Plan
	managementRemovalHTTP(t, server, "POST", "/api/v1/plans", request, 200, &review)
	assertManagementRemovalConflict(t, review)
	var failure management.Error
	managementRemovalHTTP(t, server, "POST", "/api/v1/plans/"+review.PlanId+"/apply", management.ApplyRequest{RequestId: "blocked-native-remove-0001"}, 409, &failure)
	if failure.Code != "PLAN_STALE" {
		t.Fatal("nonconfirmable review was accepted", failure)
	}
	if _, err = os.Stat(filepath.Join(backend.config.StateRoot, "management")); !os.IsNotExist(err) {
		t.Fatal("a blocked review wrote an accepted operation", err)
	}
	if beforeManifest != dependencyRead(t, manifest) || beforeLock != dependencyRead(t, lock) || !reflect.DeepEqual(beforeState, j.state(t)) || calls != j.runner.calls {
		t.Fatal("missing attestation changed intent, receipts or native client state")
	}
	request.ExternalUninstalled = true
	confirmed := managementPrepare(t, backend, request)
	assertManagementRemovalAttested(t, confirmed.Review())
	managementApply(t, confirmed)
	if len(j.state(t).EnvironmentReceipts) != 0 {
		t.Fatal("explicitly confirmed removal retained owned receipts")
	}
}

func TestManagementRemovalAttestationFollowsPreparedActions(t *testing.T) {
	repo, sha := savedJourneyRepository(t)
	j := newSavedJourney(t, repo)
	backend := managementJourneyBackend(t, j)
	managementApply(t, managementPrepare(t, backend, managementInstallRequest("github:acme/demo@"+sha)))
	_, err := backend.Prepare(t.Context(), management.ChangeRequest{Action: management.Sync, ExternalUninstalled: true}, "irrelevant-attestation")
	var refusal *management.Refusal
	if !errors.As(err, &refusal) || refusal.Code != "EXTERNAL_UNINSTALL_NOT_APPLICABLE" {
		t.Fatal("attestation without a requiring removal was accepted", err)
	}
	// The existing CLI publishes removed intent before it refuses unattested
	// Codex removal. Keep this unchanged behavior as a real divergence fixture.
	if _, _, err = j.cli.execute(false, "remove", "demo", "--format", "json"); err == nil {
		t.Fatal("legacy Codex removal manufactured external confirmation")
	}
	request := management.ChangeRequest{Action: management.Sync}
	blocked, err := backend.Prepare(t.Context(), request, "sync-requiring-removal")
	if err != nil {
		t.Fatal(err)
	}
	defer blocked.Close()
	assertManagementRemovalConflict(t, blocked.Review())
	request.ExternalUninstalled = true
	confirmed := managementPrepare(t, backend, request)
	assertManagementRemovalAttested(t, confirmed.Review())
	managementApply(t, confirmed)
	if len(j.state(t).EnvironmentReceipts) != 0 {
		t.Fatal("sync did not apply its explicitly attested removal")
	}
}

func TestManagementRemovalPolicyComesFromFacadePlan(t *testing.T) {
	repo, sha := savedJourneyRepository(t)
	j := newSavedJourney(t, repo)
	backend := managementJourneyBackend(t, j)
	managementApply(t, managementPrepare(t, backend, managementInstallRequest("github:acme/demo@"+sha)))
	binding := savedJourneyBinding(t, j.state(t), "codex")
	installationID := j.state(t).EnvironmentReceipts[0].InstallationID
	for _, supplied := range []bool{false, true} {
		prepared, err := backend.app.Installer.Prepare(t.Context(), installer.Request{
			Operation: installer.OpRemove, ClientID: "codex", ClientConfigRoot: binding.NativeProfileRoot,
			InstallationID: installationID, OperationID: "facade-requirement", ExternalUninstalled: supplied,
		})
		if err != nil {
			t.Fatal(err)
		}
		plan := prepared.Plan()
		if err = prepared.Close(); err != nil {
			t.Fatal(err)
		}
		if !plan.RequiresExternalUninstalled || plan.NoChange {
			t.Fatal("facade lost the adapter requirement or replaced it with attestation state", plan)
		}
	}
}

func assertManagementRemovalConflict(t *testing.T, plan management.Plan) {
	t.Helper()
	if plan.Confirmable || len(plan.Conflicts) != 1 {
		t.Fatal("missing exact nonconfirmable removal conflict", plan)
	}
	conflict := plan.Conflicts[0]
	if conflict.Code != "EXTERNAL_UNINSTALL_REQUIRED" || conflict.Alias != "demo" || conflict.TargetId != "profile-codex" {
		t.Fatal("removal conflict lost its binding", conflict)
	}
}

func assertManagementRemovalAttested(t *testing.T, plan management.Plan) {
	t.Helper()
	if !plan.Confirmable || len(plan.Conflicts) != 0 {
		t.Fatal("explicit confirmation did not produce a fresh confirmable review", plan)
	}
	for _, warning := range plan.Warnings {
		if warning.Code == "EXTERNAL_UNINSTALL_ATTESTED" && warning.Alias == "demo" && warning.TargetId == "profile-codex" {
			return
		}
	}
	t.Fatal("review omitted the exact attested removal binding", plan)
}

func TestManagementReceiptRecoveryCannotConsumeExternalUninstalled(t *testing.T) {
	repo, _ := savedJourneyRepository(t)
	j := newSavedJourney(t, repo)
	backend := managementJourneyBackend(t, j)
	owned := j.cli.app.ManagedRoot
	staging := filepath.Join(owned, ".agentplugins-staging-pending-removal-review")
	if err := os.MkdirAll(staging, 0700); err != nil {
		t.Fatal(err)
	}
	manager := dirswap.Manager{JournalDir: j.cli.operations, Fault: func(phase string) error {
		if phase == dirswap.PhaseBackupPending {
			return errors.New("fixture interruption before any native removal")
		}
		return nil
	}}
	receipt, err := manager.Apply(t.Context(), dirswap.Input{
		OperationID: "pending-removal-review", ClientBindingID: "fixture-pending-binding", Sequence: 1,
		OwnedBase: owned, ActivePath: filepath.Join(owned, "pending-removal-review"), StagingPath: staging, RequireAbsent: true,
	})
	if err == nil || receipt.Phase != dirswap.PhaseBackupPending {
		t.Fatal("failed to retain actual directory-swap interruption", receipt, err)
	}
	before := savedJourneyTree(t, j.cli.operations)
	_, err = backend.Prepare(t.Context(), management.ChangeRequest{Action: management.Recover, ExternalUninstalled: true}, "attestation-on-receipts")
	var refusal *management.Refusal
	if !errors.As(err, &refusal) || refusal.Code != "EXTERNAL_UNINSTALL_NOT_APPLICABLE" {
		t.Fatal("receipt reconciliation accepted external-removal attestation", err)
	}
	plain := managementPrepare(t, backend, management.ChangeRequest{Action: management.Recover})
	if _, ok := plain.(*managementRecovery); !ok || !reflect.DeepEqual(before, savedJourneyTree(t, j.cli.operations)) {
		t.Fatal("receipt review changed journals or skipped the separate recovery phase")
	}
}
