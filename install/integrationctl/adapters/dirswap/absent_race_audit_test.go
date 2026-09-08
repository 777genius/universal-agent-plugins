package dirswap

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// Regression: content that appears after the absent precondition must survive
// a rejected publication and the rollback subsequently called by the kernel.
func TestAuditAbsentPublicationRacePreservesForeignDirectory(t *testing.T) {
	root := t.TempDir()
	base := filepath.Join(root, "managed")
	active := filepath.Join(base, "plugin")
	staging := filepath.Join(base, "plugin.staging")
	writeBody(t, staging, "owned-stage")
	manager := Manager{JournalDir: filepath.Join(root, "journal")}
	manager.Fault = func(phase string) error {
		if phase == PhaseActivationPending {
			writeBody(t, active, "foreign-arrived-after-precondition")
		}
		return nil
	}
	input := Input{OperationID: "audit-absent-race", ClientBindingID: "binding-1", Sequence: 1,
		OwnedBase: base, ActivePath: active, StagingPath: staging, RequireAbsent: true}
	receipt, err := manager.Apply(context.Background(), input)
	if err == nil {
		t.Fatal("publication unexpectedly succeeded over nonempty foreign directory")
	}
	assertBody(t, active, "foreign-arrived-after-precondition")
	assertBody(t, staging, "owned-stage")
	manager.Fault = nil
	rollbackErr := manager.Rollback(context.Background(), receipt)
	if _, err := os.Stat(active); err != nil {
		t.Fatalf("rollback deleted foreign directory after failed publication: stat=%v rollback=%v", err, rollbackErr)
	}
	assertBody(t, active, "foreign-arrived-after-precondition")
	if rollbackErr == nil {
		t.Fatal("rollback must retain a recovery error for unowned active content")
	}
	if err := manager.Recover(context.Background(), input.OperationID, false); err == nil {
		t.Fatal("persisted recovery must not accept unowned active content")
	}
	assertBody(t, active, "foreign-arrived-after-precondition")
	assertBody(t, staging, "owned-stage")
}
