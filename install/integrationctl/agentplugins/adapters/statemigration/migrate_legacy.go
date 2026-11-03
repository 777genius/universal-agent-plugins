package statemigration

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/atomicfile"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/statev2"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

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
		return nil, legacyState{}, fmt.Errorf("agent plugins state already exists; migration is not idempotent over authoritative state")
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
