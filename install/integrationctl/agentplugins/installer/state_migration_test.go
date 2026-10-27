package installer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/processlock"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/statev2"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func legacyMigrationFixture(schema int) []byte {
	return []byte(fmt.Sprintf(`{
  "schema_version": %d,
  "installations": [
    {"installation_id":"00000000-0000-4000-8000-000000000001","declared_name":"demo","source":{"source_binding_id":"src_demo","requested_source":"./demo","canonical_source":"/fixtures/demo","resolved_revision":"local","tree_digest":"sha256:tree"},"package":{"loader_kind":"agent_plugins","format_id":"agent-plugins/1.0.0","schema_uri":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","declared_name":"demo","manifest_digest":"sha256:manifest","inventory":{}},"clients":{"client_demo":{"client_binding_id":"client_demo","client_id":"codex","scope":"user","target_locator":"user","physical_artifact_id":"physical_demo","materialization":"materialized","activation":"active","authentication":"not_required","policy":"allowed","verification":"installation_verified","package_revision":{"resolved_revision":"local","tree_digest":"sha256:client-tree","manifest_digest":"sha256:client-manifest"},"native_objects":[{"object_id":"native_demo","kind":"plugin_root","logical_name":"demo","path":"/fixtures/installed/demo","managed_digest":"sha256:managed","protection_class":"user_mutable"}],"updated_at":"2026-08-01T00:00:00Z"}},"created_at":"2026-08-01T00:00:00Z","updated_at":"2026-08-01T00:00:00Z"},
    {"installation_id":"00000000-0000-4000-8000-000000000002","declared_name":"other","source":{"source_binding_id":"src_other","requested_source":"./other","canonical_source":"/fixtures/other","resolved_revision":"local","tree_digest":"sha256:other"},"package":{"loader_kind":"agent_plugins","format_id":"agent-plugins/1.0.0","schema_uri":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","declared_name":"other","manifest_digest":"sha256:other-manifest","inventory":{}},"clients":{},"created_at":"2026-08-02T00:00:00Z","updated_at":"2026-08-02T00:00:00Z"}
  ]
}
`, schema))
}

func writeMigrationState(t *testing.T, eng *Engine, body []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(eng.cfg.StateFile), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(eng.cfg.StateFile, body, 0o600); err != nil {
		t.Fatal(err)
	}
}

func assertMigrationStateBytes(t *testing.T, eng *Engine, expected []byte) {
	t.Helper()
	body, err := os.ReadFile(eng.cfg.StateFile)
	if err != nil || !bytes.Equal(body, expected) {
		t.Fatalf("authoritative state changed: %v\n%s", err, body)
	}
}

