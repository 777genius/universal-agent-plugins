package dirswap

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestApplyAndCommitAtomicallyReplacesDirectory(t *testing.T) {
	t.Parallel()
	manager, input := fixture(t)
	receipt, err := manager.Apply(context.Background(), input)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	assertBody(t, input.ActivePath, "new")
	if err := manager.Commit(context.Background(), receipt); err != nil {
		t.Fatalf("commit: %v", err)
	}
	if _, err := os.Stat(receipt.BackupPath); !os.IsNotExist(err) {
		t.Fatalf("backup survived commit: %v", err)
	}
	if _, err := os.Stat(manager.journalPath(input.OperationID)); !os.IsNotExist(err) {
		t.Fatalf("journal survived commit: %v", err)
	}
}

func TestApplyRequireAbsentRejectsWithoutTouchingUnexpectedExistingContent(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	base := filepath.Join(root, "managed")
	active := filepath.Join(base, "plugin")
	staging := filepath.Join(base, "plugin.staging")
	writeBody(t, active, "foreign")
	writeBody(t, staging, "new")
	manager := Manager{JournalDir: filepath.Join(root, "journal")}
	input := Input{OperationID: "operation-1", ClientBindingID: "binding-1", Sequence: 1,
		OwnedBase: base, ActivePath: active, StagingPath: staging, RequireAbsent: true}
	if _, err := manager.Apply(context.Background(), input); err == nil {
		t.Fatal("RequireAbsent accepted an already-existing active path")
	}
	assertBody(t, active, "foreign")
	if _, err := os.Stat(staging); err != nil {
		t.Fatalf("staged directory was consumed despite the rejected apply: %v", err)
	}
	if _, err := os.Stat(manager.journalPath(input.OperationID)); !os.IsNotExist(err) {
		t.Fatalf("rejected RequireAbsent apply left a journal entry: %v", err)
	}
}

func TestApplyRequireAbsentAcceptsGenuinelyAbsentActivePath(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	base := filepath.Join(root, "managed")
	active := filepath.Join(base, "plugin")
	staging := filepath.Join(base, "plugin.staging")
	writeBody(t, staging, "new")
	manager := Manager{JournalDir: filepath.Join(root, "journal")}
	input := Input{OperationID: "operation-1", ClientBindingID: "binding-1", Sequence: 1,
		OwnedBase: base, ActivePath: active, StagingPath: staging, RequireAbsent: true}
	receipt, err := manager.Apply(context.Background(), input)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	assertBody(t, active, "new")
	if err := manager.Commit(context.Background(), receipt); err != nil {
		t.Fatalf("commit: %v", err)
	}
}

func TestRecoverRollsBackAfterOldDirectoryWasBackedUp(t *testing.T) {
	t.Parallel()
	manager, input := fixture(t)
	manager.Fault = func(phase string) error {
		if phase == PhaseOldBackedUp {
			return errors.New("crash after backup")
		}
		return nil
	}
	if _, err := manager.Apply(context.Background(), input); err == nil {
		t.Fatal("fault was not injected")
	}
	manager.Fault = nil
	if err := manager.Recover(context.Background(), input.OperationID, false); err != nil {
		t.Fatalf("recover: %v", err)
	}
	assertBody(t, input.ActivePath, "old")
	assertBody(t, input.StagingPath, "new")
}

func TestRecoverRollsBackCrashBetweenBackupRenameAndPhaseSave(t *testing.T) {
	t.Parallel()
	manager, input := fixture(t)
	manager.Fault = func(point string) error {
		if point == FaultBackupRenamed {
			return errors.New("crash after backup rename")
		}
		return nil
	}
	if _, err := manager.Apply(context.Background(), input); err == nil {
		t.Fatal("fault was not injected")
	}
	manager.Fault = nil
	if err := manager.Recover(context.Background(), input.OperationID, false); err != nil {
		t.Fatalf("recover: %v", err)
	}
	assertBody(t, input.ActivePath, "old")
}

func TestRecoverRollsBackCrashBetweenActivationRenameAndPhaseSave(t *testing.T) {
	t.Parallel()
	manager, input := fixture(t)
	manager.Fault = func(point string) error {
		if point == FaultActivationApplied {
			return errors.New("crash after activation rename")
		}
		return nil
	}
	if _, err := manager.Apply(context.Background(), input); err == nil {
		t.Fatal("fault was not injected")
	}
	manager.Fault = nil
	if err := manager.Recover(context.Background(), input.OperationID, false); err != nil {
		t.Fatalf("recover: %v", err)
	}
	assertBody(t, input.ActivePath, "old")
}

