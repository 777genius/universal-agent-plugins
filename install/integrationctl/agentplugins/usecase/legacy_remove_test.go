package usecase

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/dirswap"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/locks"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/legacylock"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/processlock"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/statev2"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/transaction"
)

type legacyLifecycleStub struct {
	exists              bool
	removeCalls         int
	reappearAfterRemove bool
}

func (stub *legacyLifecycleStub) Exists(context.Context, string) (bool, error) {
	return stub.exists, nil
}

func (stub *legacyLifecycleStub) PlanRemove(context.Context, string) (ports.LegacyRemovalPlan, error) {
	return ports.LegacyRemovalPlan{Summary: "remove legacy", TargetIDs: []string{"codex"}}, nil
}

func (stub *legacyLifecycleStub) Remove(context.Context, string) (ports.LegacyRemovalPlan, error) {
	stub.removeCalls++
	stub.exists = stub.reappearAfterRemove
	return ports.LegacyRemovalPlan{Summary: "removed", TargetIDs: []string{"codex"}}, nil
}

func TestRemoveLegacyAbortsReconcileWhenLegacyStateReappears(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	store := statev2.Store{Path: filepath.Join(root, "state-v2.json")}
	installationID := "00000000-0000-4000-8000-000000000001"
	clientID := domain.ComputeClientBindingID(installationID, "codex", "user", "/legacy/plugin")
	state := domain.StateFileV2{SchemaVersion: domain.StateSchemaVersion, Installations: []domain.Installation{{
		InstallationID: installationID, DeclaredName: "legacy-demo",
		Source:  domain.SourceBinding{SourceBindingID: "src_legacy_race", RequestedSource: "legacy-demo", CanonicalSource: "legacy-demo"},
		Package: domain.PackageBinding{LoaderKind: domain.LoaderKindLegacy, FormatID: domain.FormatIDLegacyV1, SchemaURI: "plugin.yaml/v1", DeclaredName: "legacy-demo"},
		Clients: map[string]domain.ClientBinding{clientID: {
			ClientBindingID: clientID, ClientID: "codex", Scope: "user", TargetLocator: "/legacy/plugin",
			PhysicalArtifact: domain.ComputePhysicalArtifactID("legacy-demo", installationID), Materialization: domain.MaterializationMaterialized,
			Activation: domain.ActivationManual, Authentication: domain.AuthenticationNotRequired,
			Policy: domain.PolicyAllowed, Verification: domain.VerificationNotRun,
		}},
	}}}
	if err := store.Save(state); err != nil {
		t.Fatal(err)
	}
	legacy := &legacyLifecycleStub{exists: true, reappearAfterRemove: true}
	service := testService(Service{
		StateStore: store, Legacy: legacy, LegacyLock: legacylock.StateLock{Manager: locks.FileLock{BaseDir: filepath.Join(root, "legacy-locks")}},
		Lock:   processlock.Lock{Path: filepath.Join(root, "mutation.lock")},
		Kernel: transaction.Kernel{StateStore: store, Directory: dirswap.Manager{JournalDir: filepath.Join(root, "operations")}},
	})
	if _, err := service.RemoveLegacy(context.Background(), LegacyRemoveInput{Selector: installationID, Confirmed: true}); err == nil {
		t.Fatal("concurrent legacy reappearance was reconciled as absent")
	}
	loaded, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if onlyBinding(loaded.Installations[0]).Materialization != domain.MaterializationMaterialized {
		t.Fatal("State v2 was marked absent after legacy state reappeared")
	}
}

