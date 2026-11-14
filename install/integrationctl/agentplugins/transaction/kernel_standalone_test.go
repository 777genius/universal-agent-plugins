package transaction

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/dirswap"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/profileauthority"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/statev2"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func standaloneFixture(t *testing.T) (Kernel, DirectoryMutation, statev2.Store) {
	t.Helper()
	kernel, packageMutation, store := transactionFixture(t, "auxiliary-root")
	mutation := DirectoryMutation{
		Standalone: true, RequireAbsent: true, OperationID: packageMutation.OperationID,
		InstallationID: packageMutation.InstallationID, ClientBindingID: packageMutation.ClientBindingID,
		Sequence: 1, OwnedBase: packageMutation.OwnedBase,
		ActivePath: filepath.Join(packageMutation.OwnedBase, "session"), StagingPath: packageMutation.StagingPath,
		AfterDigest: "auxiliary-content", DesiredState: packageMutation.DesiredState, Verify: verifyTransactionBody("new"),
	}
	parent, err := profileauthority.Capture(t.Context(), mutation.OwnedBase)
	if err != nil {
		t.Fatal(err)
	}
	mutation.PublicationParentAuthority = &parent
	return kernel, mutation, store
}

func TestStandalonePublicationRetainsPackageAndSurvivesBindingRemoval(t *testing.T) {
	kernel, mutation, store := standaloneFixture(t)
	before, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := kernel.ApplyDirectory(t.Context(), mutation); err != nil {
		t.Fatal(err)
	}
	after, err := store.Load()
	if err != nil || !reflect.DeepEqual(before.Installations, after.Installations) {
		t.Fatalf("auxiliary publication changed package binding: %v", err)
	}
	if len(after.TransactionReceipts) != 1 || after.TransactionReceipts[0].Phase != ReceiptPhaseCommitted || !domain.SameProfileAuthority(after.TransactionReceipts[0].PublicationParentAuthority, mutation.PublicationParentAuthority) {
		t.Fatalf("no standalone receipt: %+v", after.TransactionReceipts)
	}
	assertTransactionBody(t, filepath.Join(mutation.OwnedBase, "plugin"), "old")
	assertTransactionBody(t, mutation.ActivePath, "new")
	after.Installations = nil
	if err := store.Save(after); err != nil {
		t.Fatal(err)
	}
	if err := kernel.Recover(t.Context()); err != nil {
		t.Fatal(err)
	}
	recovered, err := store.Load()
	if err != nil || !reflect.DeepEqual(after, recovered) {
		t.Fatalf("removed first binding lost standalone receipt: %v", err)
	}
}

func TestStandalonePublicationRefusesBindingChangesAndGroupsBeforeEffects(t *testing.T) {
	for _, change := range []string{"replace", "metadata", "state", "group", "missing-parent", "zero-parent", "wrong-parent"} {
		t.Run(change, func(t *testing.T) {
			kernel, mutation, store := standaloneFixture(t)
			before, err := store.Load()
			if err != nil {
				t.Fatal(err)
			}
			switch change {
			case "missing-parent":
				mutation.PublicationParentAuthority = nil
			case "zero-parent":
				mutation.PublicationParentAuthority = &domain.ProfileAuthority{}
			case "wrong-parent":
				other, err := profileauthority.Capture(t.Context(), filepath.Dir(mutation.OwnedBase))
				if err != nil {
					t.Fatal(err)
				}
				mutation.PublicationParentAuthority = &other
			case "replace":
				mutation.RequireAbsent = false
			case "metadata":
				mutation.NativeObjects = []domain.NativeObjectOwnership{{Kind: "foreign"}}
			case "state":
				mutation.DesiredState.Installations[0].UpdatedAt = "different"
			}
			if change == "group" {
				_, err = kernel.ApplyDirectoryGroup(t.Context(), DirectoryGroup{OperationGroupID: "bad-group", Mutations: []DirectoryMutation{mutation}, DesiredState: mutation.DesiredState})
			} else {
				_, err = kernel.ApplyDirectory(t.Context(), mutation)
			}
			if err == nil {
				t.Fatal("invalid standalone publication succeeded")
			}
			if _, err := os.Lstat(mutation.ActivePath); !os.IsNotExist(err) {
				t.Fatalf("invalid publication touched root: %v", err)
			}
			after, err := store.Load()
			if err != nil || !reflect.DeepEqual(before, after) {
				t.Fatalf("invalid publication changed state: %v", err)
			}
			if journals, err := kernel.Directory.ListOpen(); err != nil || len(journals) != 0 {
				t.Fatalf("invalid publication left journals: %+v, %v", journals, err)
			}
		})
	}
}

