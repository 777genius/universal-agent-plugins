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
	crashProcessRootEnv = "UAP_TEST_TRANSACTION_CRASH_ROOT_20261102"
	crashProcessExitCode = 83
)

// This is a separate OS process, not a callback returning an error. Exit after
// the new active directory and phase=activated journal have been durably written,
// but before the installer can save its ownership receipt.
func runPostWriteCrashChild(root string) {
	store := statev2.Store{Path: filepath.Join(root, "state-v2.json")}
	state, err := store.Load()
	if err != nil || len(state.Installations) != 1 {
		fmt.Fprintf(os.Stderr, "load disposable crash fixture: %v\n", err)
		os.Exit(76)
	}
	installation := state.Installations[0]
	binding := onlyClient(installation)
	mutation := DirectoryMutation{
		OperationID: "crash-after-activated",
		InstallationID: installation.InstallationID,
		ClientBindingID: binding.ClientBindingID,
		Sequence: 1,
		OwnedBase: filepath.Join(root, "managed"),
		ActivePath: filepath.Join(root, "managed", "plugin"),
		StagingPath: filepath.Join(root, "managed", "plugin.staging"),
		VerifyBefore: verifyTransactionBody("old"),
		Verify: verifyTransactionBody("new"),
		Activation: domain.ActivationPrepared,
		Authentication: domain.AuthenticationNotRequired,
		Policy: domain.PolicyAllowed,
		Verification: domain.VerificationInstalled,
		DesiredState: state,
	}
	kernel := Kernel{StateStore: store, Directory: dirswap.Manager{
		JournalDir: filepath.Join(root, "operations-v2"),
		Fault: func(phase string) error {
			if phase == dirswap.PhaseActivated {
				os.Exit(crashProcessExitCode)
			}
			return nil
		},
	}}
	_, err = kernel.ApplyDirectory(context.Background(), mutation)
	fmt.Fprintf(os.Stderr, "post-write crash point was not reached: %v\n", err)
	os.Exit(76)
}

// A process killed *after* real activation cannot claim the old state as a
// successful installation. On a new process/kernel, the durable journal must
// restore exact old content, retain foreign data and permit a clean retry.
// This qualifies the filesystem transaction kernel, not native-client startup.
func TestDirectoryProcessCrashAfterActivationRecovers(t *testing.T) {
	if root := os.Getenv(crashProcessRootEnv); root != "" {
		runPostWriteCrashChild(root)
		return
	}
	kernel, mutation, store := transactionFixture(t, "crash-after-activated")
	root := filepath.Dir(store.Path)
	foreign := filepath.Join(mutation.OwnedBase, "user-data.txt")
	if err := os.WriteFile(foreign, []byte("unmanaged contents\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	beforeState, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	assertTransactionBody(t, mutation.ActivePath, "old")

	command := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestDirectoryProcessCrashAfterActivationRecovers$", "-test.count=1")
	command.Env = append(os.Environ(), crashProcessRootEnv+"="+root)
	output, err := command.CombinedOutput()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != crashProcessExitCode {
		t.Fatalf("child did not crash at the post-write phase, exit=%v output=%q", err, output)
	}
	assertTransactionBody(t, mutation.ActivePath, "new")
	open, err := kernel.Directory.ListOpen()
	if err != nil || len(open) != 1 || open[0].Phase != dirswap.PhaseActivated ||
		open[0].OperationID != mutation.OperationID {
		t.Fatalf("post-write journal not durable: %+v %v", open, err)
	}
	afterCrash, err := store.Load()
	if err != nil || !reflect.DeepEqual(beforeState, afterCrash) {
		t.Fatalf("child crash wrote a false committed state: %v", err)
	}
	if got, err := os.ReadFile(foreign); err != nil || string(got) != "unmanaged contents\n" {
		t.Fatalf("unmanaged data changed during crash: %q %v", got, err)
	}

	// Reconstruct the kernel from disk; no in-memory object from the crashed
	// process is available to authorize ownership or decide commit-vs-rollback.
	restarted := Kernel{
		StateStore: statev2.Store{Path: store.Path},
		Directory: dirswap.Manager{JournalDir: filepath.Join(root, "operations-v2")},
	}
	if err := restarted.Recover(t.Context()); err != nil {
		t.Fatalf("recover on a new kernel: %v", err)
	}
	assertTransactionBody(t, mutation.ActivePath, "old")
	recovered, err := store.Load()
	if err != nil || !reflect.DeepEqual(beforeState, recovered) {
		t.Fatalf("rollback left false receipt or changed state: %v", err)
	}
	if open, err := restarted.Directory.ListOpen(); err != nil || len(open) != 0 {
		t.Fatalf("recovered journal remains open: %+v %v", open, err)
	}
	if got, err := os.ReadFile(foreign); err != nil || string(got) != "unmanaged contents\n" {
		t.Fatalf("recovery damaged unmanaged data: %q %v", got, err)
	}

	// The same source can still be installed by a new, distinct operation.
	mutation.OperationID = "post-crash-clean-retry"
	mutation.DesiredState = recovered
	writeTransactionBody(t, mutation.StagingPath, "new")
	receipt, err := restarted.ApplyDirectory(t.Context(), mutation)
	if err != nil || receipt.Phase != ReceiptPhaseCommitted {
		t.Fatalf("clean retry did not commit after recovery: %+v %v", receipt, err)
	}
	assertTransactionBody(t, mutation.ActivePath, "new")
	committed, err := store.Load()
	if err != nil || len(committed.Installations) != 1 {
		t.Fatalf("clean retry did not persist a valid installation: %v", err)
	}
	owned := onlyClient(committed.Installations[0])
	if len(owned.Receipts) != 1 || owned.Receipts[0].Phase != ReceiptPhaseCommitted ||
		owned.Receipts[0].OperationID != mutation.OperationID {
		t.Fatalf("clean retry returned success without a durable committed receipt: %+v", owned.Receipts)
	}
	if open, err := restarted.Directory.ListOpen(); err != nil || len(open) != 0 {
		t.Fatalf("clean retry left an uncommitted journal: %+v %v", open, err)
	}
	if got, err := os.ReadFile(foreign); err != nil || string(got) != "unmanaged contents\n" {
		t.Fatalf("clean retry damaged unmanaged data: %q %v", got, err)
	}
}
