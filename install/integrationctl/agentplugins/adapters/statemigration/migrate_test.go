package statemigration

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
	"testing"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/statev2"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
)

func TestMigrateCopiesLegacyStateAndPreservesLegacyLoaderBinding(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	legacyPath := filepath.Join(root, "state.json")
	v2Path := filepath.Join(root, "state-v2.json")
	original := `{
  "schema_version": 1,
  "installations": [{
    "integration_id": "context7",
    "requested_source_ref": {"kind":"github_repo_path","value":"context7"},
    "resolved_source_ref": {"kind":"git_commit","value":"https://example.com/catalog@abc"},
    "resolved_version": "1.0.0",
    "source_digest": "sha256:tree",
    "manifest_digest": "sha256:manifest",
    "policy": {"scope":"user"},
    "targets": {
      "codex": {
        "target_id":"codex",
        "state":"activation_pending",
        "activation_state":"native_pending",
        "owned_native_objects":[{"kind":"plugin_root","name":"context7","path":"/test/plugin","protection_class":"user_mutable"}]
      }
    },
    "last_checked_at":"2026-08-01T00:00:00Z",
    "last_updated_at":"2026-08-02T00:00:00Z"
  }]
}`
	if err := os.WriteFile(legacyPath, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	migrator := Migrator{
		LegacyPath: legacyPath,
		V2Store:    statev2.Store{Path: v2Path},
		Now:        func() time.Time { return time.Date(2026, 8, 8, 10, 0, 0, 0, time.UTC) },
		NewID:      func() (string, error) { return "00000000-0000-4000-8000-000000000001", nil },
	}
	report, err := migrator.Migrate()
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if report.Migrated != 1 || report.NeedsRebind != 1 || !report.LegacyStateUntouched {
		t.Fatalf("report = %+v", report)
	}
	backup, err := os.ReadFile(report.BackupPath)
	if err != nil {
		t.Fatal(err)
	}
	legacyAfter, err := os.ReadFile(legacyPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(backup) != original || string(legacyAfter) != original {
		t.Fatal("legacy state or backup changed")
	}
	state, err := migrator.V2Store.Load()
	if err != nil {
		t.Fatal(err)
	}
	installation := state.Installations[0]
	if installation.Package.LoaderKind != domain.LoaderKindLegacy || installation.Package.FormatID != domain.FormatIDLegacyV1 {
		t.Fatalf("legacy binding changed: %+v", installation.Package)
	}
	if installation.OriginMode != domain.OriginModeDirect || installation.Directory != nil || installation.Source.ResolvedRevision != "https://example.com/catalog@abc" ||
		installation.Source.TreeDigest != "sha256:tree" || installation.Package.ManifestDigest != "sha256:manifest" {
		t.Fatalf("short legacy source manufactured Directory provenance: %+v", installation)
	}
	if !installation.NeedsRebind || len(installation.Clients) != 1 {
		t.Fatalf("installation = %+v", installation)
	}
	for _, client := range installation.Clients {
		if client.ClientID != string(domain.ClientCodex) {
			t.Fatalf("legacy codex binding was reclassified: %+v", client)
		}
		if client.Materialization != domain.MaterializationMaterialized || client.Activation != domain.ActivationManual || client.Verification != domain.VerificationNotRun {
			t.Fatalf("client states = %+v", client)
		}
		if client.PackageRevision != nil {
			t.Fatalf("direct legacy binding gained a Directory revision: %+v", client.PackageRevision)
		}
	}
}

func TestMigrateCurrentV2PreservesHeterogeneousDirectoryRevisions(t *testing.T) {
	t.Parallel()
	installationRevision := strings.Repeat("a", 40)
	clientRevision := strings.Repeat("b", 40)
	for _, schema := range []int{2, 3} {
		schema := schema
		t.Run(fmt.Sprintf("schema_%d", schema), func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(t.TempDir(), "state-v2.json")
			original := fmt.Sprintf(`{"schema_version":%d,"installations":[{"installation_id":"00000000-0000-4000-8000-000000000001","declared_name":"demo","source":{"source_binding_id":"src_demo","requested_source":"demo","canonical_source":"https://github.com/777genius/universal-agent-plugins","repository":"777genius/universal-agent-plugins","package_subpath":"plugins/demo","resolved_revision":"%s","tree_digest":"sha256:desired-tree"},"package":{"loader_kind":"agent_plugins","format_id":"agent-plugins/1.0.0","schema_uri":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","declared_name":"demo","manifest_digest":"sha256:desired-manifest","inventory":{}},"clients":{"client_a":{"client_binding_id":"client_a","client_id":"codex","scope":"user","target_locator":"user","physical_artifact_id":"physical_a","materialization":"materialized","activation":"active","authentication":"not_required","policy":"allowed","verification":"installation_verified","package_revision":{"resolved_revision":"%s","tree_digest":"sha256:client-a-tree","manifest_digest":"sha256:client-a-manifest"},"updated_at":"2026-08-01T00:00:00Z"},"client_b":{"client_binding_id":"client_b","client_id":"cursor","scope":"user","target_locator":"user","physical_artifact_id":"physical_b","materialization":"materialized","activation":"active","authentication":"not_required","policy":"allowed","verification":"installation_verified","package_revision":{"resolved_revision":"%s","tree_digest":"sha256:client-b-tree","manifest_digest":"sha256:client-b-manifest"},"updated_at":"2026-08-01T00:00:00Z"}},"created_at":"2026-08-01T00:00:00Z","updated_at":"2026-08-01T00:00:00Z"}]}`, schema, installationRevision, installationRevision, clientRevision)
			if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
				t.Fatal(err)
			}
			migrator := Migrator{V2Store: statev2.Store{Path: path}, Lock: &migrationTestLock{}, PendingCheck: func(context.Context) error { return nil }}
			plan, err := migrator.PlanCurrentV2()
			if err != nil {
				t.Fatal(err)
			}
			if plan.NeedsRebind != 0 {
				t.Fatalf("plan = %+v", plan)
			}
			if _, err := migrator.MigrateCurrentV2(context.Background(), plan.LegacyDigest); err != nil {
				t.Fatal(err)
			}
			state, err := migrator.V2Store.Load()
			if err != nil {
				t.Fatal(err)
			}
			installation := state.Installations[0]
			if installation.OriginMode != domain.OriginModeDirectory || installation.Directory == nil || installation.Source.ResolvedRevision != installationRevision {
				t.Fatalf("installation provenance = %+v", installation)
			}
			clientA := installation.Clients["client_a"].PackageRevision
			clientB := installation.Clients["client_b"].PackageRevision
			if clientA == nil || clientA.ResolvedRevision != installationRevision || clientA.TreeDigest != "sha256:client-a-tree" || clientA.ManifestDigest != "sha256:client-a-manifest" {
				t.Fatalf("client A provenance = %+v", clientA)
			}
			if clientB == nil || clientB.ResolvedRevision != clientRevision || clientB.TreeDigest != "sha256:client-b-tree" || clientB.ManifestDigest != "sha256:client-b-manifest" {
				t.Fatalf("client B provenance = %+v", clientB)
			}
		})
	}
}

