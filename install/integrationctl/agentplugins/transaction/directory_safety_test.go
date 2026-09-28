package transaction

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/dirswap"
)

func TestDirectoryVerificationFailurePreservesForeignBytesAndOriginalError(t *testing.T) {
	kernel, mutation, store := transactionFixture(t, "foreign-rollback")
	original := errors.New("original publication verification failed")
	mutation.Verify = func(_ context.Context, path string) error {
		if err := os.WriteFile(filepath.Join(path, "foreign"), []byte("keep"), 0600); err != nil {
			return err
		}
		return original
	}
	_, err := kernel.ApplyDirectory(context.Background(), mutation)
	if !errors.Is(err, original) || !strings.Contains(err.Error(), "rollback failed") {
		t.Fatalf("lost original/rollback failure: %v", err)
	}
	r, loadErr := kernel.Directory.Load(mutation.OperationID)
	if loadErr != nil {
		t.Fatal(loadErr)
	}
	for _, path := range []string{r.ActivePath, r.BackupPath, r.QuarantinePath, filepath.Join(kernel.Directory.JournalDir, mutation.OperationID+".json")} {
		if !strings.Contains(err.Error(), path) {
			t.Fatalf("recovery path missing: %s in %v", path, err)
		}
	}
	assertTransactionBody(t, r.ActivePath, "new")
	assertTransactionBody(t, r.BackupPath, "old")
	body, readErr := os.ReadFile(filepath.Join(r.ActivePath, "foreign"))
	if readErr != nil || string(body) != "keep" {
		t.Fatalf("foreign data lost: %q %v", body, readErr)
	}
	state, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(onlyClient(state.Installations[0]).Receipts) != 0 {
		t.Fatal("failed verification committed state")
	}
}

func TestDirectoryRejectsBackupDriftBeforeStateCommit(t *testing.T) {
	for _, group := range []bool{false, true} {
		t.Run(map[bool]string{false: "single", true: "group"}[group], func(t *testing.T) {
			kernel, mutation, store := transactionFixture(t, "backup-drift")
			mutate := func() error {
				r, err := kernel.Directory.Load(mutation.OperationID)
				if err != nil {
					return err
				}
				return os.WriteFile(filepath.Join(r.BackupPath, "foreign"), []byte("keep backup"), 0600)
			}
			var err error
			if group {
				_, err = kernel.ApplyDirectoryGroup(context.Background(), DirectoryGroup{OperationGroupID: "backup-drift-group", Mutations: []DirectoryMutation{mutation}, DesiredState: mutation.DesiredState, PostApplyVerify: func(context.Context) error { return mutate() }})
			} else {
				mutation.Verify = func(context.Context, string) error { return mutate() }
				_, err = kernel.ApplyDirectory(context.Background(), mutation)
			}
			if err == nil {
				t.Fatal("backup drift was committed")
			}
			r, loadErr := kernel.Directory.Load(mutation.OperationID)
			if loadErr != nil {
				t.Fatal(loadErr)
			}
			assertTransactionBody(t, r.BackupPath, "old")
			body, readErr := os.ReadFile(filepath.Join(r.BackupPath, "foreign"))
			if readErr != nil || string(body) != "keep backup" {
				t.Fatalf("foreign backup lost: %q %v", body, readErr)
			}
			state, err := store.Load()
			if err != nil {
				t.Fatal(err)
			}
			if len(onlyClient(state.Installations[0]).Receipts) != 0 {
				t.Fatal("changed backup committed to state")
			}
		})
	}
}

func TestRemovalApplyFailureReportsRollbackFailureAndPreservesBackup(t *testing.T) {
	kernel, mutation, store := transactionFixture(t, "remove-backup-drift")
	original := errors.New("original removal failed")
	kernel.Directory.Fault = func(at string) error {
		if at != dirswap.FaultBackupRenamed {
			return nil
		}
		r, err := kernel.Directory.Load(mutation.OperationID)
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(r.BackupPath, "foreign"), []byte("keep backup"), 0600); err != nil {
			return err
		}
		return original
	}
	_, err := kernel.RemoveDirectory(context.Background(), DirectoryRemoval{
		OperationID: mutation.OperationID, InstallationID: mutation.InstallationID, ClientBindingID: mutation.ClientBindingID,
		Sequence: mutation.Sequence, OwnedBase: mutation.OwnedBase, ActivePath: mutation.ActivePath, Verify: verifyTransactionBody("old"),
	})
	if !errors.Is(err, original) || !strings.Contains(err.Error(), "rollback failed") {
		t.Fatalf("missing original failure: %v", err)
	}
	r, loadErr := kernel.Directory.Load(mutation.OperationID)
	if loadErr != nil {
		t.Fatal(loadErr)
	}
	if !strings.Contains(err.Error(), r.BackupPath) {
		t.Fatal("backup path missing")
	}
	assertTransactionBody(t, r.BackupPath, "old")
	body, readErr := os.ReadFile(filepath.Join(r.BackupPath, "foreign"))
	if readErr != nil || string(body) != "keep backup" {
		t.Fatalf("foreign bytes lost: %q %v", body, readErr)
	}
	state, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(onlyClient(state.Installations[0]).Receipts) != 0 {
		t.Fatal("failed removal committed state")
	}
}
