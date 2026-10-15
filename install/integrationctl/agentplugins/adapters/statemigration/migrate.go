package statemigration

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/atomicfile"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/statev2"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
)

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

// PlanCurrentV2 inspects authoritative schema 2 without writing it. The digest
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
	for index := range state.Installations {
		installation := &state.Installations[index]
		migrateV2Origin(installation)
		if installation.NeedsRebind {
			plan.NeedsRebind++
		}
	}
	return plan, nil
}

// MigrateCurrentV2 performs the explicit schema 2 -> current migration in
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
		return Report{}, fmt.Errorf("state changed after review; rerun migrate-state --dry-run")
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
	for installationIndex := range state.Installations {
		installation := &state.Installations[installationIndex]
		migrateV2Origin(installation)
	}
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
		return Report{}, fmt.Errorf("backup schema 2 state: %w", err)
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

func (migrator Migrator) Plan() (Plan, error) {
	body, legacy, err := migrator.load()
	if err != nil {
		return Plan{}, err
	}
	plan := Plan{LegacyDigest: digest(body), Installations: len(legacy.Installations)}
	duplicates := duplicateLegacySourceBindings(legacy.Installations)
	for _, record := range legacy.Installations {
		if needsRebind(record) || duplicates[legacySourceBindingID(record)] {
			plan.NeedsRebind++
		}
	}
	return plan, nil
}

func (migrator Migrator) Migrate() (Report, error) {
	return migrator.MigrateExpected("")
}

func (migrator Migrator) MigrateExpected(expectedDigest string) (Report, error) {
	body, legacy, err := migrator.load()
	if err != nil {
		return Report{}, err
	}
	actualDigest := digest(body)
	if strings.TrimSpace(expectedDigest) != "" && expectedDigest != actualDigest {
		return Report{}, fmt.Errorf("legacy state changed after review; run migrate-state again")
	}
	now := time.Now().UTC()
	if migrator.Now != nil {
		now = migrator.Now().UTC()
	}
	newID := domain.NewInstallationID
	if migrator.NewID != nil {
		newID = migrator.NewID
	}
	state := domain.StateFileV2{SchemaVersion: domain.StateSchemaVersion}
	report := Report{LegacyStateUntouched: true}
	for _, record := range legacy.Installations {
		installation, err := migrateInstallation(record, now, newID)
		if err != nil {
			return report, err
		}
		if installation.NeedsRebind {
			report.NeedsRebind++
		}
		state.Installations = append(state.Installations, installation)
		report.Migrated++
	}
	deduplicateMigratedSourceBindings(state.Installations)
	report.NeedsRebind = 0
	for _, installation := range state.Installations {
		if installation.NeedsRebind {
			report.NeedsRebind++
		}
	}
	if err := statev2.Validate(state); err != nil {
		return report, fmt.Errorf("validate complete migrated state before backup: %w", err)
	}
	backupPath := migrator.LegacyPath + ".backup-agentplugins-" + now.Format("20060102T150405.000000000Z") + "-" + strings.TrimPrefix(actualDigest, "sha256:")[:12]
	if _, err := os.Lstat(backupPath); err == nil {
		return report, fmt.Errorf("legacy state backup already exists: %s", backupPath)
	} else if !os.IsNotExist(err) {
		return report, fmt.Errorf("inspect legacy state backup: %w", err)
	}
	if err := atomicfile.Write(backupPath, body, 0o600); err != nil {
		return report, fmt.Errorf("backup legacy state: %w", err)
	}
	report.BackupPath = backupPath
	if err := migrator.V2Store.Save(state); err != nil {
		return report, fmt.Errorf("commit migrated Agent Plugins state: %w", err)
	}
	return report, nil
}

func (migrator Migrator) load() ([]byte, legacyState, error) {
	if strings.TrimSpace(migrator.LegacyPath) == "" || strings.TrimSpace(migrator.V2Store.Path) == "" {
		return nil, legacyState{}, fmt.Errorf("legacy and Agent Plugins state paths are required")
	}
	if _, err := os.Stat(migrator.V2Store.Path); err == nil {
		return nil, legacyState{}, fmt.Errorf("Agent Plugins state already exists; migration is not idempotent over authoritative state")
	} else if !os.IsNotExist(err) {
		return nil, legacyState{}, err
	}
	body, err := os.ReadFile(migrator.LegacyPath)
	if err != nil {
		return nil, legacyState{}, fmt.Errorf("read legacy state: %w", err)
	}
	var legacy legacyState
	if err := json.Unmarshal(body, &legacy); err != nil {
		return nil, legacyState{}, fmt.Errorf("decode legacy state: %w", err)
	}
	if legacy.SchemaVersion != 0 && legacy.SchemaVersion != 1 {
		return nil, legacyState{}, fmt.Errorf("unsupported legacy state schema_version %d", legacy.SchemaVersion)
	}
	return body, legacy, nil
}

