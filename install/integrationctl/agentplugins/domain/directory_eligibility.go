package domain

import (
	"fmt"
	"strings"
)

func directoryEligibilityTargets(targets []ClientID) []ClientID {
	complete := append([]ClientID(nil), targets...)
	for _, target := range targets {
		complete = append(complete, BackendSiblings(target)...)
	}
	return uniqueClients(complete)
}

// hasPassedUpstreamMaterialization is the static promotion/package gate. Its
// environment dimensions describe the isolated promotion run and must not be
// mistaken for evidence about the caller's local environment.
func hasPassedUpstreamMaterialization(snapshot DirectorySnapshot, distribution DirectoryDistribution, release DirectoryRelease, policy DirectoryReleasePolicy, client ClientID) bool {
	for _, evidenceID := range policy.CurrentEvidence {
		evidence := evidenceByID(snapshot, evidenceID)
		if evidence != nil && evidence.HasTrustedEligibilityProvenanceAtSequence(snapshot.Sequence) && evidence.DistributionID == distribution.ID && evidence.ReleaseSequence == release.Sequence &&
			evidence.PackageTreeDigest == release.TreeDigest && evidence.Client == client && evidence.Level == "materialization" && evidence.Outcome == "passed" {
			return true
		}
	}
	return false
}

func directoryEvidenceApplies(e DirectoryEvidence, distribution DirectoryDistribution, release DirectoryRelease, client ClientID, request DirectoryResolveRequest) bool {
	if e.DistributionID != distribution.ID || e.ReleaseSequence != release.Sequence || e.PackageTreeDigest != release.TreeDigest {
		return false
	}
	if e.Level == "schema" {
		return e.Client == ""
	}
	if e.Client != client || !evidenceDimensionMatches(e.InstallerVersion, request.InstallerVersion) ||
		!evidenceDimensionMatches(e.OS, request.OS) || !evidenceDimensionMatches(e.Architecture, request.Architecture) {
		return false
	}
	if e.ClientVersion != "" && request.ClientVersions[e.Client] != e.ClientVersion {
		return false
	}
	if e.DependencyIdentity != "" && request.DependencyIdentity[e.Client] != e.DependencyIdentity {
		return false
	}
	return true
}

func evidenceDimensionMatches(recorded, actual string) bool {
	return recorded == "" || recorded == actual
}

func selectionFrom(snapshot DirectorySnapshot, product DirectoryProduct, distribution DirectoryDistribution, release DirectoryRelease, fallback bool, diagnostics []DirectoryDiagnostic) DirectorySelection {
	return DirectorySelection{ProductID: product.ID, DistributionID: distribution.ID, DistributionKind: distribution.Kind, ReleaseSequence: release.Sequence, PackageVersion: release.PackageVersion, Source: release.PackageSource, TreeDigestAlgorithm: release.TreeDigestAlgorithm, TreeDigest: release.TreeDigest, ManifestDigest: release.ManifestDigest, SnapshotSequence: snapshot.Sequence, Fallback: fallback, Diagnostics: append([]DirectoryDiagnostic(nil), diagnostics...)}
}

func distributionByID(snapshot DirectorySnapshot, product DirectoryProduct, id string) *DirectoryDistribution {
	if !containsString(product.Distributions, id) {
		return nil
	}
	for i := range snapshot.Distributions {
		if snapshot.Distributions[i].ID == id && snapshot.Distributions[i].ProductID == product.ID {
			return &snapshot.Distributions[i]
		}
	}
	return nil
}
func targetByClient(values []DirectoryTarget, client ClientID) *DirectoryTarget {
	for i := range values {
		if values[i].Client == client {
			return &values[i]
		}
	}
	return nil
}
func evidenceByID(snapshot DirectorySnapshot, id string) *DirectoryEvidence {
	for i := range snapshot.Evidence {
		if snapshot.Evidence[i].ID == id {
			return &snapshot.Evidence[i]
		}
	}
	return nil
}
func isRevoked(snapshot DirectorySnapshot, distribution string, sequence uint64) bool {
	for _, item := range snapshot.Revocations {
		if item.DistributionID == distribution && item.ReleaseSequence == sequence {
			return true
		}
	}
	return false
}
func containsString(values []string, value string) bool {
	for _, item := range values {
		if item == value {
			return true
		}
	}
	return false
}
func containsScope(values []InstallScope, value InstallScope) bool {
	for _, item := range values {
		if item == value {
			return true
		}
	}
	return false
}
func uniqueClients(values []ClientID) []ClientID {
	seen := map[ClientID]bool{}
	out := []ClientID{}
	for _, v := range values {
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}
func ineligibleError(id string, reasons []eligibilityReason) error {
	parts := make([]string, 0, len(reasons))
	noSafeUpdate := false
	for _, r := range reasons {
		parts = append(parts, r.message)
		noSafeUpdate = noSafeUpdate || r.code == "no_safe_update"
	}
	if len(parts) == 0 {
		parts = []string{"no release matches the operation"}
	}
	if noSafeUpdate {
		return fmt.Errorf("%w: %w in %q: %s", ErrDirectoryIneligible, ErrDirectoryNoSafeUpdate, id, strings.Join(parts, "; "))
	}
	return fmt.Errorf("%w in %q: %s", ErrDirectoryIneligible, id, strings.Join(parts, "; "))
}
func joinDiagnosticMessages(values []DirectoryDiagnostic) string {
	parts := make([]string, 0, len(values))
	for _, v := range values {
		parts = append(parts, v.DistributionID+": "+v.Message)
	}
	return strings.Join(parts, "; ")
}