func TestMigrateCurrentV2InvalidClientRevisionRetainsDirectProvenance(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "state-v2.json")
	installationRevision := strings.Repeat("a", 40)
	original := fmt.Sprintf(`{"schema_version":2,"installations":[{"installation_id":"00000000-0000-4000-8000-000000000001","declared_name":"demo","source":{"source_binding_id":"src_demo","requested_source":"demo","canonical_source":"https://github.com/777genius/universal-agent-plugins","repository":"777genius/universal-agent-plugins","package_subpath":"plugins/demo","resolved_revision":"%s","tree_digest":"sha256:desired-tree"},"package":{"loader_kind":"agent_plugins","format_id":"agent-plugins/1.0.0","schema_uri":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","declared_name":"demo","manifest_digest":"sha256:desired-manifest","inventory":{}},"clients":{"client_a":{"client_binding_id":"client_a","client_id":"codex","scope":"user","target_locator":"user","physical_artifact_id":"physical_a","materialization":"materialized","activation":"active","authentication":"not_required","policy":"allowed","verification":"installation_verified","package_revision":{"resolved_revision":"abc","tree_digest":"sha256:client-tree","manifest_digest":"sha256:client-manifest"},"updated_at":"2026-08-01T00:00:00Z"}},"created_at":"2026-08-01T00:00:00Z","updated_at":"2026-08-01T00:00:00Z"}]}`, installationRevision)
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	migrator := Migrator{V2Store: statev2.Store{Path: path}, Lock: &migrationTestLock{}, PendingCheck: func(context.Context) error { return nil }}
	plan, err := migrator.PlanCurrentV2()
	if err != nil {
		t.Fatal(err)
	}
	if plan.NeedsRebind != 1 {
		t.Fatalf("plan = %+v", plan)
	}
	if _, err := migrator.MigrateCurrentV2(context.Background(), plan.LegacyDigest); err != nil {
		t.Fatal(err)
	}
	state, err := migrator.V2Store.Load()
	if err != nil {
		t.Fatal(err)
	}
	installation := state.Installations[0]
	client := installation.Clients["client_a"].PackageRevision
	if !installation.NeedsRebind || installation.OriginMode != domain.OriginModeDirect || installation.Directory != nil || installation.Source.ResolvedRevision != installationRevision {
		t.Fatalf("invalid client revision gained Directory provenance: %+v", installation)
	}
	if client == nil || client.ResolvedRevision != "abc" || client.TreeDigest != "sha256:client-tree" || client.ManifestDigest != "sha256:client-manifest" || client.DistributionID != "" || client.ReleaseSequence != 0 {
		t.Fatalf("client provenance was manufactured or overwritten: %+v", client)
	}
}

