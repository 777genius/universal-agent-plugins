package transaction

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/dirswap"
)

// Schema 3 inventory deliberately allows legacy journals with no physical
// proof. An unexpected owner entry must fail closed, never panic in Recover.
func TestRecoverRejectsLegacyJournalWithNilOwnerAuthority(t *testing.T) {
	kernel, mutation, store := transactionFixture(t, "legacy-nil-owner")
	operationID := "legacy-nil-owner"
	sum := sha256.Sum256([]byte(operationID))
	receipt := dirswap.Receipt{
		SchemaVersion: 3, Operation: dirswap.OperationSwap, OperationID: operationID,
		ClientBindingID: mutation.ClientBindingID, Sequence: 1,
		OwnedBase: mutation.OwnedBase, ActivePath: mutation.ActivePath,
		StagingPath: mutation.StagingPath,
		BackupPath:  filepath.Join(mutation.OwnedBase, ".agentplugins-backup-"+hex.EncodeToString(sum[:8])),
		Phase:       dirswap.PhaseIntent,
		ProfileOwners: []dirswap.ProfileOwner{{
			Namespace: "user", InstallationID: mutation.InstallationID,
			ClientID: "codex", ClientBindingID: mutation.ClientBindingID,
		}},
	}
	if err := os.MkdirAll(kernel.Directory.JournalDir, 0o700); err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(receipt)
	if err != nil {
		t.Fatal(err)
	}
	journalPath := filepath.Join(kernel.Directory.JournalDir, operationID+".json")
	if err := os.WriteFile(journalPath, body, 0o600); err != nil {
		t.Fatal(err)
	}
	open, err := kernel.Directory.ListOpen()
	if err != nil || len(open) != 1 {
		t.Fatalf("legacy journal was not admitted to inventory: %+v, %v", open, err)
	}
	err = kernel.Recover(context.Background())
	if err == nil || !strings.Contains(err.Error(), "invalid exact physical profile scope") {
		t.Fatalf("nil legacy owner authority must be refused without panic: %v", err)
	}
	assertTransactionBody(t, mutation.ActivePath, "old")
	if _, err := os.Stat(journalPath); err != nil {
		t.Fatalf("refused legacy journal was removed: %v", err)
	}
	state, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(onlyClient(state.Installations[0]).Receipts) != 0 {
		t.Fatal("refused recovery modified durable state")
	}
}