func TestRemoveLegacyDelegatesThenReconcilesStateV2(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name             string
		initiallyPresent bool
		failSave         bool
		failLock         bool
		nilRelease       bool
	}{
		{name: "delegated", initiallyPresent: true},
		{name: "reconciled_after_interruption"},
		{name: "v2_save_failure_releases_legacy_lock", failSave: true},
		{name: "old_lock_unavailable_preserves_v2", initiallyPresent: true, failLock: true},
		{name: "missing_release_refuses_reconciliation", initiallyPresent: true, nilRelease: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			store := statev2.Store{Path: filepath.Join(root, "state-v2.json")}
			installationID := "00000000-0000-4000-8000-000000000001"
			clientID := domain.ComputeClientBindingID(installationID, "codex", "user", "/legacy/plugin")
			state := domain.StateFileV2{SchemaVersion: domain.StateSchemaVersion, Installations: []domain.Installation{{
				InstallationID: installationID, DeclaredName: "legacy-demo",
				Source:  domain.SourceBinding{SourceBindingID: "src_legacy", RequestedSource: "legacy-demo", CanonicalSource: "legacy-demo"},
				Package: domain.PackageBinding{LoaderKind: domain.LoaderKindLegacy, FormatID: domain.FormatIDLegacyV1, SchemaURI: "plugin.yaml/v1", DeclaredName: "legacy-demo"},
				Clients: map[string]domain.ClientBinding{clientID: {
					ClientBindingID: clientID, ClientID: "codex", Scope: "user", TargetLocator: "/legacy/plugin",
					PhysicalArtifact: domain.ComputePhysicalArtifactID("legacy-demo", installationID), Materialization: domain.MaterializationMaterialized,
					Activation: domain.ActivationManual, Authentication: domain.AuthenticationNotRequired,
					Policy: domain.PolicyAllowed, Verification: domain.VerificationNotRun,
					NativeObjects: []domain.NativeObjectOwnership{{ObjectID: "object_one", Kind: "plugin_root", Path: "/legacy/plugin"}},
				}},
			}}}
			if err := store.Save(state); err != nil {
				t.Fatal(err)
			}
			legacy := &legacyLifecycleStub{exists: tc.initiallyPresent}
			legacySpy := &inspectedLegacyStateLock{failAcquire: tc.failLock, nilRelease: tc.nilRelease}
			var stateStore transaction.StateStore = store
			if tc.failSave {
				stateStore = refusingLegacyStateSave{StateStore: store}
			}
			service := testService(Service{
				StateStore: observingLegacyStateStore{StateStore: stateStore, lock: legacySpy},
				Legacy:     legacy, LegacyLock: legacySpy,
				Lock:   processlock.Lock{Path: filepath.Join(root, "mutation.lock")},
				Kernel: transaction.Kernel{StateStore: store, Directory: dirswap.Manager{JournalDir: filepath.Join(root, "operations")}},
			})
			result, err := service.RemoveLegacy(context.Background(), LegacyRemoveInput{
				Selector: installationID, Confirmed: true, OperationID: "legacy-remove",
			})
			if tc.failSave || tc.failLock || tc.nilRelease {
				if err == nil || result.Mutated {
					t.Fatalf("failed reconciliation = (%+v, %v), want no mutation and an error", result, err)
				}
				if legacySpy.held {
					t.Fatal("legacy state lock was retained after v2 save failure")
				}
				loaded, loadErr := store.Load()
				if loadErr != nil {
					t.Fatal(loadErr)
				}
				if onlyBinding(loaded.Installations[0]).Materialization != domain.MaterializationMaterialized {
					t.Fatal("failed v2 save overwrote the materialized installation")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !result.Mutated || result.Reconciled == tc.initiallyPresent {
				t.Fatalf("result = %+v", result)
			}
			wantCalls := 0
			if tc.initiallyPresent {
				wantCalls = 1
			}
			if legacy.removeCalls != wantCalls {
				t.Fatalf("remove calls = %d, want %d", legacy.removeCalls, wantCalls)
			}
			if legacySpy.held {
				t.Fatal("legacy state lock was not released after reconciliation")
			}
			updated, err := store.Load()
			if err != nil {
				t.Fatal(err)
			}
			client := onlyBinding(updated.Installations[0])
			if client.Materialization != domain.MaterializationAbsent || len(client.NativeObjects) != 0 || len(client.Receipts) != 1 {
				t.Fatalf("client = %+v", client)
			}
		})
	}
}

type inspectedLegacyStateLock struct {
	held        bool
	failAcquire bool
	nilRelease  bool
}

func (lock *inspectedLegacyStateLock) Acquire(context.Context) (ports.UnlockFunc, error) {
	if lock.failAcquire {
		return nil, fmt.Errorf("injected legacy lock acquisition failure")
	}
	if lock.nilRelease {
		return nil, nil
	}
	if lock.held {
		return nil, fmt.Errorf("legacy state lock reacquired while already held")
	}
	lock.held = true
	return func() error {
		lock.held = false
		return nil
	}, nil
}

// observingLegacyStateStore proves the state-v2 save occurs while the
// legacy state lock is held, not after the negative observation releases it.
type observingLegacyStateStore struct {
	transaction.StateStore
	lock *inspectedLegacyStateLock
}

func (store observingLegacyStateStore) Save(state domain.StateFileV2) error {
	if !store.lock.held {
		return fmt.Errorf("reconciliation wrote state-v2 without the legacy state lock")
	}
	return store.StateStore.Save(state)
}

type refusingLegacyStateSave struct{ transaction.StateStore }

func (refusingLegacyStateSave) Save(domain.StateFileV2) error {
	return fmt.Errorf("injected state-v2 save failure")
}
