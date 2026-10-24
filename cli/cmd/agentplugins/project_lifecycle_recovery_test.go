package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/dirswap"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

func projectJourneyRecoverInterruptedJournal(t *testing.T, journey projectJourney) {
	t.Helper()
	before := securityFixtureTree(t, journey.nativeRoot(journey.a))
	statePath := filepath.Join(journey.namespace(journey.a), "state-v2.json")
	state := projectJourneyRead(t, statePath)
	staging, active := projectJourneyInterruptDirectoryJournal(t, journey)
	selected, err := journey.app.ForScope(domain.ScopeProject, journey.a)
	if err != nil {
		t.Fatal(err)
	}
	view, err := selected.Installer.Inspect(context.Background())
	if err != nil || !view.Recovery.Required || len(view.Recovery.Journals) != 1 {
		t.Fatalf("interrupted project journal not observed: %+v, %v", view.Recovery, err)
	}
	journey.run(t, journey.a, "deps", "recover", "--yes")
	journey.inspectNoRecovery(t, journey.a)
	projectJourneyAbsent(t, active, filepath.Join(journey.namespace(journey.a), "operations-v2", "project-interrupted-journal.json"))
	// Directory rollback preserves unpublished staging owned by its caller;
	// recovery consumes the durable journal without adopting those bytes.
	if projectJourneyRead(t, filepath.Join(staging, "unpublished.txt")) != "caller-owned unpublished bytes\n" {
		t.Fatal("recovery consumed unpublished caller staging")
	}
	if !reflect.DeepEqual(before, securityFixtureTree(t, journey.nativeRoot(journey.a))) || state != projectJourneyRead(t, statePath) {
		t.Fatal("project journal recovery changed committed native/package ownership")
	}
	journey.run(t, journey.a, "deps", "recover", "--yes")
}

// Reuse the existing directory transaction's fault seam and durable receipt,
// matching the installer pending-journal fixture. No alternate recovery engine
// or product-only injection mechanism is introduced.
func projectJourneyInterruptDirectoryJournal(t *testing.T, journey projectJourney) (string, string) {
	t.Helper()
	const operation = "project-interrupted-journal"
	owned := filepath.Join(journey.namespace(journey.a), "managed")
	active := filepath.Join(owned, "pending-"+operation)
	staging := filepath.Join(owned, ".agentplugins-staging-"+operation)
	if err := os.MkdirAll(staging, 0700); err != nil {
		t.Fatal(err)
	}
	projectJourneyWrite(t, filepath.Join(staging, "unpublished.txt"), "caller-owned unpublished bytes\n")
	operations := filepath.Join(journey.namespace(journey.a), "operations-v2")
	manager := dirswap.Manager{Namespace: journey.namespace(journey.a), JournalDir: operations, Fault: func(phase string) error {
		if phase == dirswap.PhaseBackupPending {
			return errors.New("disposable interrupted project transaction")
		}
		return nil
	}}
	receipt, err := manager.Apply(context.Background(), dirswap.Input{EnvironmentID: environment.EnvironmentProjectDefault, OperationID: operation, ClientBindingID: "project-journal-fixture", Sequence: 1,
		OwnedBase: owned, ActivePath: active, StagingPath: staging, RequireAbsent: true})
	if err == nil || receipt.OperationID != operation {
		t.Fatalf("fixture did not interrupt transaction: %+v, %v", receipt, err)
	}
	receipt.Phase = dirswap.PhaseIntent
	body, err := json.Marshal(receipt)
	if err != nil {
		t.Fatal(err)
	}
	projectJourneyWrite(t, filepath.Join(operations, operation+".json"), string(body))
	return staging, active
}
