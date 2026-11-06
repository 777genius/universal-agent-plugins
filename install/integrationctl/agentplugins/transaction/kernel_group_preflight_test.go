package transaction

import (
	"context"
	"path/filepath"
	"testing"
)

// A later invalid group member must be refused before the first member can
// acquire a physical journal or touch the active directory.
func TestDirectoryGroupPreflightsEveryItemBeforePhysicalEffects(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*DirectoryMutation, DirectoryMutation)
	}{
		{"invalid sequence", func(m *DirectoryMutation, _ DirectoryMutation) { m.Sequence = 0 }},
		{"duplicate operation", func(m *DirectoryMutation, first DirectoryMutation) { m.OperationID = first.OperationID }},
		{"duplicate physical path", func(m *DirectoryMutation, first DirectoryMutation) { m.ActivePath = first.ActivePath }},
		{"unknown client binding", func(m *DirectoryMutation, _ DirectoryMutation) { m.ClientBindingID = "not-a-managed-binding" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			kernel, first, store := transactionFixture(t, "preflight")
			second := first
			second.OperationID = "second-operation"
			second.Sequence = 2
			second.ActivePath = filepath.Join(first.OwnedBase, "second-plugin")
			second.StagingPath = filepath.Join(first.OwnedBase, "second-plugin.staging")
			tc.mutate(&second, first)
			physicalEffects := 0
			kernel.Directory.Fault = func(string) error { physicalEffects++; return nil }
			_, err := kernel.ApplyDirectoryGroup(context.Background(), DirectoryGroup{
				OperationGroupID: "preflight-group",
				DesiredState:     first.DesiredState,
				Mutations:        []DirectoryMutation{first, second},
			})
			if err == nil {
				t.Fatal("invalid member was accepted")
			}
			if physicalEffects != 0 {
				t.Fatalf("preflight allowed %d physical effects before refusal", physicalEffects)
			}
			assertTransactionBody(t, first.ActivePath, "old")
			state, loadErr := store.Load()
			if loadErr != nil {
				t.Fatal(loadErr)
			}
			if len(onlyClient(state.Installations[0]).Receipts) != 0 {
				t.Fatal("preflight refusal persisted transaction receipts")
			}
		})
	}
}
