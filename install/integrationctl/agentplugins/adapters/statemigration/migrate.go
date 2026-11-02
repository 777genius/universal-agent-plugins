package statemigration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/atomicfile"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/statev2"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
)

// ErrPlanChanged refuses a migration whose reviewed bytes are no longer live.
var ErrPlanChanged = errors.New("state changed after review; rerun migrate-state --dry-run")

// ErrPendingRecovery identifies persisted old-schema receipts and native intents
// that must be recovered with the corresponding previous SDK or binary.
var ErrPendingRecovery = errors.New("old-schema pending recovery requires the corresponding previous SDK/binary; recover with it, then rerun migration dry-run and apply")

type Migrator struct {
	LegacyPath string
	V2Store    statev2.Store
	Now        func() time.Time
	NewID      func() (string, error)
	Lock       ports.MutationLock
	// PendingCheck is strictly read-only. It refuses when any old-schema journal,
	// receipt or native intent needs the matching previous SDK or binary.
	PendingCheck func(context.Context) error
}

type Report struct {
	BackupPath           string
	Migrated             int
	NeedsRebind          int
	LegacyStateUntouched bool
}

type Plan struct {
	LegacyDigest  string `json:"legacy_digest"`
	Installations int    `json:"installations"`
	NeedsRebind   int    `json:"needs_rebind"`
	SourceSchema  int    `json:"source_schema,omitempty"`
}

// PlanCurrentV2 inspects authoritative schemas 2-4 without writing them. The digest
// is the token the apply step must recheck under the global mutation lock.
func (migrator Migrator) PlanCurrentV2() (Plan, error) {
	body, err := os.ReadFile(migrator.V2Store.Path)
	if err != nil {
		return Plan{}, fmt.Errorf("read authoritative state: %w", err)
	}
	var header struct {
		SchemaVersion int `json:"schema_version"`
	}
	if err := json.Unmarshal(body, &header); err != nil {
		return Plan{}, fmt.Errorf("decode authoritative state header: %w", err)
	}
	if header.SchemaVersion < domain.LegacyStateSchemaVersion || header.SchemaVersion > domain.PreviousStateSchemaV4 {
		if header.SchemaVersion == domain.StateSchemaVersion {
			return Plan{}, fmt.Errorf("state already uses schema %d; fixed-forward migration refuses to rewrite it", domain.StateSchemaVersion)
		}
		return Plan{}, fmt.Errorf("unsupported authoritative state schema_version %d", header.SchemaVersion)
	}
	state, err := migrator.V2Store.Load()
	if err != nil {
		return Plan{}, err
	}
	if err := refusePendingOldState(state); err != nil {
		return Plan{}, err
	}
	plan := Plan{LegacyDigest: digest(body), SourceSchema: header.SchemaVersion, Installations: len(state.Installations)}
	if migrator.PendingCheck != nil {
		if err := migrator.PendingCheck(context.Background()); err != nil {
			return Plan{}, fmt.Errorf("old-schema pending recovery blocks migration: %w", err)
		}
	}
	migrateLegacyOrigins(&state, header.SchemaVersion)
	for _, installation := range state.Installations {
		if installation.NeedsRebind {
			plan.NeedsRebind++
		}
	}
	return plan, nil
}

// MigrateCurrentV2 performs the explicit schema 2-4 -> current migration in
// place. It is intentionally separate from MigrateExpected, which migrates the
// pre-v2 legacy store into a new authoritative file.
func (migrator Migrator) MigrateCurrentV2(ctx context.Context, expectedDigest string) (Report, error) {
	if migrator.Lock == nil {
		return Report{}, fmt.Errorf("global mutation lock is required for state migration")
	}
	if migrator.PendingCheck == nil {
		return Report{}, fmt.Errorf("read-only pending recovery check is required for state migration")
	}
	if err := migrator.PendingCheck(ctx); err != nil {
		return Report{}, fmt.Errorf("old-schema pending recovery blocks migration: %w", err)
	}
	release, err := migrator.Lock.Acquire(ctx)
	if err != nil {
		return Report{}, err
	}
	defer func() { _ = release() }()
	if err := migrator.PendingCheck(ctx); err != nil {
		return Report{}, fmt.Errorf("old-schema pending recovery blocks migration: %w", err)
	}
	body, err := os.ReadFile(migrator.V2Store.Path)
	if err != nil {
		return Report{}, err
	}
	actual := digest(body)
	if strings.TrimSpace(expectedDigest) == "" || expectedDigest != actual {
		return Report{}, ErrPlanChanged
	}
	var header struct {
		SchemaVersion int `json:"schema_version"`
	}
	if err := json.Unmarshal(body, &header); err != nil {
		return Report{}, err
	}
	if header.SchemaVersion < domain.LegacyStateSchemaVersion || header.SchemaVersion > domain.PreviousStateSchemaV4 {
		if header.SchemaVersion == domain.StateSchemaVersion {
			return Report{}, fmt.Errorf("state already uses schema %d; binary rollback is fixed-forward only", domain.StateSchemaVersion)
		}
		return Report{}, fmt.Errorf("unsupported state schema_version %d", header.SchemaVersion)
	}
	state, err := migrator.V2Store.Load()
	if err != nil {
		return Report{}, err
	}
	if err := refusePendingOldState(state); err != nil {
		return Report{}, err
	}
	migrateLegacyOrigins(&state, header.SchemaVersion)
	state.SchemaVersion = domain.StateSchemaVersion
	if err := statev2.Validate(state); err != nil {
		return Report{}, fmt.Errorf("validate migrated state before backup: %w", err)
	}
	now := time.Now().UTC()
	if migrator.Now != nil {
		now = migrator.Now().UTC()
	}
	backupPath := fmt.Sprintf("%s.schema%d.backup-agentplugins-%s-%s", migrator.V2Store.Path, header.SchemaVersion,
		now.Format("20060102T150405.000000000Z"), strings.TrimPrefix(actual, "sha256:")[:12])
	if _, err := os.Lstat(backupPath); err == nil {
		return Report{}, fmt.Errorf("state backup already exists: %s", backupPath)
	} else if !os.IsNotExist(err) {
		return Report{}, err
	}
	if err := atomicfile.Write(backupPath, body, 0o600); err != nil {
		return Report{}, fmt.Errorf("backup schema %d state: %w", header.SchemaVersion, err)
	}
	if err := migrator.V2Store.Save(state); err != nil {
		return Report{BackupPath: backupPath}, fmt.Errorf("commit migrated state: %w", err)
	}
	report := Report{BackupPath: backupPath, Migrated: len(state.Installations), LegacyStateUntouched: false}
	for _, installation := range state.Installations {
		if installation.NeedsRebind {
			report.NeedsRebind++
		}
	}
	return report, nil
}

