package agentpluginscli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func installedDirectoryWarnings(snapshot domain.DirectorySnapshot, installation domain.Installation) []publicSafetyWarning {
	if installation.Directory == nil {
		return nil
	}
	origin := installation.Directory
	warnings := installationDigestWarnings(snapshot, installation)
	if distribution := snapshotDistribution(snapshot, origin.DistributionID); distribution != nil && distribution.Status == domain.DistributionSuspended {
		warnings = append(warnings, publicSafetyWarning{Code: "directory_distribution_suspended", DistributionID: origin.DistributionID,
			Message: "the signed Directory has suspended the installed distribution", Action: "review an explicit qualified switch or remove the installation"})
	}
	return warnings
}

func installationDigestWarnings(snapshot domain.DirectorySnapshot, installation domain.Installation) []publicSafetyWarning {
	warnings := exactDigestWarnings(snapshot, installation.Source.TreeDigest, installation.InstallationID)
	byIdentity := map[string]int{}
	for index, warning := range warnings {
		byIdentity[warning.Code+"\x00"+warning.DistributionID+fmt.Sprint("\x00", warning.ReleaseSequence)] = index
	}
	for _, binding := range installation.Clients {
		if binding.PackageRevision == nil {
			continue
		}
		for _, warning := range exactDigestWarnings(snapshot, binding.PackageRevision.TreeDigest, installation.InstallationID) {
			key := warning.Code + "\x00" + warning.DistributionID + fmt.Sprint("\x00", warning.ReleaseSequence)
			index, exists := byIdentity[key]
			if !exists {
				index = len(warnings)
				byIdentity[key] = index
				warnings = append(warnings, warning)
			}
			warnings[index].Clients = appendUniqueString(warnings[index].Clients, binding.ClientID)
		}
	}
	for index := range warnings {
		sort.Strings(warnings[index].Clients)
		if installation.OriginMode == domain.OriginModeDirect {
			warnings[index].Action = "choose a different reviewed local/full-SHA source, or remove the installation"
		}
	}
	return warnings
}

func appendUniqueString(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func exactDigestWarnings(snapshot domain.DirectorySnapshot, treeDigest, installationID string) []publicSafetyWarning {
	if strings.TrimSpace(treeDigest) == "" {
		return nil
	}
	warnings := []publicSafetyWarning{}
	for _, distribution := range snapshot.Distributions {
		for _, release := range distribution.Releases {
			if release.TreeDigest != treeDigest || !releaseRevokedInSnapshot(snapshot, distribution, release.Sequence) {
				continue
			}
			subject := "installed package bytes"
			if installationID == "the direct source" {
				subject = "direct package bytes"
			}
			warnings = append(warnings, publicSafetyWarning{Code: "directory_release_revoked", DistributionID: distribution.ID, ReleaseSequence: release.Sequence,
				Message: fmt.Sprintf("the signed Directory identifies the %s as revoked (%s release %d)", subject, distribution.ID, release.Sequence),
				Action:  "run `agentplugins update " + installationID + "` for a safe release, explicitly switch distributions, or remove the installation"})
		}
	}
	return warnings
}

func releaseRevokedInSnapshot(snapshot domain.DirectorySnapshot, distribution domain.DirectoryDistribution, sequence uint64) bool {
	for _, policy := range distribution.ReleasePolicies {
		if policy.ReleaseSequence == sequence && policy.Status == domain.ReleaseRevoked {
			return true
		}
	}
	for _, revocation := range snapshot.Revocations {
		if revocation.DistributionID == distribution.ID && revocation.ReleaseSequence == sequence {
			return true
		}
	}
	return false
}

func convergenceState(installation domain.Installation) (bool, string) {
	pending := []string{}
	for _, binding := range installation.Clients {
		if binding.Materialization == domain.MaterializationAbsent || binding.PackageRevision == nil {
			continue
		}
		revision := binding.PackageRevision
		mixed := revision.TreeDigest != installation.Source.TreeDigest || revision.ManifestDigest != installation.Package.ManifestDigest
		if installation.Directory != nil && revision.ReleaseSequence != installation.Directory.DesiredReleaseSequence {
			mixed = true
		}
		if mixed {
			pending = appendUniqueString(pending, binding.ClientID)
		}
	}
	if len(pending) == 0 {
		return false, ""
	}
	sort.Strings(pending)
	return true, fmt.Sprintf("run `agentplugins update %s --target %s` to converge the remaining clients", installation.InstallationID, strings.Join(pending, ","))
}
