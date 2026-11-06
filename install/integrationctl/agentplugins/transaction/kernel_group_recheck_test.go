package transaction

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/dirswap"
)

// A preflight can pass and a later recheck can fail after a prior physical
// mutation. The successful first mutation must be rolled back, not orphaned.
func TestDirectoryGroupRecheckFailureRollsBackAppliedMutation(t *testing.T) {
	kernel, first, store := transactionFixture(t, "recheck-apply-one")
	second := first
	second.OperationID = "recheck-apply-two"
	second.Sequence = 2
	second.ActivePath = filepath.Join(first.OwnedBase, "second")
	second.StagingPath = filepath.Join(first.OwnedBase, "second.staging")
	group := DirectoryGroup{
		OperationGroupID: "recheck-apply-group",
		DesiredState:     first.DesiredState,
		Mutations:        []DirectoryMutation{first, second},
	}
	verifyFirst := group.Mutations[0].Verify
	group.Mutations[0].Verify = func(ctx context.Context, active string) error {
		if err := verifyFirst(ctx, active); err != nil {
			return err
		}
		// This callback happens only after Directory.Apply has activated
		// the first mutation, but before the second per-item recheck.
		group.Mutations[1].Sequence = 0
		return nil
	}
	receipts, err := kernel.ApplyDirectoryGroup(context.Background(), group)
	if err == nil || len(receipts) != 0 {
		t.Fatalf("post-effect recheck = receipts=%+v err=%v", receipts, err)
	}
	if phase := FailurePhase(err); phase != GroupFailureRolledBack {
		t.Fatalf("failure phase = %q, want rolled back: %v", phase, err)
	}
	if !strings.Contains(err.Error(), "mutation 1 has incomplete identity") {
		t.Fatalf("lost recheck error: %v", err)
	}
	assertTransactionBody(t, first.ActivePath, "old")
	open, listErr := kernel.Directory.ListOpen()
	if listErr != nil || len(open) != 0 {
		t.Fatalf("rollback left open directory journals: %+v, %v", open, listErr)
	}
	state, loadErr := store.Load()
	if loadErr != nil {
		t.Fatal(loadErr)
	}
	if len(onlyClient(state.Installations[0]).Receipts) != 0 {
		t.Fatal("failed group persisted receipts")
	}
}

func TestDirectoryRemovalGroupRecheckFailureRollsBackAppliedRemoval(t *testing.T) {
	kernel, mutation, store := transactionFixture(t, "recheck-remove-one")
	first := DirectoryRemoval{
		OperationID: mutation.OperationID, InstallationID: mutation.InstallationID,
		ClientBindingID: mutation.ClientBindingID, Sequence: 1,
		OwnedBase: mutation.OwnedBase, ActivePath: mutation.ActivePath,
		Verify: verifyTransactionBody("old"),
	}
	second := first
	second.OperationID = "recheck-remove-two"
	second.Sequence = 2
	second.ActivePath = filepath.Join(mutation.OwnedBase, "second-remove")
	second.Verify = nil
	group := DirectoryRemovalGroup{
		OperationGroupID: "recheck-remove-group",
		DesiredState:     mutation.DesiredState,
		Removals:         []DirectoryRemoval{first, second},
	}
	verifyFirst := group.Removals[0].Verify
	verifyCalls := 0
	group.Removals[0].Verify = func(ctx context.Context, active string) error {
		verifyCalls++
		if err := verifyFirst(ctx, active); err != nil {
			return err
		}
		if verifyCalls == 2 {
			// The first call is preflight; dirswap invokes VerifyActive
			// during its first physical removal, after group preflight.
			group.Removals[1].Sequence = 0
		}
		return nil
	}
	receipts, err := kernel.RemoveDirectoryGroup(context.Background(), group)
	if err == nil || len(receipts) != 0 {
		t.Fatalf("post-effect removal recheck = receipts=%+v err=%v", receipts, err)
	}
	if verifyCalls < 2 || !strings.Contains(err.Error(), "removal 1 has incomplete identity") {
		t.Fatalf("recheck not exercised, verify calls=%d err=%v", verifyCalls, err)
	}
	if phase := FailurePhase(err); phase != GroupFailureRolledBack {
		t.Fatalf("failure phase = %q, want rolled back: %v", phase, err)
	}
	assertTransactionBody(t, mutation.ActivePath, "old")
	open, listErr := kernel.Directory.ListOpen()
	if listErr != nil || len(open) != 0 {
		t.Fatalf("rollback left open removal journals: %+v, %v", open, listErr)
	}
	state, loadErr := store.Load()
	if loadErr != nil {
		t.Fatal(loadErr)
	}
	if len(onlyClient(state.Installations[0]).Receipts) != 0 {
		t.Fatal("failed removal persisted receipts")
	}
}

// A failed rollback after a post-effect recheck is unknown, not a proven
// rollback. The journal must survive so a later recovery can restore old bytes.
func TestDirectoryGroupRecheckRollbackFailureRemainsRecoverable(t *testing.T) {
	kernel, first, store := transactionFixture(t, "recheck-unknown-one")
	second := first
	second.OperationID = "recheck-unknown-two"
	second.Sequence = 2
	second.ActivePath = filepath.Join(first.OwnedBase, "second")
	second.StagingPath = filepath.Join(first.OwnedBase, "second.staging")
	group := DirectoryGroup{
		OperationGroupID: "recheck-unknown-group",
		DesiredState:     first.DesiredState,
		Mutations:        []DirectoryMutation{first, second},
	}
	verifyFirst := group.Mutations[0].Verify
	group.Mutations[0].Verify = func(ctx context.Context, active string) error {
		if err := verifyFirst(ctx, active); err != nil {
			return err
		}
		group.Mutations[1].Sequence = 0
		return nil
	}
	interrupted := errors.New("rollback interrupted")
	kernel.Directory.Fault = func(phase string) error {
		if phase == dirswap.PhaseRollbackPending {
			return interrupted
		}
		return nil
	}
	receipts, err := kernel.ApplyDirectoryGroup(context.Background(), group)
	if len(receipts) != 0 || !errors.Is(err, interrupted) || FailurePhase(err) != GroupFailureUnknown {
		t.Fatalf("interrupted rollback = receipts=%+v phase=%q err=%v", receipts, FailurePhase(err), err)
	}
	assertTransactionBody(t, first.ActivePath, "new")
	open, err := kernel.Directory.ListOpen()
	if err != nil || len(open) != 1 || open[0].Phase != dirswap.PhaseRollbackPending {
		t.Fatalf("interrupted rollback lost recovery journal: %+v, %v", open, err)
	}
	state, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(onlyClient(state.Installations[0]).Receipts) != 0 {
		t.Fatal("ambiguous rollback persisted a receipt")
	}

	// Model a restart without reusing the faulty in-process directory adapter.
	restarted := kernel
	restarted.Directory.Fault = nil
	if err := restarted.Recover(context.Background()); err != nil {
		t.Fatalf("restart recovery: %v", err)
	}
	assertTransactionBody(t, first.ActivePath, "old")
	open, err = restarted.Directory.ListOpen()
	if err != nil || len(open) != 0 {
		t.Fatalf("restart left directory journals: %+v, %v", open, err)
	}
}