func TestMigratePinnedLegacySourcePreservesExactDirectoryRevision(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	legacyPath := filepath.Join(root, "state.json")
	revision := strings.Repeat("a", 40)
	body := fmt.Sprintf(`{"schema_version":1,"installations":[{
"integration_id":"context7",
"requested_source_ref":{"kind":"github_repo_path","value":"context7"},
"resolved_source_ref":{"kind":"git_commit","value":"https://example.com/catalog@%s//plugins/context7"},
"resolved_version":"1.0.0","source_digest":"sha256:tree","manifest_digest":"sha256:manifest",
"policy":{"scope":"user"},"targets":{"codex":{"target_id":"codex","state":"active"}}
}]}`, revision)
	if err := os.WriteFile(legacyPath, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	migrator := Migrator{
		LegacyPath: legacyPath,
		V2Store:    statev2.Store{Path: filepath.Join(root, "state-v2.json")},
		NewID:      func() (string, error) { return "00000000-0000-4000-8000-000000000001", nil },
	}
	report, err := migrator.Migrate()
	if err != nil {
		t.Fatal(err)
	}
	if report.NeedsRebind != 0 || !report.LegacyStateUntouched {
		t.Fatalf("report = %+v", report)
	}
	state, err := migrator.V2Store.Load()
	if err != nil {
		t.Fatal(err)
	}
	installation := state.Installations[0]
	if installation.OriginMode != domain.OriginModeDirectory || installation.Directory == nil || installation.Source.ResolvedRevision != revision {
		t.Fatalf("pinned legacy Directory provenance = %+v", installation)
	}
	for _, client := range installation.Clients {
		if client.PackageRevision == nil || client.PackageRevision.ResolvedRevision != revision {
			t.Fatalf("pinned client revision = %+v", client.PackageRevision)
		}
	}
}

func TestExactLegacyGitRevisionAcceptsOnlySupportedImmutableForms(t *testing.T) {
	t.Parallel()
	revision := strings.Repeat("a", 40)
	for _, test := range []struct {
		name   string
		source string
		wantOK bool
	}{
		{name: "raw SHA", source: revision, wantOK: true},
		{name: "last at suffix", source: "ssh://git@example.com/catalog@" + revision, wantOK: true},
		{name: "suffix before subpath", source: "https://example.com/catalog@" + revision + "//plugins/demo", wantOK: true},
		{name: "short SHA", source: "https://example.com/catalog@abc", wantOK: false},
		{name: "uppercase SHA", source: strings.Repeat("A", 40), wantOK: false},
		{name: "raw SHA with subpath", source: revision + "//plugins/demo", wantOK: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, ok := exactLegacyGitRevision(test.source)
			if ok != test.wantOK || (ok && got != revision) {
				t.Fatalf("exactLegacyGitRevision(%q) = %q, %v", test.source, got, ok)
			}
		})
	}
}

