package statemigration

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

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
	for _, r := range state.TransactionReceipts {
		if r.Phase != "" && r.Phase != "committed" {
			return ErrPendingRecovery
		}
	}
	for _, i := range state.Installations {
		for _, b := range i.Clients {
			if b.PendingNativeIntent != nil {
				return ErrPendingRecovery
			}
			for _, r := range b.Receipts {
				if r.Phase != "" && r.Phase != "committed" {
					return ErrPendingRecovery
				}
			}
		}
	}
	return nil
}
