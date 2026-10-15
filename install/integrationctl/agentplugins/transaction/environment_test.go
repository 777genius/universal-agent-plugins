package transaction

import (
	"errors"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestEnvironmentRecoveryRefusesMixedForeignScope(t *testing.T) {
	owned := &domain.EnvironmentAuthority{EnvironmentID: "user-default", Alias: "review", ClientID: "codex", ExpectedAction: "repair"}
	foreign := &domain.EnvironmentAuthority{EnvironmentID: "another-environment", Alias: "other", ClientID: "codex", ExpectedAction: "repair"}
	state := domain.StateFileV2{SchemaVersion: domain.StateSchemaVersion, TransactionReceipts: []domain.MutationReceipt{
		{OperationID: "one", Sequence: 1, MutationType: "directory_swap", ClientBindingID: "binding", Phase: "state_committed", EnvironmentAuthority: owned},
		{OperationID: "two", Sequence: 2, MutationType: "directory_swap", ClientBindingID: "binding", Phase: "state_committed", EnvironmentAuthority: foreign},
	}}
	if err := ValidateEnvironmentRecoveryScope(state, owned); !errors.Is(err, ErrEnvironmentAuthority) {
		t.Fatalf("mixed recovery scope = %v", err)
	}
	state.TransactionReceipts = state.TransactionReceipts[:1]
	if err := ValidateEnvironmentRecoveryScope(state, owned); err != nil {
		t.Fatalf("owned recovery scope was refused: %v", err)
	}
	if err := ValidateEnvironmentRecoveryScope(state, nil); !errors.Is(err, ErrEnvironmentAuthority) {
		t.Fatalf("nil-owner recovery scope = %v", err)
	}
}

func TestEnvironmentReceiptSurvivesPartialAndDropsOnlyAtTerminalRemoval(t *testing.T) {
	authority := &domain.EnvironmentAuthority{EnvironmentID: "user-default", Alias: "review", ClientID: "codex",
		InstallationID: "install", ClientBindingID: "binding", ExpectedAction: "remove", SourceKey: "github:acme/tools:"}
	state := domain.StateFileV2{SchemaVersion: domain.StateSchemaVersion}
	if err := UpsertEnvironmentReceipt(&state, authority, "install", "binding", authority.SourceKey, "sha256:aa", nil, "committed"); err != nil {
		t.Fatal(err)
	}
	if len(state.EnvironmentReceipts) != 1 {
		t.Fatalf("receipts = %+v", state.EnvironmentReceipts)
	}
	DropEnvironmentReceipt(&state, authority)
	if len(state.EnvironmentReceipts) != 0 {
		t.Fatalf("terminal removal retained association: %+v", state.EnvironmentReceipts)
	}
}

func TestEnvironmentAuthorityPreservesUnmanagedImperativeOperations(t *testing.T) {
	state := domain.StateFileV2{SchemaVersion: domain.StateSchemaVersion, EnvironmentReceipts: []domain.EnvironmentReceipt{{EnvironmentID: "user-default", Alias: "other", ClientID: "codex", InstallationID: "owned", ClientBindingID: "owned-binding"}}}
	for _, action := range []string{"install", "update", "remove", "repair"} {
		if err := AuthorizeEnvironmentMutation(state, nil, action, "unmanaged", "unmanaged-binding", "", false); err != nil {
			t.Fatalf("unmanaged %s: %v", action, err)
		}
		if err := AuthorizeEnvironmentMutation(state, nil, action, "owned", "owned-binding", "", false); !errors.Is(err, ErrEnvironmentAuthority) {
			t.Fatalf("owned %s escaped: %v", action, err)
		}
	}
}

func TestRecoveryAuthorityCoversMultipleOwnedAliasesAndTargets(t *testing.T) {
	authority := &domain.EnvironmentAuthority{EnvironmentID: "user-default"}
	state := domain.StateFileV2{SchemaVersion: domain.StateSchemaVersion}
	for _, alias := range []string{"one", "two"} {
		for _, client := range []string{"claude", "codex"} {
			state.TransactionReceipts = append(state.TransactionReceipts, domain.MutationReceipt{Phase: "state_committed", EnvironmentAuthority: &domain.EnvironmentAuthority{EnvironmentID: authority.EnvironmentID, Alias: alias, ClientID: client}})
		}
	}
	if err := ValidateEnvironmentRecoveryScope(state, authority); err != nil {
		t.Fatal(err)
	}
	state.TransactionReceipts[3].EnvironmentAuthority = nil
	if err := ValidateEnvironmentRecoveryScope(state, authority); !errors.Is(err, ErrEnvironmentAuthority) {
		t.Fatalf("indeterminate member accepted: %v", err)
	}
}