func TestMigrateCurrentV2IsDigestCheckedBackedUpAndFixedForward(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := filepath.Join(root, "state-v2.json")
	original := `{"schema_version":2,"installations":[{"installation_id":"00000000-0000-4000-8000-000000000001","declared_name":"demo","source":{"source_binding_id":"src_demo","requested_source":"./demo","canonical_source":"/fixtures/demo","resolved_revision":"local","tree_digest":"sha256:tree"},"package":{"loader_kind":"agent_plugins","format_id":"agent-plugins/1.0.0","schema_uri":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","declared_name":"demo","manifest_digest":"sha256:manifest","inventory":{}},"clients":{},"created_at":"2026-08-01T00:00:00Z","updated_at":"2026-08-01T00:00:00Z"}]}`
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	lock := &migrationTestLock{}
	migrator := Migrator{V2Store: statev2.Store{Path: path}, Lock: lock,
		PendingCheck: func(context.Context) error { return nil },
		Now:          func() time.Time { return time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC) }}
	plan, err := migrator.PlanCurrentV2()
	if err != nil {
		t.Fatal(err)
	}
	if plan.SourceSchema != 2 || plan.Installations != 1 {
		t.Fatalf("plan = %+v", plan)
	}
	if _, err := migrator.MigrateCurrentV2(context.Background(), "sha256:stale"); err == nil {
		t.Fatal("stale reviewed digest was accepted")
	}
	report, err := migrator.MigrateCurrentV2(context.Background(), plan.LegacyDigest)
	if err != nil {
		t.Fatal(err)
	}
	backup, err := os.ReadFile(report.BackupPath)
	if err != nil || string(backup) != original {
		t.Fatalf("backup mismatch: %v", err)
	}
	state, err := migrator.V2Store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if state.Installations[0].OriginMode != domain.OriginModeDirect || state.Installations[0].Directory != nil {
		t.Fatalf("direct provenance changed: %+v", state.Installations[0])
	}
	if lock.acquired < 2 {
		t.Fatalf("migration lock acquisitions = %d", lock.acquired)
	}
	if _, err := migrator.MigrateCurrentV2(context.Background(), plan.LegacyDigest); err == nil {
		t.Fatal("current schema was rewritten instead of fixed-forward refusal")
	}
}

func TestMigrateCurrentV2DirectoryShapedSourceWithoutExactRevisionNeedsRebind(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "state-v2.json")
	original := `{"schema_version":2,"installations":[{"installation_id":"00000000-0000-4000-8000-000000000001","declared_name":"demo","source":{"source_binding_id":"src_demo","requested_source":"demo","canonical_source":"https://github.com/777genius/universal-agent-plugins@abc//demo","repository":"777genius/universal-agent-plugins","package_subpath":"demo","resolved_revision":"abc","tree_digest":"sha256:tree"},"package":{"loader_kind":"agent_plugins","format_id":"agent-plugins/1.0.0","schema_uri":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","declared_name":"demo","manifest_digest":"sha256:manifest","inventory":{}},"clients":{},"created_at":"2026-08-01T00:00:00Z","updated_at":"2026-08-01T00:00:00Z"}]}`
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	migrator := Migrator{V2Store: statev2.Store{Path: path}, Lock: &migrationTestLock{},
		PendingCheck: func(context.Context) error { return nil }}
	plan, err := migrator.PlanCurrentV2()
	if err != nil {
		t.Fatal(err)
	}
	if plan.NeedsRebind != 1 {
		t.Fatalf("plan = %+v", plan)
	}
	report, err := migrator.MigrateCurrentV2(context.Background(), plan.LegacyDigest)
	if err != nil {
		t.Fatal(err)
	}
	state, err := migrator.V2Store.Load()
	if err != nil {
		t.Fatal(err)
	}
	installation := state.Installations[0]
	if !installation.NeedsRebind || installation.OriginMode != domain.OriginModeDirect || installation.Directory != nil || installation.Source.ResolvedRevision != "abc" {
		t.Fatalf("unproven Directory source was promoted: %+v", installation)
	}
	if report.NeedsRebind != 1 {
		t.Fatalf("report = %+v", report)
	}
}

