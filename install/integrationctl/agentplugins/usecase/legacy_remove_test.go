package usecase

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/dirswap"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/locks"
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
		StateStore: store, Legacy: legacy, LegacyLock: locks.FileLock{BaseDir: filepath.Join(root, "legacy-locks")},
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
	for _, initiallyPresent := range []bool{true, false} {
		initiallyPresent := initiallyPresent
		t.Run(map[bool]string{true: "delegated", false: "reconciled_after_interruption"}[initiallyPresent], func(t *testing.T) {
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
			legacy := &legacyLifecycleStub{exists: initiallyPresent}
			service := testService(Service{
				StateStore: store, Legacy: legacy, LegacyLock: locks.FileLock{BaseDir: filepath.Join(root, "legacy-locks")},
				Lock:   processlock.Lock{Path: filepath.Join(root, "mutation.lock")},
				Kernel: transaction.Kernel{StateStore: store, Directory: dirswap.Manager{JournalDir: filepath.Join(root, "operations")}},
			})
			result, err := service.RemoveLegacy(context.Background(), LegacyRemoveInput{
				Selector: installationID, Confirmed: true, OperationID: "legacy-remove",
			})
			if err != nil {
				t.Fatal(err)
			}
			if !result.Mutated || result.Reconciled == initiallyPresent {
				t.Fatalf("result = %+v", result)
			}
			wantCalls := 0
			if initiallyPresent {
				wantCalls = 1
			}
			if legacy.removeCalls != wantCalls {
				t.Fatalf("remove calls = %d, want %d", legacy.removeCalls, wantCalls)
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