func TestPublicationParentCannotBeDiscardedByOrdinaryGroup(t *testing.T) {
	for _, misplaced := range []bool{false, true} {
		t.Run(fmt.Sprint(misplaced), func(t *testing.T) {
			kernel, mutation, store := transactionFixture(t, "ordinary-group-parent")
			before, err := store.Load()
			if err != nil {
				t.Fatal(err)
			}
			if misplaced {
				parent, err := profileauthority.Capture(t.Context(), mutation.OwnedBase)
				if err != nil {
					t.Fatal(err)
				}
				mutation.PublicationParentAuthority = &parent
			}
			_, err = kernel.ApplyDirectoryGroup(t.Context(), DirectoryGroup{OperationGroupID: "TEST-parent-group", Mutations: []DirectoryMutation{mutation}, DesiredState: mutation.DesiredState})
			if !misplaced {
				if err != nil {
					t.Fatal(err)
				}
				assertTransactionBody(t, mutation.ActivePath, "new")
				return
			}
			if err == nil || !strings.Contains(err.Error(), "standalone publication cannot join a package group") {
				t.Fatalf("ordinary group discarded original parent: %v", err)
			}
			after, err := store.Load()
			if err != nil || !reflect.DeepEqual(before, after) {
				t.Fatalf("misplaced parent changed state: %v", err)
			}
			assertTransactionBody(t, mutation.ActivePath, "old")
			assertTransactionBody(t, mutation.StagingPath, "new")
			if journals, err := kernel.Directory.ListOpen(); err != nil || len(journals) != 0 {
				t.Fatalf("misplaced parent left journals: %+v, %v", journals, err)
			}
		})
	}
}

const standaloneCrashRoot = "UAP_TEST_STANDALONE_CRASH_ROOT"
const standaloneCrashPhase = "UAP_TEST_STANDALONE_CRASH_PHASE"

func runStandaloneCrash(root, phase string) {
	store := statev2.Store{Path: filepath.Join(root, "state-v2.json")}
	state, err := store.Load()
	if err != nil || len(state.Installations) != 1 {
		fmt.Fprintf(os.Stderr, "load standalone fixture: %v", err)
		os.Exit(76)
	}
	installation := state.Installations[0]
	binding := onlyClient(installation)
	base := filepath.Join(root, "managed")
	kernel := Kernel{StateStore: store, Directory: dirswap.Manager{
		JournalDir: filepath.Join(root, "operations-v2"),
		Fault: func(observed string) error {
			if observed == phase {
				os.Exit(85)
			}
			return nil
		},
	}}
	parent, captureErr := profileauthority.Capture(context.Background(), base)
	if captureErr != nil {
		fmt.Fprintln(os.Stderr, captureErr)
		os.Exit(76)
	}
	_, err = kernel.ApplyDirectory(context.Background(), DirectoryMutation{
		PublicationParentAuthority: &parent,
		Standalone:                 true, RequireAbsent: true, OperationID: "auxiliary-root", Sequence: 1,
		InstallationID: installation.InstallationID, ClientBindingID: binding.ClientBindingID,
		OwnedBase: base, ActivePath: filepath.Join(base, "session"), StagingPath: filepath.Join(base, "plugin.staging"),
		AfterDigest: "auxiliary-content", DesiredState: state, Verify: verifyTransactionBody("new"),
	})
	fmt.Fprintf(os.Stderr, "standalone crash phase not reached: %v", err)
	os.Exit(76)
}

func TestStandalonePublicationProcessCrashRecovery(t *testing.T) {
	if root := os.Getenv(standaloneCrashRoot); root != "" {
		runStandaloneCrash(root, os.Getenv(standaloneCrashPhase))
		return
	}
	for _, phase := range []string{dirswap.PhaseActivationPending, dirswap.PhaseActivated, dirswap.PhaseCommitPending} {
		t.Run(phase, func(t *testing.T) {
			kernel, mutation, store := standaloneFixture(t)
			before, err := store.Load()
			if err != nil {
				t.Fatal(err)
			}
			child := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestStandalonePublicationProcessCrashRecovery$", "-test.count=1")
			child.Env = append(os.Environ(), standaloneCrashRoot+"="+filepath.Dir(store.Path), standaloneCrashPhase+"="+phase)
			output, err := child.CombinedOutput()
			var exit *exec.ExitError
			if !errors.As(err, &exit) || exit.ExitCode() != 85 {
				t.Fatalf("process did not crash at %s: %v %s", phase, err, output)
			}
			if phase == dirswap.PhaseCommitPending {
				committed, err := store.Load()
				if err != nil {
					t.Fatal(err)
				}
				committed.Installations = nil
				if err := store.Save(committed); err != nil {
					t.Fatal(err)
				}
			}
			if err := kernel.Recover(t.Context()); err != nil {
				t.Fatal(err)
			}
			recovered, err := store.Load()
			if err != nil {
				t.Fatal(err)
			}
			if phase == dirswap.PhaseCommitPending {
				assertTransactionBody(t, mutation.ActivePath, "new")
				if len(recovered.Installations) != 0 || len(recovered.TransactionReceipts) != 1 || recovered.TransactionReceipts[0].Phase != ReceiptPhaseCommitted {
					t.Fatalf("standalone commit needs a deleted binding: %+v", recovered)
				}
			} else if _, err := os.Lstat(mutation.ActivePath); !os.IsNotExist(err) || !reflect.DeepEqual(before, recovered) {
				t.Fatalf("uncommitted root survived recovery or changed state: %v", err)
			}
			assertTransactionBody(t, filepath.Join(mutation.OwnedBase, "plugin"), "old")
			if journals, err := kernel.Directory.ListOpen(); err != nil || len(journals) != 0 {
				t.Fatalf("recovery left journals: %+v %v", journals, err)
			}
		})
	}
}