func TestMigrateCurrentV2ReadsPendingStateWithoutRecoveringIt(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "state-v2.json")
	original := `{"schema_version":2,"installations":[]}`
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	migrator := Migrator{V2Store: statev2.Store{Path: path}, Lock: &migrationTestLock{}}
	migrator.PendingCheck = func(context.Context) error { return fmt.Errorf("degraded operation") }
	if _, err := migrator.PlanCurrentV2(); err == nil || !strings.Contains(err.Error(), "degraded operation") {
		t.Fatalf("unresolved recovery was accepted: %v", err)
	}
	if _, err := migrator.MigrateCurrentV2(context.Background(), "sha256:unused"); err == nil || !strings.Contains(err.Error(), "degraded operation") {
		t.Fatalf("unresolved recovery was accepted: %v", err)
	}
	body, err := os.ReadFile(path)
	if err != nil || string(body) != original {
		t.Fatalf("refused migration changed state: %v %q", err, body)
	}
}

func TestMigrateCurrentV2PendingCheckIsReadOnlyAcrossOldSchemas(t *testing.T) {
	for _, schema := range []int{2, 3, 4} {
		for _, phase := range []string{"state_committed", "rolled_back"} {
			t.Run(fmt.Sprintf("schema-%d-%s", schema, phase), func(t *testing.T) {
				root := t.TempDir()
				path := filepath.Join(root, "state-v2.json")
				journal := filepath.Join(root, "operation.json")
				original := []byte(fmt.Sprintf(`{"schema_version":%d,"installations":[],"transaction_receipts":[{"operation_id":"pending-op","sequence":1,"mutation_type":"directory_swap","client_binding_id":"client_pending","phase":%q}]}`, schema, phase))
				if err := os.WriteFile(path, original, 0600); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(journal, []byte(phase), 0600); err != nil {
					t.Fatal(err)
				}
				migrator := Migrator{V2Store: statev2.Store{Path: path}, Lock: &migrationTestLock{}, PendingCheck: func(context.Context) error { return nil }}
				if _, err := migrator.PlanCurrentV2(); err == nil {
					t.Fatal("pending state planned migration")
				}
				if _, err := migrator.MigrateCurrentV2(context.Background(), digest(original)); err == nil {
					t.Fatal("pending state migrated")
				}
				after, err := os.ReadFile(path)
				if err != nil || !bytes.Equal(original, after) {
					t.Fatalf("refusal changed state: %v", err)
				}
				j, err := os.ReadFile(journal)
				if err != nil || string(j) != phase {
					t.Fatal("refusal changed journal")
				}
				entries, _ := os.ReadDir(root)
				if len(entries) != 2 {
					t.Fatal("refusal created backup or durable files")
				}
				// Input fixture representing completion by the corresponding old writer:
				// schema stays old; the new migrator never performs recovery itself.
				recovered := []byte(fmt.Sprintf(`{"schema_version":%d,"installations":[]}`, schema))
				if err := os.WriteFile(path, recovered, 0600); err != nil {
					t.Fatal(err)
				}
				if err := os.Remove(journal); err != nil {
					t.Fatal(err)
				}
				plan, err := migrator.PlanCurrentV2()
				if err != nil {
					t.Fatal(err)
				}
				report, err := migrator.MigrateCurrentV2(context.Background(), plan.LegacyDigest)
				if err != nil {
					t.Fatal(err)
				}
				backup, err := os.ReadFile(report.BackupPath)
				if err != nil || !bytes.Equal(backup, recovered) {
					t.Fatal("backup did not contain this apply's exact old bytes")
				}
				state, err := migrator.V2Store.Load()
				if err != nil || state.SchemaVersion != domain.StateSchemaVersion || len(state.EnvironmentReceipts) != 0 {
					t.Fatalf("migration roundtrip: %+v %v", state, err)
				}
			})
		}
	}
}

