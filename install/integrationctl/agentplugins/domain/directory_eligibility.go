package domain

import (
	"fmt"
	"strings"
)

func directoryEligibilityTargets(targets []ClientID, scope InstallScope) []ClientID {
	complete := append([]ClientID(nil), targets...)
	for _, target := range targets {
		complete = append(complete, BackendSiblingsForScope(target, scope)...)
	}
	return uniqueClients(complete)
}

// directoryTrustedEligibilityEvidence keeps v1's historical sequence cutover
// independent from the new v2 sequence space. A v2 sequence 1 is not v1's
// historical legacy-evidence lane: it must carry explicit signed provenance.
func directoryTrustedEligibilityEvidence(snapshot DirectorySnapshot, evidence DirectoryEvidence) bool {
	if snapshot.SnapshotSchemaVersion == 2 {
		return evidence.HasTrustedEligibilityProvenance()
	}
	return evidence.HasTrustedEligibilityProvenanceAtSequence(snapshot.Sequence)
}

// hasPassedUpstreamMaterialization is the static promotion/package gate. Its
// environment dimensions describe the isolated promotion run and must not be
// mistaken for evidence about the caller's local environment.
func hasPassedUpstreamMaterialization(snapshot DirectorySnapshot, distribution DirectoryDistribution, release DirectoryRelease, policy DirectoryReleasePolicy, client ClientID, scope InstallScope) bool {
	evidenceIDs := policy.CurrentEvidence
	if target := targetByClientScope(policy.Targets, client, scope); target != nil {
		evidenceIDs = append(append([]string(nil), evidenceIDs...), target.CurrentEvidence...)
	}
	for _, evidenceID := range evidenceIDs {
		evidence := evidenceByID(snapshot, evidenceID)
		if evidence != nil && directoryTrustedEligibilityEvidence(snapshot, *evidence) && evidence.DistributionID == distribution.ID && evidence.ReleaseSequence == release.Sequence &&
			evidence.PackageTreeDigest == release.TreeDigest && evidence.Client == client && evidence.Level == "materialization" && evidence.Outcome == "passed" &&
			(scope == ScopeProject && evidence.Scope == ScopeProject || scope == ScopeUser && (evidence.Scope == "" || evidence.Scope == ScopeUser)) {
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
	// Historical V1 evidence is scoped to the user environment only.
	if request.Scope == ScopeProject && e.Scope != ScopeProject ||
		request.Scope == ScopeUser && e.Scope != "" && e.Scope != ScopeUser {
		return false
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

// targetByClientScope never borrows a policy for another scope. V1 targets
// contain a list of scopes; V2 grants carry exactly one explicit scope.
func targetByClientScope(values []DirectoryTarget, client ClientID, scope InstallScope) *DirectoryTarget {
	for i := range values {
		if values[i].Client != client {
			continue
		}
		if values[i].Scope == scope || values[i].Scope == "" && containsScope(values[i].Scopes, scope) {
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
