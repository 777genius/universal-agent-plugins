package domain

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

func releaseEligibility(snapshot DirectorySnapshot, product DirectoryProduct, distribution DirectoryDistribution, release DirectoryRelease, policy DirectoryReleasePolicy, request DirectoryResolveRequest) *eligibilityReason {
	if release.ManifestName != product.ManifestName {
		return &eligibilityReason{"manifest_mismatch", "release manifest identity differs from product"}
	}
	if request.SchemaVersion != "" && request.SchemaVersion != "1.0.0" {
		return &eligibilityReason{"unsupported_schema", "required Agent Plugins schema is not supported"}
	}
	if release.AgentPluginsSchema != "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json" {
		return &eligibilityReason{"unsupported_schema", "release Agent Plugins schema is not supported"}
	}
	if policy.MinimumInstallerVersion != "" {
		comparison, err := compareSemver(request.InstallerVersion, policy.MinimumInstallerVersion)
		if err != nil || comparison < 0 {
			return &eligibilityReason{"installer_too_old", "minimum installer version is " + policy.MinimumInstallerVersion}
		}
	}
	for _, evidenceID := range policy.CurrentEvidence {
		if e := evidenceByID(snapshot, evidenceID); e != nil && e.HasTrustedEligibilityProvenanceAtSequence(snapshot.Sequence) && directoryEvidenceApplies(*e, distribution, release, "", request) && e.Level == "schema" && e.Outcome == "failed" {
			return &eligibilityReason{"blocking_evidence", "current trusted schema evidence failed"}
		}
	}
	if request.Purpose == DirectoryResolveContext7ChatGPTPreparation && !hasContext7PreparationPromotion(snapshot, distribution, release, policy) {
		return &eligibilityReason{"upstream_materialization_required", "Context7 preparation requires trusted materialization on a declared supported target"}
	}
	for _, target := range directoryEligibilityTargets(request.Targets) {
		entry := targetByClient(policy.Targets, target)
		// Only absence of signed ChatGPT support can use the source-only lane.
		if entry != nil || request.Purpose != DirectoryResolveContext7ChatGPTPreparation {
			if entry == nil || !containsScope(entry.Scopes, request.Scope) {
				return &eligibilityReason{"incomplete_targets", "release does not support complete target set; missing " + string(target)}
			}
			delivery, supported := ExpectedDirectoryDelivery(target)
			if !supported || entry.Delivery != delivery {
				return &eligibilityReason{"incompatible_delivery", fmt.Sprintf("release delivery %q is incompatible with %s; expected %q", entry.Delivery, target, delivery)}
			}
			if distribution.Kind == DistributionUpstream && !hasPassedUpstreamMaterialization(snapshot, distribution, release, policy, target) {
				return &eligibilityReason{"upstream_materialization_required", "upstream release lacks current passed materialization evidence for " + string(target)}
			}
		}
		for _, evidenceID := range policy.CurrentEvidence {
			if e := evidenceByID(snapshot, evidenceID); e != nil && e.HasTrustedEligibilityProvenanceAtSequence(snapshot.Sequence) && directoryEvidenceApplies(*e, distribution, release, target, request) && (e.Level == "materialization" || e.Level == "discovery" || e.Level == "runtime" || e.Level == "oauth") && e.Outcome == "failed" {
				return &eligibilityReason{"blocking_evidence", "current trusted " + e.Level + " evidence failed for " + string(target)}
			}
		}
	}
	required := append([]string(nil), request.RequiredComponents...)
	if product.MinimumCapabilities.Skills == "required" {
		required = append(required, "skills")
	}
	if product.MinimumCapabilities.MCP == "required" {
		required = append(required, "mcp")
	}
	for _, component := range required {
		if !containsString(release.Components, component) {
			return &eligibilityReason{"missing_component", "release lacks required component " + component}
		}
	}
	return nil
}

// Directory policy, evidence, and delivery must authorize every logical surface
// that shares a physical backend, even when the user selected only one of them.
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

// compareSemver is used only for the installer compatibility floor. Directory
// release ordering always uses signed release sequence.
func compareSemver(left, right string) (int, error) {
	type version struct {
		core [3]uint64
		pre  []string
	}
	parse := func(value string) (version, error) {
		var v version
		value = strings.SplitN(strings.TrimSpace(value), "+", 2)[0]
		parts := strings.SplitN(value, "-", 2)
		core := strings.Split(parts[0], ".")
		if len(core) != 3 {
			return v, errors.New("not semver")
		}
		for i, s := range core {
			n, e := strconv.ParseUint(s, 10, 64)
			if e != nil || (len(s) > 1 && s[0] == '0') {
				return v, errors.New("not semver")
			}
			v.core[i] = n
		}
		if len(parts) == 2 {
			if parts[1] == "" {
				return v, errors.New("not semver")
			}
			v.pre = strings.Split(parts[1], ".")
		}
		return v, nil
	}
	a, e := parse(left)
	if e != nil {
		return 0, e
	}
	b, e := parse(right)
	if e != nil {
		return 0, e
	}
	for i := range 3 {
		if a.core[i] < b.core[i] {
			return -1, nil
		}
		if a.core[i] > b.core[i] {
			return 1, nil
		}
	}
	if len(a.pre) == 0 && len(b.pre) > 0 {
		return 1, nil
	}
	if len(a.pre) > 0 && len(b.pre) == 0 {
		return -1, nil
	}
	for i := 0; i < len(a.pre) && i < len(b.pre); i++ {
		if a.pre[i] == b.pre[i] {
			continue
		}
		an, ae := strconv.ParseUint(a.pre[i], 10, 64)
		bn, be := strconv.ParseUint(b.pre[i], 10, 64)
		if ae == nil && be == nil {
			if an < bn {
				return -1, nil
			}
			return 1, nil
		}
		if ae == nil {
			return -1, nil
		}
		if be == nil {
			return 1, nil
		}
		if a.pre[i] < b.pre[i] {
			return -1, nil
		}
		return 1, nil
	}
	if len(a.pre) < len(b.pre) {
		return -1, nil
	}
	if len(a.pre) > len(b.pre) {
		return 1, nil
	}
	return 0, nil
}
