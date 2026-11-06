package transaction

import (
	"context"
	"path/filepath"
	"testing"
)

func TestDirectoryRemovalGroupPreflightsAllTargetsBeforePhysicalEffects(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*DirectoryRemoval, DirectoryRemoval)
	}{
		{"invalid sequence", func(r *DirectoryRemoval, _ DirectoryRemoval) { r.Sequence = 0 }},
		{"duplicate operation", func(r *DirectoryRemoval, first DirectoryRemoval) { r.OperationID = first.OperationID }},
		{"duplicate path", func(r *DirectoryRemoval, first DirectoryRemoval) { r.ActivePath = first.ActivePath }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			kernel, mutation, store := transactionFixture(t, "removal-preflight")
			verified := 0
			first := DirectoryRemoval{
				OperationID: mutation.OperationID, InstallationID: mutation.InstallationID,
				ClientBindingID: mutation.ClientBindingID, Sequence: 1,
				OwnedBase: mutation.OwnedBase, ActivePath: mutation.ActivePath,
				Verify: func(context.Context, string) error { verified++; return nil },
			}
			second := first
			second.OperationID = "removal-second"
			second.Sequence = 2
			second.ActivePath = filepath.Join(mutation.OwnedBase, "removal-second-target")
			second.Verify = nil
			tc.mutate(&second, first)
			physicalEffects := 0
			kernel.Directory.Fault = func(string) error { physicalEffects++; return nil }
			_, err := kernel.RemoveDirectoryGroup(context.Background(), DirectoryRemovalGroup{
				OperationGroupID: "removal-preflight-group",
				DesiredState:     mutation.DesiredState,
				Removals:         []DirectoryRemoval{first, second},
			})
			if err == nil {
				t.Fatal("invalid removal member was accepted")
			}
			if verified != 1 {
				t.Fatalf("expected preflight to verify first target, got %d checks", verified)
			}
			if physicalEffects != 0 {
				t.Fatalf("rejected group had %d physical effects", physicalEffects)
			}
			assertTransactionBody(t, mutation.ActivePath, "old")
			state, loadErr := store.Load()
			if loadErr != nil {
				t.Fatal(loadErr)
			}
			if len(onlyClient(state.Installations[0]).Receipts) != 0 {
				t.Fatal("rejected group recorded receipts")
			}
		})
	}
}