type migrationTestLock struct{ acquired int }

func (lock *migrationTestLock) Acquire(context.Context) (ports.UnlockFunc, error) {
	lock.acquired++
	return func() error { return nil }, nil
}

type migrationHookLock struct{ acquired func() }

func (lock migrationHookLock) Acquire(context.Context) (ports.UnlockFunc, error) {
	lock.acquired()
	return func() error { return nil }, nil
}

func TestMigrateCurrentV2RechecksDigestAndPendingOperationsUnderLock(t *testing.T) {
	for _, change := range []string{"digest", "pending"} {
		t.Run(change, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "state-v2.json")
			original := []byte(`{"schema_version":4,"installations":[]}`)
			if err := os.WriteFile(path, original, 0o600); err != nil {
				t.Fatal(err)
			}
			pending := false
			migrator := Migrator{V2Store: statev2.Store{Path: path},
				Lock: migrationHookLock{acquired: func() {
					if change == "pending" {
						pending = true
					} else if err := os.WriteFile(path, append(original, '\n'), 0o600); err != nil {
						t.Fatal(err)
					}
				}},
				PendingCheck: func(context.Context) error {
					if pending {
						return fmt.Errorf("pending operation appeared while acquiring mutation lock")
					}
					return nil
				},
			}
			plan, err := migrator.PlanCurrentV2()
			if err != nil {
				t.Fatal(err)
			}
			_, err = migrator.MigrateCurrentV2(t.Context(), plan.LegacyDigest)
			if change == "digest" && !errors.Is(err, ErrPlanChanged) {
				t.Fatalf("digest changed under lock but migration continued: %v", err)
			}
			if change == "pending" && (err == nil || !strings.Contains(err.Error(), "pending operation appeared")) {
				t.Fatalf("pending operation appeared under lock but migration continued: %v", err)
			}
			backups, _ := filepath.Glob(path + ".schema*.backup-agentplugins-*")
			if len(backups) != 0 {
				t.Fatalf("refused migration created backup: %v", backups)
			}
			version, err := migrator.V2Store.SchemaVersion()
			if err != nil || version != 4 {
				t.Fatalf("refused migration rewrote schema: %d %v", version, err)
			}
		})
	}
}

func TestMigrateCurrentV2BackupFailureLeavesStateUntouched(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state-v2.json")
	original := []byte(`{"schema_version":3,"installations":[]}`)
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 27, 0, 0, 0, 0, time.UTC)
	backupPath := fmt.Sprintf("%s.schema3.backup-agentplugins-%s-%s", path, now.Format("20060102T150405.000000000Z"), strings.TrimPrefix(digest(original), "sha256:")[:12])
	if err := os.Mkdir(backupPath, 0o700); err != nil {
		t.Fatal(err)
	}
	migrator := Migrator{V2Store: statev2.Store{Path: path}, Lock: &migrationTestLock{}, Now: func() time.Time { return now }, PendingCheck: func(context.Context) error { return nil }}
	if _, err := migrator.MigrateCurrentV2(t.Context(), digest(original)); err == nil {
		t.Fatal("unavailable backup did not refuse migration")
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(after, original) {
		t.Fatalf("backup failure changed state: %v %q", err, after)
	}
}