func digest(body []byte) string {
	sum := sha256.Sum256(body)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func needsRebind(record legacyInstallation) bool {
	if strings.TrimSpace(record.RequestedSourceRef.Value) == "" ||
		strings.TrimSpace(record.ResolvedSourceRef.Value) == "" ||
		strings.TrimSpace(record.SourceDigest) == "" {
		return true
	}
	if legacyDirectoryShaped(record) {
		_, exactRevision := exactLegacyGitRevision(record.ResolvedSourceRef.Value)
		return !exactRevision || strings.TrimSpace(record.ManifestDigest) == ""
	}
	return false
}

func legacySourceBindingID(record legacyInstallation) string {
	requested := strings.TrimSpace(record.RequestedSourceRef.Value)
	canonical := strings.TrimSpace(record.ResolvedSourceRef.Value)
	if canonical == "" {
		canonical = requested
	}
	return domain.ComputeSourceBindingID(domain.SourceIdentity{RequestedSource: requested, CanonicalSource: canonical})
}

func duplicateLegacySourceBindings(records []legacyInstallation) map[string]bool {
	counts := map[string]int{}
	for _, record := range records {
		counts[legacySourceBindingID(record)]++
	}
	duplicates := map[string]bool{}
	for sourceID, count := range counts {
		duplicates[sourceID] = count > 1
	}
	return duplicates
}

func deduplicateMigratedSourceBindings(installations []domain.Installation) {
	counts := map[string]int{}
	for _, installation := range installations {
		counts[installation.Source.SourceBindingID]++
	}
	for index := range installations {
		installation := &installations[index]
		if counts[installation.Source.SourceBindingID] < 2 {
			continue
		}
		original := installation.Source.SourceBindingID
		sum := sha256.Sum256([]byte(original + "\x00" + installation.InstallationID))
		installation.Source.SourceBindingID = "legacy_src_" + hex.EncodeToString(sum[:12])
		installation.NeedsRebind = true
	}
}

func migrateInstallation(record legacyInstallation, now time.Time, newID func() (string, error)) (domain.Installation, error) {
	installationID, err := newID()
	if err != nil {
		return domain.Installation{}, err
	}
	requested := strings.TrimSpace(record.RequestedSourceRef.Value)
	canonical := strings.TrimSpace(record.ResolvedSourceRef.Value)
	if canonical == "" {
		canonical = requested
	}
	sourceBindingID := legacySourceBindingID(record)
	requiresRebind := needsRebind(record)
	directory := legacyDirectoryOrigin(record)
	resolvedRevision := record.ResolvedSourceRef.Value
	if directory != nil {
		resolvedRevision, _ = exactLegacyGitRevision(record.ResolvedSourceRef.Value)
	}
	clients := map[string]domain.ClientBinding{}
	targetNames := make([]string, 0, len(record.Targets))
	for targetName := range record.Targets {
		targetNames = append(targetNames, targetName)
	}
	sort.Strings(targetNames)
	for _, targetName := range targetNames {
		target := record.Targets[targetName]
		clientID := strings.TrimSpace(target.TargetID)
		if clientID == "" {
			clientID = targetName
		}
		targetLocator := "user"
		if strings.EqualFold(record.Policy.Scope, "project") && strings.TrimSpace(record.WorkspaceRoot) != "" {
			targetLocator = filepath.Clean(record.WorkspaceRoot)
		}
		bindingID := domain.ComputeClientBindingID(installationID, clientID, record.Policy.Scope, targetLocator)
		physicalID := domain.ComputePhysicalArtifactID(record.IntegrationID, installationID+"\x00"+clientID)
		client := domain.ClientBinding{
			ClientBindingID:  bindingID,
			ClientID:         clientID,
			Scope:            record.Policy.Scope,
			TargetLocator:    targetLocator,
			PhysicalArtifact: physicalID,
			Materialization:  migrateMaterialization(target.State),
			Activation:       migrateActivation(target.ActivationState),
			Authentication:   migrateAuthentication(target.InteractiveAuthState),
			Policy:           domain.PolicyAllowed,
			Verification:     domain.VerificationNotRun,
			UpdatedAt:        firstNonEmpty(record.LastUpdatedAt, now.Format(time.RFC3339)),
		}
		if directory != nil {
			client.PackageRevision = &domain.ClientPackageRevision{Version: record.ResolvedVersion, ResolvedRevision: resolvedRevision,
				TreeDigest: record.SourceDigest, ManifestDigest: record.ManifestDigest, DistributionID: directory.DistributionID, ReleaseSequence: directory.DesiredReleaseSequence}
		}
		for _, object := range target.OwnedNativeObjects {
			client.NativeObjects = append(client.NativeObjects, domain.NativeObjectOwnership{
				ObjectID:        nativeObjectID(bindingID, object),
				Kind:            object.Kind,
				LogicalName:     object.Name,
				Path:            object.Path,
				ProtectionClass: object.ProtectionClass,
			})
		}
		clients[bindingID] = client
	}
	createdAt := firstNonEmpty(record.LastCheckedAt, record.LastUpdatedAt, now.Format(time.RFC3339))
	updatedAt := firstNonEmpty(record.LastUpdatedAt, record.LastCheckedAt, now.Format(time.RFC3339))
	return domain.Installation{
		InstallationID: installationID,
		DeclaredName:   record.IntegrationID,
		OriginMode:     legacyOriginMode(directory),
		Directory:      directory,
		Source: domain.SourceBinding{
			SourceBindingID:  sourceBindingID,
			RequestedSource:  requested,
			CanonicalSource:  canonical,
			ResolvedRevision: resolvedRevision,
			TreeDigest:       record.SourceDigest,
		},
		Package: domain.PackageBinding{
			LoaderKind:     domain.LoaderKindLegacy,
			FormatID:       domain.FormatIDLegacyV1,
			SchemaURI:      "plugin.yaml/v1",
			DeclaredName:   record.IntegrationID,
			Version:        record.ResolvedVersion,
			ManifestDigest: record.ManifestDigest,
		},
		Clients:     clients,
		NeedsRebind: requiresRebind,
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
	}, nil
}

func legacyOriginMode(directory *domain.DirectoryOrigin) domain.OriginMode {
	if directory != nil {
		return domain.OriginModeDirectory
	}
	return domain.OriginModeDirect
}

func legacyDirectoryOrigin(record legacyInstallation) *domain.DirectoryOrigin {
	if !legacyDirectoryShaped(record) || strings.TrimSpace(record.SourceDigest) == "" || strings.TrimSpace(record.ManifestDigest) == "" {
		return nil
	}
	if _, ok := exactLegacyGitRevision(record.ResolvedSourceRef.Value); !ok {
		return nil
	}
	return &domain.DirectoryOrigin{ProductID: record.IntegrationID, DistributionID: "777genius/" + record.IntegrationID,
		DistributionKind: domain.DistributionCommunity, DesiredReleaseSequence: 1}
}

func legacyDirectoryShaped(record legacyInstallation) bool {
	kind := strings.ToLower(strings.TrimSpace(record.RequestedSourceRef.Kind))
	value := strings.TrimSpace(record.RequestedSourceRef.Value)
	return kind == "github_repo_path" && value != "" && !strings.Contains(value, "/") &&
		!strings.HasPrefix(value, ".") && !filepath.IsAbs(value)
}

func exactFullGitRevision(source string) (string, bool) {
	value := source
	if len(value) != 40 {
		return "", false
	}
	for _, char := range value {
		if (char < '0' || char > '9') && (char < 'a' || char > 'f') {
			return "", false
		}
	}
	return value, true
}

// exactLegacyGitRevision accepts only immutable legacy source forms whose Git
// object identity is explicit: a raw SHA, or the suffix after the final '@'
// and before an optional package subpath.
func exactLegacyGitRevision(source string) (string, bool) {
	value := strings.TrimSpace(source)
	if at := strings.LastIndex(value, "@"); at >= 0 {
		value = value[at+1:]
		if subpath := strings.Index(value, "//"); subpath >= 0 {
			value = value[:subpath]
		}
	}
	return exactFullGitRevision(value)
}

func nativeObjectID(bindingID string, object legacyNativeObject) string {
	sum := sha256.Sum256([]byte(strings.Join([]string{bindingID, object.Kind, object.Name, object.Path}, "\x00")))
	return "object_" + hex.EncodeToString(sum[:12])
}

func migrateMaterialization(state string) domain.MaterializationState {
	switch strings.ToLower(strings.TrimSpace(state)) {
	case "removed", "absent":
		return domain.MaterializationAbsent
	case "installed", "activation_pending", "auth_pending", "prepared":
		return domain.MaterializationMaterialized
	default:
		return domain.MaterializationDegraded
	}
}

func migrateActivation(state string) domain.ActivationState {
	switch strings.ToLower(strings.TrimSpace(state)) {
	case "active":
		return domain.ActivationActive
	case "not_required", "":
		return domain.ActivationNotRequired
	case "native_pending", "reload_pending", "restart_pending":
		return domain.ActivationManual
	default:
		return domain.ActivationPrepared
	}
}

func migrateAuthentication(state string) domain.AuthenticationState {
	switch strings.ToLower(strings.TrimSpace(state)) {
	case "auth_pending", "pending":
		return domain.AuthenticationPending
	case "authenticated", "complete":
		return domain.AuthenticationComplete
	case "failed":
		return domain.AuthenticationFailed
	default:
		return domain.AuthenticationNotRequired
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func refusePendingOldState(state domain.StateFileV2) error {
	refusal := fmt.Errorf("old-schema pending recovery requires the corresponding previous SDK/binary; recover with it, then rerun migration dry-run and apply")
	for _, r := range state.TransactionReceipts {
		if r.Phase != "" && r.Phase != "committed" {
			return refusal
		}
	}
	for _, i := range state.Installations {
		for _, b := range i.Clients {
			if b.PendingNativeIntent != nil {
				return refusal
			}
			for _, r := range b.Receipts {
				if r.Phase != "" && r.Phase != "committed" {
					return refusal
				}
			}
		}
	}
	return nil
}