func TestStateMigrationPreservesRecordsAndExactBackupAcrossOldSchemas(t *testing.T) {
	for _, schema := range []int{2, 3, 4} {
		t.Run(fmt.Sprintf("schema-%d", schema), func(t *testing.T) {
			runner := &countingRunner{}
			callbacks := 0
			eng, err := newTestEngine(t, Config{StateRoot: filepath.Join(t.TempDir(), "state"), Runner: runner,
				OnCommittedBinding: func(context.Context, BindingFacts) error { callbacks++; return nil }})
			if err != nil {
				t.Fatal(err)
			}
			original := legacyMigrationFixture(schema)
			writeMigrationState(t, eng, original)
			before, err := eng.store.Load()
			if err != nil {
				t.Fatal(err)
			}
			plan, err := eng.PrepareStateMigration(t.Context())
			if err != nil || !plan.Required || plan.SourceSchema != schema || plan.TargetSchema != domain.StateSchemaVersion || plan.Installations != 2 {
				t.Fatalf("prepare: %+v %v", plan, err)
			}
			assertMigrationStateBytes(t, eng, original)
			entries, _ := os.ReadDir(eng.cfg.StateRoot)
			if len(entries) != 1 {
				t.Fatalf("preparation created durable files: %v", entries)
			}
			// The host persists review and confirmation across processes.
			serialized, _ := json.Marshal(plan)
			var confirmed StateMigrationPlan
			if err := json.Unmarshal(serialized, &confirmed); err != nil {
				t.Fatal(err)
			}
			next, err := newTestEngine(t, eng.cfg)
			if err != nil {
				t.Fatal(err)
			}
			result, err := next.ApplyStateMigration(t.Context(), confirmed)
			if err != nil || result.Migrated != 2 || result.NoChange || result.BackupPath == "" {
				t.Fatalf("apply: %+v %v", result, err)
			}
			backup, err := os.ReadFile(result.BackupPath)
			if err != nil || !bytes.Equal(backup, original) {
				t.Fatalf("backup is not the reviewed original bytes: %v", err)
			}
			after, err := eng.store.Load()
			if err != nil || !reflect.DeepEqual(before, after) {
				t.Fatalf("record identities, bindings, ownership, or unrelated installation changed:\nbefore: %+v\nafter: %+v\nerror: %v", before, after, err)
			}
			if err := (statev2.Store{Path: eng.cfg.StateFile}).RequireMutationReady(); err != nil {
				t.Fatalf("migrated state still refuses mutation: %v", err)
			}
			current, err := eng.PrepareStateMigration(t.Context())
			if err != nil || current.Required {
				t.Fatalf("current preparation is not idempotent: %+v %v", current, err)
			}
			if result, err := eng.ApplyStateMigration(t.Context(), current); err != nil || !result.NoChange || result.BackupPath != "" {
				t.Fatalf("current apply: %+v %v", result, err)
			}
			backups, _ := filepath.Glob(eng.cfg.StateFile + ".schema*.backup-agentplugins-*")
			if len(backups) != 1 || runner.n != 0 || callbacks != 0 {
				t.Fatalf("unexpected effects: backups=%v commands=%d callbacks=%d", backups, runner.n, callbacks)
			}
		})
	}
}

func TestStateMigrationMissingIsReadOnlyAndBindsObservedAbsence(t *testing.T) {
	eng, err := newTestEngine(t, Config{StateRoot: filepath.Join(t.TempDir(), "missing")})
	if err != nil {
		t.Fatal(err)
	}
	plan, err := eng.PrepareStateMigration(t.Context())
	if err != nil || plan.Required {
		t.Fatalf("missing prepare: %+v %v", plan, err)
	}
	if result, err := eng.ApplyStateMigration(t.Context(), plan); err != nil || !result.NoChange {
		t.Fatalf("missing apply: %+v %v", result, err)
	}
	if _, err := os.Lstat(eng.cfg.StateRoot); !os.IsNotExist(err) {
		t.Fatalf("missing-state no-op created a directory: %v", err)
	}
	original := legacyMigrationFixture(2)
	writeMigrationState(t, eng, original)
	if _, err := eng.ApplyStateMigration(t.Context(), plan); !errors.Is(err, ErrPlanChanged) {
		t.Fatalf("newly appeared old state bypassed review: %v", err)
	}
	assertMigrationStateBytes(t, eng, original)
}