// Schema 4 already carries explicit origins; only earlier schemas need inference.
func migrateLegacyOrigins(state *domain.StateFileV2, sourceSchema int) {
	if sourceSchema >= domain.PreviousStateSchemaV4 {
		return
	}
	for index := range state.Installations {
		migrateV2Origin(&state.Installations[index])
	}
}

func migrateV2Origin(installation *domain.Installation) {
	installation.OriginMode = domain.OriginModeDirect
	installation.Directory = nil
	var evidence *domain.CatalogEvidence
	for _, client := range installation.Clients {
		if client.PackageRevision != nil && client.PackageRevision.CatalogEvidence != nil {
			evidence = client.PackageRevision.CatalogEvidence
			break
		}
	}
	requested := strings.TrimSpace(installation.Source.RequestedSource)
	directoryShaped := requested != "" && !strings.ContainsAny(requested, "/\\@") && !strings.HasPrefix(requested, ".") &&
		(strings.EqualFold(strings.TrimSpace(installation.Source.Repository), "777genius/universal-agent-plugins") ||
			strings.Contains(strings.ToLower(installation.Source.CanonicalSource), "github.com/777genius/universal-agent-plugins"))
	if evidence == nil && !directoryShaped {
		return
	}
	revision, installationRevisionOK := exactFullGitRevision(installation.Source.ResolvedRevision)
	clientRevisionsOK := true
	for _, client := range installation.Clients {
		if client.PackageRevision == nil {
			clientRevisionsOK = false
			break
		}
		if _, ok := exactFullGitRevision(client.PackageRevision.ResolvedRevision); !ok {
			clientRevisionsOK = false
			break
		}
	}
	if !installationRevisionOK || !clientRevisionsOK {
		// Old catalog evidence and short-name sources identify a likely Directory
		// record, but every installation and applied client revision must independently
		// prove its immutable Directory release. Keep the original direct provenance,
		// revisions, and digests instead of relabeling them with invented provenance.
		installation.NeedsRebind = true
		return
	}
	// The frozen catalog had one community distribution per manifest name.
	// Preserve that source; never map it to a newly preferred upstream source.
	distributionID := "777genius/" + installation.DeclaredName
	installation.Source.ResolvedRevision = revision
	installation.OriginMode = domain.OriginModeDirectory
	installation.Directory = &domain.DirectoryOrigin{ProductID: installation.DeclaredName, DistributionID: distributionID,
		DistributionKind: domain.DistributionCommunity, DesiredReleaseSequence: 1}
	for key, client := range installation.Clients {
		client.PackageRevision.DistributionID = distributionID
		client.PackageRevision.ReleaseSequence = 1
		installation.Clients[key] = client
	}
}

type legacyState struct {
	SchemaVersion int                  `json:"schema_version"`
	Installations []legacyInstallation `json:"installations"`
}

type legacyInstallation struct {
	IntegrationID      string                  `json:"integration_id"`
	RequestedSourceRef legacySourceRef         `json:"requested_source_ref"`
	ResolvedSourceRef  legacySourceRef         `json:"resolved_source_ref"`
	ResolvedVersion    string                  `json:"resolved_version"`
	SourceDigest       string                  `json:"source_digest"`
	ManifestDigest     string                  `json:"manifest_digest"`
	Policy             legacyPolicy            `json:"policy"`
	WorkspaceRoot      string                  `json:"workspace_root"`
	Targets            map[string]legacyTarget `json:"targets"`
	LastCheckedAt      string                  `json:"last_checked_at"`
	LastUpdatedAt      string                  `json:"last_updated_at"`
}

type legacySourceRef struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
}

type legacyPolicy struct {
	Scope string `json:"scope"`
}

type legacyTarget struct {
	TargetID             string               `json:"target_id"`
	State                string               `json:"state"`
	ActivationState      string               `json:"activation_state"`
	InteractiveAuthState string               `json:"interactive_auth_state"`
	OwnedNativeObjects   []legacyNativeObject `json:"owned_native_objects"`
}

type legacyNativeObject struct {
	Kind            string `json:"kind"`
	Name            string `json:"name"`
	Path            string `json:"path"`
	ProtectionClass string `json:"protection_class"`
}