func TestRecoverResumesRollbackAfterActivatedDirectoryRemoval(t *testing.T) {
	t.Parallel()
	manager, input := fixture(t)
	receipt, err := manager.Apply(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	manager.Fault = func(point string) error {
		if point == FaultRollbackRemoved {
			return errors.New("crash during rollback")
		}
		return nil
	}
	if err := manager.Rollback(context.Background(), receipt); err == nil {
		t.Fatal("fault was not injected")
	}
	manager.Fault = nil
	if err := manager.Recover(context.Background(), input.OperationID, false); err != nil {
		t.Fatalf("recover: %v", err)
	}
	assertBody(t, input.ActivePath, "old")
}

func TestRecoverResumesCommitAfterBackupRemoval(t *testing.T) {
	t.Parallel()
	manager, input := fixture(t)
	receipt, err := manager.Apply(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	manager.Fault = func(point string) error {
		if point == FaultBackupRemoved {
			return errors.New("crash during commit")
		}
		return nil
	}
	if err := manager.Commit(context.Background(), receipt); err == nil {
		t.Fatal("fault was not injected")
	}
	manager.Fault = nil
	if err := manager.Recover(context.Background(), input.OperationID, true); err != nil {
		t.Fatalf("recover: %v", err)
	}
	assertBody(t, input.ActivePath, "new")
}

func TestRecoverRollsBackOrCommitsAfterActivation(t *testing.T) {
	t.Parallel()
	for _, stateCommitted := range []bool{false, true} {
		stateCommitted := stateCommitted
		t.Run(map[bool]string{false: "rollback", true: "commit"}[stateCommitted], func(t *testing.T) {
			manager, input := fixture(t)
			manager.Fault = func(phase string) error {
				if phase == PhaseActivated {
					return errors.New("crash after activation")
				}
				return nil
			}
			if _, err := manager.Apply(context.Background(), input); err == nil {
				t.Fatal("fault was not injected")
			}
			manager.Fault = nil
			if err := manager.Recover(context.Background(), input.OperationID, stateCommitted); err != nil {
				t.Fatalf("recover: %v", err)
			}
			want := "old"
			if stateCommitted {
				want = "new"
			}
			assertBody(t, input.ActivePath, want)
		})
	}
}

func TestApplyRejectsPathsOutsideOwnedBase(t *testing.T) {
	t.Parallel()
	manager, input := fixture(t)
	outside := filepath.Join(t.TempDir(), "outside")
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	input.ActivePath = outside
	if _, err := manager.Apply(context.Background(), input); err == nil {
		t.Fatal("outside active path accepted")
	}
}

func TestCrossParentStagingAtomicallyCommits(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	owned := filepath.Join(root, "skills")
	active := filepath.Join(owned, "plugin")
	staging := filepath.Join(root, ".agentplugins-staging-0123456789abcdef")
	writeBody(t, active, "old")
	writeBody(t, staging, "new")
	manager := Manager{JournalDir: filepath.Join(root, "journal")}
	receipt, err := manager.Apply(context.Background(), Input{
		OperationID: "cross-parent", ClientBindingID: "binding-1", Sequence: 1,
		OwnedBase: owned, ActivePath: active, StagingPath: staging,
	})
	if err != nil {
		t.Fatal(err)
	}
	assertBody(t, active, "new")
	if err := manager.Commit(context.Background(), receipt); err != nil {
		t.Fatal(err)
	}
}

func TestCrossParentStagingRecoversRollbackAfterActivationCrash(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	owned := filepath.Join(root, "skills")
	active := filepath.Join(owned, "plugin")
	staging := filepath.Join(root, ".agentplugins-staging-0123456789abcdef")
	writeBody(t, active, "old")
	writeBody(t, staging, "new")
	manager := Manager{JournalDir: filepath.Join(root, "journal"), Fault: func(point string) error {
		if point == FaultActivationApplied {
			return errors.New("crash after cross-parent activation")
		}
		return nil
	}}
	if _, err := manager.Apply(context.Background(), Input{
		OperationID: "cross-parent-crash", ClientBindingID: "binding-1", Sequence: 1,
		OwnedBase: owned, ActivePath: active, StagingPath: staging,
	}); err == nil {
		t.Fatal("fault was not injected")
	}
	manager.Fault = nil
	if err := manager.Recover(context.Background(), "cross-parent-crash", false); err != nil {
		t.Fatal(err)
	}
	assertBody(t, active, "old")
}

func TestCrossParentStagingRejectsUnreservedOrForeignSibling(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	owned := filepath.Join(root, "skills")
	active := filepath.Join(owned, "plugin")
	writeBody(t, active, "old")
	manager := Manager{JournalDir: filepath.Join(root, "journal")}
	for name, staging := range map[string]string{
		"unreserved": filepath.Join(root, "staging"),
		"foreign":    filepath.Join(t.TempDir(), ".agentplugins-staging-0123456789abcdef"),
	} {
		t.Run(name, func(t *testing.T) {
			writeBody(t, staging, "new")
			if _, err := manager.Apply(context.Background(), Input{
				OperationID: "reject-" + name, ClientBindingID: "binding-1", Sequence: 1,
				OwnedBase: owned, ActivePath: active, StagingPath: staging,
			}); err == nil {
				t.Fatal("unsafe cross-parent staging path was accepted")
			}
		})
	}
}

func TestRemoveCommitsOrRollsBackAtomically(t *testing.T) {
	t.Parallel()
	for _, commit := range []bool{false, true} {
		commit := commit
		t.Run(map[bool]string{false: "rollback", true: "commit"}[commit], func(t *testing.T) {
			manager, input := fixture(t)
			input.Remove = true
			input.StagingPath = ""
			receipt, err := manager.Apply(context.Background(), input)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := os.Lstat(input.ActivePath); !os.IsNotExist(err) {
				t.Fatalf("active path survived remove apply: %v", err)
			}
			if commit {
				err = manager.Commit(context.Background(), receipt)
			} else {
				err = manager.Rollback(context.Background(), receipt)
			}
			if err != nil {
				t.Fatal(err)
			}
			if commit {
				if _, err := os.Lstat(input.ActivePath); !os.IsNotExist(err) {
					t.Fatalf("active path survived committed remove: %v", err)
				}
			} else {
				assertBody(t, input.ActivePath, "old")
			}
		})
	}
}

func TestRecoverRemoveUsesStateCommitDecision(t *testing.T) {
	t.Parallel()
	for _, stateCommitted := range []bool{false, true} {
		stateCommitted := stateCommitted
		t.Run(map[bool]string{false: "rollback", true: "commit"}[stateCommitted], func(t *testing.T) {
			manager, input := fixture(t)
			input.Remove = true
			input.StagingPath = ""
			manager.Fault = func(phase string) error {
				if phase == PhaseActivated {
					return errors.New("simulated crash")
				}
				return nil
			}
			if _, err := manager.Apply(context.Background(), input); err == nil {
				t.Fatal("simulated crash did not occur")
			}
			manager.Fault = nil
			if err := manager.Recover(context.Background(), input.OperationID, stateCommitted); err != nil {
				t.Fatal(err)
			}
			_, statErr := os.Lstat(input.ActivePath)
			if stateCommitted && !os.IsNotExist(statErr) {
				t.Fatalf("committed removal restored active path: %v", statErr)
			}
			if !stateCommitted {
				assertBody(t, input.ActivePath, "old")
			}
		})
	}
}

func TestListOpenFailsClosedOnCorruptJournal(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	manager := Manager{JournalDir: filepath.Join(root, "journal")}
	if err := os.MkdirAll(manager.JournalDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(manager.JournalDir, "broken.json"), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.ListOpen(); err == nil {
		t.Fatal("corrupt directory swap journal was skipped")
	}
}

func fixture(t *testing.T) (Manager, Input) {
	t.Helper()
	root := t.TempDir()
	base := filepath.Join(root, "managed")
	active := filepath.Join(base, "plugin")
	staging := filepath.Join(base, "plugin.staging")
	writeBody(t, active, "old")
	writeBody(t, staging, "new")
	return Manager{JournalDir: filepath.Join(root, "journal")}, Input{
		OperationID: "operation-1", ClientBindingID: "binding-1", Sequence: 1,
		OwnedBase: base, ActivePath: active, StagingPath: staging,
	}
}

func writeBody(t *testing.T, root, body string) {
	t.Helper()
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "body"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func assertBody(t *testing.T, root, want string) {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(root, "body"))
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != want {
		t.Fatalf("body = %q, want %q", body, want)
	}
}