func TestStateMigrationCurrentPlanBindsReviewedBytes(t *testing.T) {
	eng, err := newTestEngine(t, Config{StateRoot: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	writeMigrationState(t, eng, []byte(fmt.Sprintf(`{"schema_version":%d,"installations":[]}`, domain.StateSchemaVersion)))
	plan, err := eng.PrepareStateMigration(t.Context())
	if err != nil || plan.Required {
		t.Fatalf("current prepare: %+v %v", plan, err)
	}
	original := legacyMigrationFixture(4)
	writeMigrationState(t, eng, original)
	if _, err := eng.ApplyStateMigration(t.Context(), plan); !errors.Is(err, ErrPlanChanged) {
		t.Fatalf("old state replaced current state without review: %v", err)
	}
	assertMigrationStateBytes(t, eng, original)
}

func TestStateMigrationRefusesChangedPlanCancellationAndForeignPaths(t *testing.T) {
	eng, err := newTestEngine(t, Config{StateRoot: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	original := legacyMigrationFixture(3)
	writeMigrationState(t, eng, original)
	plan, err := eng.PrepareStateMigration(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := eng.ApplyStateMigration(ctx, plan); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled apply: %v", err)
	}
	foreign := plan
	foreign.StateFile = filepath.Join(t.TempDir(), "state-v2.json")
	if _, err := eng.ApplyStateMigration(t.Context(), foreign); !errors.Is(err, ErrPlanChanged) {
		t.Fatalf("foreign review accepted: %v", err)
	}
	changed := append(append([]byte(nil), original...), '\n')
	writeMigrationState(t, eng, changed)
	if _, err := eng.ApplyStateMigration(t.Context(), plan); !errors.Is(err, ErrPlanChanged) {
		t.Fatalf("stale review accepted: %v", err)
	}
	assertMigrationStateBytes(t, eng, changed)
	entries, _ := os.ReadDir(eng.cfg.StateRoot)
	if len(entries) != 1 {
		t.Fatalf("refusal created a backup or lock: %v", entries)
	}
}

func TestStateMigrationRejectsCorruptAndUnsupportedState(t *testing.T) {
	for _, body := range []string{`{`, `{"schema_version":2,"installations":[],"unknown":true}`, `{"schema_version":5,"installations":[{}]}`, `{"schema_version":99,"installations":[]}`} {
		t.Run(body, func(t *testing.T) {
			eng, err := newTestEngine(t, Config{StateRoot: t.TempDir()})
			if err != nil {
				t.Fatal(err)
			}
			writeMigrationState(t, eng, []byte(body))
			if _, err := eng.PrepareStateMigration(t.Context()); err == nil {
				t.Fatal("invalid state produced an applicable plan")
			}
			assertMigrationStateBytes(t, eng, []byte(body))
		})
	}
}

func TestStateMigrationRefusesOldPendingJournalsButLeavesCurrentRecoveryAlone(t *testing.T) {
	for _, schema := range []int{2, 3, 4, domain.StateSchemaVersion} {
		t.Run(fmt.Sprintf("schema-%d", schema), func(t *testing.T) {
			eng, _ := plantPendingJournal(t)
			original := []byte(fmt.Sprintf(`{"schema_version":%d,"installations":[]}`, schema))
			writeMigrationState(t, eng, original)
			plan, err := eng.PrepareStateMigration(t.Context())
			if schema == domain.StateSchemaVersion {
				if err != nil || plan.Required {
					t.Fatalf("current pending recovery was blocked by migration: %+v %v", plan, err)
				}
			} else if !errors.Is(err, ErrRecoveryRequired) {
				t.Fatalf("old pending journal was accepted: %+v %v", plan, err)
			}
			assertMigrationStateBytes(t, eng, original)
			if _, err := os.Lstat(eng.cfg.LockFile); !os.IsNotExist(err) {
				t.Fatalf("pending check acquired mutation lock: %v", err)
			}
		})
	}
}

func TestStateMigrationRefusesJournalAppearingAfterReview(t *testing.T) {
	eng, err := newTestEngine(t, Config{StateRoot: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	original := legacyMigrationFixture(4)
	writeMigrationState(t, eng, original)
	plan, err := eng.PrepareStateMigration(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	plantOpenJournal(t, eng, "appeared-after-review")
	if _, err := eng.ApplyStateMigration(t.Context(), plan); !errors.Is(err, ErrRecoveryRequired) {
		t.Fatalf("new journal did not block migration: %v", err)
	}
	assertMigrationStateBytes(t, eng, original)
	backups, _ := filepath.Glob(eng.cfg.StateFile + ".schema*.backup-agentplugins-*")
	if len(backups) != 0 {
		t.Fatalf("pending operation produced a backup: %v", backups)
	}
}

func TestStateMigrationConcurrentAppliesCommitOnce(t *testing.T) {
	eng, err := newTestEngine(t, Config{StateRoot: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	writeMigrationState(t, eng, legacyMigrationFixture(4))
	plan, err := eng.PrepareStateMigration(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	errorsCh := make(chan error, 2)
	var workers sync.WaitGroup
	for range 2 {
		workers.Go(func() {
			<-start
			_, err := eng.ApplyStateMigration(t.Context(), plan)
			errorsCh <- err
		})
	}
	close(start)
	workers.Wait()
	close(errorsCh)
	successes := 0
	for err := range errorsCh {
		if err == nil {
			successes++
		} else if !errors.Is(err, ErrPlanChanged) && !errors.Is(err, processlock.ErrActive) {
			t.Fatalf("unexpected concurrent refusal: %v", err)
		}
	}
	backups, _ := filepath.Glob(eng.cfg.StateFile + ".schema*.backup-agentplugins-*")
	if successes != 1 || len(backups) != 1 {
		t.Fatalf("applies: successes=%d backups=%v", successes, backups)
	}
}

func TestStateMigrationClassifiesCorruptAndUnsupportedState(t *testing.T) {
	for _, test := range []struct {
		name, body string
		want       error
		schema     int
	}{
		{name: "invalid-json", body: `{`, want: ErrStateCorrupt},
		{name: "invalid-record", body: `{"schema_version":5,"installations":[{}]}`, want: ErrStateCorrupt},
		{name: "unknown-field", body: `{"schema_version":4,"installations":[],"unknown":true}`, want: ErrStateCorrupt},
		{name: "future", body: `{"schema_version":99,"installations":[]}`, want: ErrUnsupportedStateSchema, schema: 99},
		{name: "unsupported-old", body: `{"schema_version":1,"installations":[]}`, want: ErrUnsupportedStateSchema, schema: 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			eng, err := newTestEngine(t, Config{StateRoot: t.TempDir()})
			if err != nil {
				t.Fatal(err)
			}
			writeMigrationState(t, eng, []byte(test.body))
			_, err = eng.PrepareStateMigration(t.Context())
			if !errors.Is(err, test.want) {
				t.Fatalf("unclassified refusal: %v", err)
			}
			if test.schema != 0 {
				var unsupported *UnsupportedStateSchemaError
				if !errors.As(err, &unsupported) || unsupported.SourceSchema != test.schema || unsupported.TargetSchema != domain.StateSchemaVersion {
					t.Fatalf("missing unsupported schema facts: %v", err)
				}
			}
			assertMigrationStateBytes(t, eng, []byte(test.body))
		})
	}
}

func TestStateMigrationChangedInvalidStateRefusesStaleReview(t *testing.T) {
	for _, replacement := range []string{`{`, `{"schema_version":99,"installations":[]}`, `{"schema_version":5,"installations":[{}]}`, "missing"} {
		t.Run(replacement, func(t *testing.T) {
			eng, err := newTestEngine(t, Config{StateRoot: t.TempDir()})
			if err != nil {
				t.Fatal(err)
			}
			writeMigrationState(t, eng, legacyMigrationFixture(4))
			plan, err := eng.PrepareStateMigration(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			if replacement == "missing" {
				if err := os.Remove(eng.cfg.StateFile); err != nil {
					t.Fatal(err)
				}
			} else {
				writeMigrationState(t, eng, []byte(replacement))
			}
			if _, err := eng.ApplyStateMigration(t.Context(), plan); !errors.Is(err, ErrPlanChanged) {
				t.Fatalf("changed invalid state was diagnosed as the reviewed state: %v", err)
			}
			if _, err := os.Lstat(eng.cfg.LockFile); !os.IsNotExist(err) {
				t.Fatalf("stale review acquired the mutation lock: %v", err)
			}
		})
	}
}

func TestStateMigrationBindsLockAndPendingJournalPaths(t *testing.T) {
	eng, err := newTestEngine(t, Config{StateRoot: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	original := legacyMigrationFixture(4)
	writeMigrationState(t, eng, original)
	plan, err := eng.PrepareStateMigration(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	// A journal that appears in the reviewed namespace must not be bypassed by
	// a parent process accidentally choosing a different operations directory.
	plantOpenJournal(t, eng, "reviewed-namespace")
	for _, path := range []string{"lock", "operations"} {
		t.Run(path, func(t *testing.T) {
			cfg := eng.cfg
			if path == "lock" {
				cfg.LockFile = filepath.Join(cfg.StateRoot, "other.lock")
			} else {
				cfg.OperationsDir = filepath.Join(cfg.StateRoot, "other-operations")
			}
			parent, err := newTestEngine(t, cfg)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := parent.ApplyStateMigration(t.Context(), plan); !errors.Is(err, ErrPlanChanged) {
				t.Fatalf("accepted migration with changed %s path: %v", path, err)
			}
			assertMigrationStateBytes(t, eng, original)
		})
	}
}

func TestStateMigrationClassifiesPersistedPendingWork(t *testing.T) {
	for _, kind := range []string{"transaction-receipt", "binding-receipt", "native-intent"} {
		t.Run(kind, func(t *testing.T) {
			eng, err := newTestEngine(t, Config{StateRoot: t.TempDir()})
			if err != nil {
				t.Fatal(err)
			}
			writeMigrationState(t, eng, legacyMigrationFixture(4))
			state, err := eng.store.Load()
			if err != nil {
				t.Fatal(err)
			}
			binding := state.Installations[0].Clients["client_demo"]
			receipt := domain.MutationReceipt{OperationID: "pending-op", Sequence: 1, MutationType: "directory_swap", ClientBindingID: binding.ClientBindingID, Phase: "state_committed"}
			switch kind {
			case "transaction-receipt":
				state.TransactionReceipts = []domain.MutationReceipt{receipt}
			case "binding-receipt":
				binding.Receipts = []domain.MutationReceipt{receipt}
			case "native-intent":
				root := t.TempDir()
				canonical := "sha256:" + strings.Repeat("a", 64)
				selected, err := physicalCursorSelection(root, filepath.Join(root, "observer"), filepath.Join(root, "binding"), canonical, nil, false)
				if err != nil {
					t.Fatal(err)
				}
				selected, err = selected.WithProjectionDigest("sha256:" + strings.Repeat("b", 64))
				if err != nil {
					t.Fatal(err)
				}
				facts, _ := selected.CursorFacts()
				object := selected.CursorOwnership(facts.PlannedReceipt)
				binding.ClientID, binding.TargetLocator, binding.NativeProfileRoot = "cursor", facts.Selector, root
				binding.PackageRevision.TreeDigest = canonical
				binding.SelectedDelivery, binding.NativeObjects, binding.NativeActivationAttempt = selected, []domain.NativeObjectOwnership{object}, "pending-native-op"
				binding.PendingNativeIntent = &domain.PendingNativeIntent{AttemptID: binding.NativeActivationAttempt, Direction: domain.NativeIntentRegister, Delivery: selected, PreviousCursorObject: object}
			}
			state.Installations[0].Clients[binding.ClientBindingID] = binding
			state.SchemaVersion = 4
			original, err := json.Marshal(state)
			if err != nil {
				t.Fatal(err)
			}
			writeMigrationState(t, eng, original)
			if _, err := eng.PrepareStateMigration(t.Context()); !errors.Is(err, ErrRecoveryRequired) {
				t.Fatalf("persisted %s lost recovery classification: %v", kind, err)
			}
			assertMigrationStateBytes(t, eng, original)
			if _, err := os.Lstat(eng.cfg.LockFile); !os.IsNotExist(err) {
				t.Fatalf("pending work acquired mutation lock: %v", err)
			}
		})
	}
}