func TestMigrateCurrentV2SaveFailureRetainsExactBackupAndOriginal(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "state-v2.json")
	target := filepath.Join(root, "original.json")
	original := []byte(`{"schema_version":4,"installations":[]}`)
	if err := os.WriteFile(target, original, 0o600); err != nil {
		t.Fatal(err)
	}
	// Loading follows the existing file; atomic publication refuses to replace
	// a symlink. This causes a deterministic save failure after a real backup.
	if err := os.Symlink(target, path); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	migrator := Migrator{V2Store: statev2.Store{Path: path}, Lock: &migrationTestLock{}, PendingCheck: func(context.Context) error { return nil }}
	report, err := migrator.MigrateCurrentV2(t.Context(), digest(original))
	if err == nil || !strings.Contains(err.Error(), "commit migrated state") || report.BackupPath == "" {
		t.Fatalf("expected commit failure with recoverable backup: %+v %v", report, err)
	}
	backup, err := os.ReadFile(report.BackupPath)
	if err != nil || !bytes.Equal(backup, original) {
		t.Fatalf("save failure lost backup: %v %q", err, backup)
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(after, original) {
		t.Fatalf("save failure changed original: %v %q", err, after)
	}
}

func TestMigrateMarksAmbiguousSourceForRebind(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	legacyPath := filepath.Join(root, "state.json")
	if err := os.WriteFile(legacyPath, []byte(`{"schema_version":1,"installations":[{"integration_id":"demo","requested_source_ref":{"value":"demo"},"resolved_source_ref":{},"policy":{"scope":"user"},"targets":{}}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	migrator := Migrator{
		LegacyPath: legacyPath,
		V2Store:    statev2.Store{Path: filepath.Join(root, "state-v2.json")},
		NewID:      func() (string, error) { return "00000000-0000-4000-8000-000000000001", nil },
	}
	report, err := migrator.Migrate()
	if err != nil {
		t.Fatal(err)
	}
	if report.NeedsRebind != 1 {
		t.Fatalf("report = %+v", report)
	}
	state, err := migrator.V2Store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !state.Installations[0].NeedsRebind {
		t.Fatal("ambiguous installation was not marked needs_rebind")
	}
}

func TestMigrateSeparatesDuplicateLegacySourceBindings(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	legacyPath := filepath.Join(root, "state.json")
	body := `{"schema_version":1,"installations":[
{"integration_id":"first","requested_source_ref":{"value":"same"},"resolved_source_ref":{"value":"https://example.com/same@abc"},"source_digest":"sha256:one","policy":{"scope":"user"},"targets":{}},
{"integration_id":"second","requested_source_ref":{"value":"same"},"resolved_source_ref":{"value":"https://example.com/same@abc"},"source_digest":"sha256:two","policy":{"scope":"user"},"targets":{}}
]}`
	if err := os.WriteFile(legacyPath, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	next := 0
	migrator := Migrator{
		LegacyPath: legacyPath, V2Store: statev2.Store{Path: filepath.Join(root, "state-v2.json")},
		NewID: func() (string, error) {
			next++
			return fmt.Sprintf("00000000-0000-4000-8000-%012d", next), nil
		},
	}
	plan, err := migrator.Plan()
	if err != nil || plan.NeedsRebind != 2 {
		t.Fatalf("plan = %+v, err=%v", plan, err)
	}
	report, err := migrator.MigrateExpected(plan.LegacyDigest)
	if err != nil || report.NeedsRebind != 2 {
		t.Fatalf("report = %+v, err=%v", report, err)
	}
	state, err := migrator.V2Store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if state.Installations[0].Source.SourceBindingID == state.Installations[1].Source.SourceBindingID || !state.Installations[0].NeedsRebind || !state.Installations[1].NeedsRebind {
		t.Fatalf("installations = %+v", state.Installations)
	}
}

func TestMigrateRefusesToOverwriteExistingV2State(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	legacyPath := filepath.Join(root, "state.json")
	v2Path := filepath.Join(root, "state-v2.json")
	if err := os.WriteFile(legacyPath, []byte(`{"schema_version":1,"installations":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(v2Path, []byte(`{"schema_version":2,"installations":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := (Migrator{LegacyPath: legacyPath, V2Store: statev2.Store{Path: v2Path}}).Migrate()
	if err == nil {
		t.Fatal("existing v2 state was overwritten")
	}
}

func TestMigrateExpectedRejectsLegacyStateChangedAfterPlan(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	legacyPath := filepath.Join(root, "state.json")
	if err := os.WriteFile(legacyPath, []byte(`{"schema_version":1,"installations":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	migrator := Migrator{LegacyPath: legacyPath, V2Store: statev2.Store{Path: filepath.Join(root, "state-v2.json")}}
	plan, err := migrator.Plan()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacyPath, []byte(`{"schema_version":1,"installations":[]} `), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := migrator.MigrateExpected(plan.LegacyDigest); err == nil {
		t.Fatal("changed legacy state was migrated without a new review")
	}
	if _, err := os.Stat(migrator.V2Store.Path); !os.IsNotExist(err) {
		t.Fatalf("state v2 was created: %v", err)
	}
}

// Schema 4 already records explicit Directory provenance; a schema-only upgrade
// must not infer the old frozen catalog's community distribution and release 1.
func TestMigrateSchema4PreservesDirectoryProvenance(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state-v2.json")
	original := []byte(`{"schema_version":4,"installations":[{"installation_id":"00000000-0000-4000-8000-000000000001","declared_name":"demo","origin_mode":"directory","directory":{"product_id":"demo","distribution_id":"publisher/demo","distribution_kind":"community","desired_release_sequence":7},"source":{"source_binding_id":"src_demo","requested_source":"demo","canonical_source":"https://github.com/publisher/demo","repository":"publisher/demo","resolved_revision":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","tree_digest":"sha256:tree"},"package":{"loader_kind":"agent_plugins","format_id":"agent-plugins/1.0.0","schema_uri":"https://agent-plugins.org/schemas/1.0.0/plugin.schema.json","declared_name":"demo","manifest_digest":"sha256:manifest","inventory":{}},"clients":{"client_a":{"client_binding_id":"client_a","client_id":"codex","scope":"user","target_locator":"user","physical_artifact_id":"physical_a","materialization":"materialized","activation":"active","authentication":"not_required","policy":"allowed","verification":"installation_verified","package_revision":{"distribution_id":"publisher/demo","release_sequence":6,"resolved_revision":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","tree_digest":"sha256:client-tree","manifest_digest":"sha256:client-manifest"},"updated_at":"2026-08-01T00:00:00Z"}},"created_at":"2026-08-01T00:00:00Z","updated_at":"2026-08-01T00:00:00Z"}]}`)
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}
	var expected domain.StateFileV2
	if err := json.Unmarshal(original, &expected); err != nil {
		t.Fatal(err)
	}
	migrator := Migrator{V2Store: statev2.Store{Path: path}, Lock: &migrationTestLock{}, PendingCheck: func(context.Context) error { return nil }}
	plan, err := migrator.PlanCurrentV2()
	if err != nil {
		t.Fatal(err)
	}
	if plan.SourceSchema != 4 || plan.Installations != 1 || plan.NeedsRebind != 0 {
		t.Fatalf("plan: %+v", plan)
	}
	afterPlan, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(afterPlan, original) {
		t.Fatalf("plan rewrote state: %v", err)
	}
	report, err := migrator.MigrateCurrentV2(t.Context(), plan.LegacyDigest)
	if err != nil {
		t.Fatal(err)
	}
	if report.Migrated != 1 || report.NeedsRebind != plan.NeedsRebind {
		t.Fatalf("report: %+v", report)
	}
	backup, err := os.ReadFile(report.BackupPath)
	if err != nil || !bytes.Equal(backup, original) {
		t.Fatalf("backup: %v", err)
	}
	state, err := migrator.V2Store.Load()
	if err != nil {
		t.Fatal(err)
	}
	expected.SchemaVersion = domain.StateSchemaVersion
	if !reflect.DeepEqual(state, expected) {
		t.Fatalf("schema-only upgrade changed provenance:\ngot %+v\nwant %+v", state, expected)
	}
}
