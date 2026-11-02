package agentpluginscli

import (
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/discoveryv1"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func inspectDiscoveryOutdated(bundle discoveryv1.VerifiedBundle, bundleErr error, installation domain.Installation, scope string) outdatedInstallation {
	item := discoveryOutdatedBase(installation, scope)
	if installation.NeedsRebind {
		item.Status, item.Reason = "blocked", "installation requires explicit rebind before update"
		return item
	}
	if bundleErr != nil {
		item.Status, item.Reason = "unknown", "no authenticated Discovery snapshot is available: "+bundleErr.Error()
		return item
	}
	record, ok := matchDiscoveryOutdatedRecord(bundle, strings.TrimSpace(installation.Source.RequestedSource))
	if !ok {
		item.Status, item.Reason = "blocked", "recorded Discovery selector is absent or ambiguous"
		return item
	}
	return compareDiscoveryRelease(record, installation, item)
}

func discoveryOutdatedBase(installation domain.Installation, scope string) outdatedInstallation {
	return outdatedInstallation{
		InstallationID: installation.InstallationID, Name: installation.DeclaredName, Targets: installationTargets(installation, scope),
		Installed: &outdatedReleaseIdentity{
			DistributionID: strings.TrimSpace(installation.Source.RequestedSource), PackageVersion: installation.Package.Version,
			Revision: installation.Source.ResolvedRevision, TreeDigest: installation.Source.TreeDigest, ManifestDigest: installation.Package.ManifestDigest,
		},
	}
}

func matchDiscoveryOutdatedRecord(bundle discoveryv1.VerifiedBundle, selector string) (discoveryv1.Record, bool) {
	var matches []discoveryv1.Record
	for _, record := range bundle.Search.Records {
		if record.Slug == selector {
			matches = append(matches, record)
		}
	}
	if len(matches) != 1 {
		return discoveryv1.Record{}, false
	}
	return matches[0], true
}

func compareDiscoveryRelease(record discoveryv1.Record, installation domain.Installation, item outdatedInstallation) outdatedInstallation {
	item.Available = &outdatedReleaseIdentity{DistributionID: record.Slug, Revision: record.Revision, TreeDigest: record.TreeDigest, ManifestDigest: record.ManifestDigest}
	if record.Version != nil {
		item.Available.PackageVersion = *record.Version
	}
	if record.Availability != "available" {
		item.Status, item.Reason = "blocked", "Discovery source is unavailable"
		return item
	}
	if record.Repository != installation.Source.Repository || record.PackagePath != installation.Source.PackageSubpath {
		item.Status, item.Reason = "blocked", "Discovery selector changed repository or package path; explicit switch required"
		return item
	}
	if record.Revision == installation.Source.ResolvedRevision && record.TreeDigest == installation.Source.TreeDigest && record.ManifestDigest == installation.Package.ManifestDigest {
		item.Status, item.Reason = "current", "the signed Discovery record matches the installed immutable package"
		item.Available = nil
		return item
	}
	item.Status, item.Reason = "outdated", "the same Discovery source has a newer immutable package identity"
	return item
}
