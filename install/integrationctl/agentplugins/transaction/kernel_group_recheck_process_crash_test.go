package transaction

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/dirswap"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/statev2"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

const (
	groupRecheckCrashRootEnv = "UAP_TEST_GROUP_RECHECK_CRASH_ROOT_20261106"
	groupRecheckCrashModeEnv = "UAP_TEST_GROUP_RECHECK_CRASH_MODE_20261106"
	groupRecheckCrashExitCode = 85
)

// The child reconstructs its kernel and operation inputs from durable state,
// then dies during rollback of the first physical group effect.
func runGroupRecheckCrashChild(root, mode string) {
	store := statev2.Store{Path: filepath.Join(root, "state-v2.json")}
	state, err := store.Load()
	if err != nil || len(state.Installations) != 1 {
		fmt.Fprintf(os.Stderr, "load group crash fixture: %v\n", err)
		os.Exit(76)
	}
	installation := state.Installations[0]
	binding := onlyClient(installation)
	firstID := "process-recheck-" + mode + "-first"
	manager := dirswap.Manager{
		JournalDir: filepath.Join(root, "operations-v2"),
		Fault: func(phase string) error {
			if phase == dirswap.PhaseRollbackPending {
				os.Exit(groupRecheckCrashExitCode)
			}
			return nil
		},
	}
	kernel := Kernel{StateStore: store, Directory: manager}
	base := filepath.Join(root, "managed")
	active := filepath.Join(base, "plugin")
	secondActive := filepath.Join(base, "second")
	switch mode {
	case "apply":
		first := DirectoryMutation{
			OperationID: firstID, InstallationID: installation.InstallationID,
			ClientBindingID: binding.ClientBindingID, Sequence: 1,
			OwnedBase: base, ActivePath: active, StagingPath: filepath.Join(base, "plugin.staging"),
			VerifyBefore: verifyTransactionBody("old"), DesiredState: state,
			Activation: domain.ActivationPrepared, Authentication: domain.AuthenticationNotRequired,
			Policy: domain.PolicyAllowed, Verification: domain.VerificationInstalled,
		}
		second := first
		second.OperationID = "process-recheck-apply-second"
		second.Sequence = 2
		second.ActivePath = secondActive
		second.StagingPath = filepath.Join(base, "second.staging")
		group := DirectoryGroup{
			OperationGroupID: "process-recheck-apply-group",
			DesiredState: state, Mutations: []DirectoryMutation{first, second},
		}
		group.Mutations[0].Verify = func(ctx context.Context, path string) error {
			if err := verifyTransactionBody("new")(ctx, path); err != nil {
				return err
			}
			group.Mutations[1].Sequence = 0
			return nil
		}
		_, err = kernel.ApplyDirectoryGroup(context.Background(), group)
	case "remove":
		first := DirectoryRemoval{
			OperationID: firstID, InstallationID: installation.InstallationID,
			ClientBindingID: binding.ClientBindingID, Sequence: 1,
			OwnedBase: base, ActivePath: active,
		}
		second := first
		second.OperationID = "process-recheck-remove-second"
		second.Sequence = 2
		second.ActivePath = secondActive
		group := DirectoryRemovalGroup{
			OperationGroupID: "process-recheck-remove-group",
			DesiredState: state, Removals: []DirectoryRemoval{first, second},
		}
		verifyCalls := 0
		group.Removals[0].Verify = func(ctx context.Context, path string) error {
			if err := verifyTransactionBody("old")(ctx, path); err != nil {
				return err
			}
			verifyCalls++
			if verifyCalls == 2 {
				group.Removals[1].Sequence = 0
			}
			return nil
		}
		_, err = kernel.RemoveDirectoryGroup(context.Background(), group)
	default:
		fmt.Fprintf(os.Stderr, "unknown group crash mode %q\n", mode)
		os.Exit(76)
	}
	fmt.Fprintf(os.Stderr, "group rollback crash point not reached: %v\n", err)
	os.Exit(76)
}

// A real subprocess exits after group recheck fails but before rollback can
// finish. A new kernel must recover old bytes and leave no false receipt.
func TestGroupRecheckProcessCrashRecovery(t *testing.T) {
	if root := os.Getenv(groupRecheckCrashRootEnv); root != "" {
		runGroupRecheckCrashChild(root, os.Getenv(groupRecheckCrashModeEnv))
		return
	}
	for _, mode := range []string{"apply", "remove"} {
		t.Run(mode, func(t *testing.T) {
			kernel, mutation, store := transactionFixture(t, "process-recheck-"+mode+"-first")
			root := filepath.Dir(store.Path)
			neighbor := filepath.Join(mutation.OwnedBase, "foreign-neighbor.txt")
			if err := os.WriteFile(neighbor, []byte("unmanaged\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			before, err := store.Load()
			if err != nil {
				t.Fatal(err)
			}
			child := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestGroupRecheckProcessCrashRecovery$", "-test.count=1")
			child.Env = append(os.Environ(), groupRecheckCrashRootEnv+"="+root, groupRecheckCrashModeEnv+"="+mode)
			output, err := child.CombinedOutput()
			var exit *exec.ExitError
			if !errors.As(err, &exit) || exit.ExitCode() != groupRecheckCrashExitCode {
				t.Fatalf("child did not crash during %s rollback: exit=%v output=%q", mode, err, output)
			}
			open, err := kernel.Directory.ListOpen()
			if err != nil || len(open) != 1 || open[0].Phase != dirswap.PhaseRollbackPending ||
				open[0].OperationID != mutation.OperationID {
				t.Fatalf("crash lost pending %s journal: %+v, %v", mode, open, err)
			}
			if mode == "apply" {
				assertTransactionBody(t, mutation.ActivePath, "new")
			} else if _, err := os.Lstat(mutation.ActivePath); !os.IsNotExist(err) {
				t.Fatalf("removed directory unexpectedly present before recovery: %v", err)
			}
			afterCrash, err := store.Load()
			if err != nil || !reflect.DeepEqual(before, afterCrash) {
				t.Fatalf("crashed group persisted uncommitted state: %v", err)
			}
			restarted := Kernel{
				StateStore: statev2.Store{Path: store.Path},
				Directory: dirswap.Manager{JournalDir: filepath.Join(root, "operations-v2")},
			}
			if err := restarted.Recover(t.Context()); err != nil {
				t.Fatalf("fresh-kernel group recovery: %v", err)
			}
			assertTransactionBody(t, mutation.ActivePath, "old")
			if journals, err := restarted.Directory.ListOpen(); err != nil || len(journals) != 0 {
				t.Fatalf("recovery left open %s journal: %+v, %v", mode, journals, err)
			}
			recovered, err := store.Load()
			if err != nil || !reflect.DeepEqual(before, recovered) {
				t.Fatalf("recovery changed durable %s state: %v", mode, err)
			}
			if body, err := os.ReadFile(neighbor); err != nil || string(body) != "unmanaged\n" {
				t.Fatalf("recovery modified foreign neighbor: %q, %v", body, err)
			}
		})
	}
}
