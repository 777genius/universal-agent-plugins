package domain

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// ResolveDirectory is deterministic and side-effect free. It selects one
// distribution and one release for the complete target set; acquisition
// failure is intentionally not a fallback input.
func ResolveDirectory(snapshot DirectorySnapshot, request DirectoryResolveRequest) (DirectorySelection, error) {
	if err := validateDirectoryResolvePurpose(snapshot, request); err != nil {
		return DirectorySelection{}, err
	}
	selector := strings.TrimSpace(request.Selector)
	if selector == "" {
		return DirectorySelection{}, fmt.Errorf("%w: empty selector", ErrDirectoryNotFound)
	}
	if request.Scope == "" {
		request.Scope = ScopeUser
	}
	if request.DesiredReleaseSequence != 0 && request.Recorded != nil {
		return DirectorySelection{}, fmt.Errorf("desired Directory release and recorded installed release are mutually exclusive")
	}
	if request.Operation == "" {
		request.Operation = DirectoryInstall
	}
	product, qualified, err := findDirectoryProduct(snapshot, selector)
	if err != nil {
		return DirectorySelection{}, err
	}
	if request.Purpose == DirectoryResolveContext7ChatGPTPreparation {
		if product.ID != "context7" || product.ManifestName != "context7" || (qualified != "" && qualified != "upstash/context7") ||
			(request.Recorded != nil && request.Recorded.DistributionID != "upstash/context7") {
			return DirectorySelection{}, fmt.Errorf("%w: Context7 preparation requires the upstream Context7 product", ErrDirectoryIneligible)
		}
		qualified = "upstash/context7"
	}
	if request.Recorded != nil {
		if strings.TrimSpace(request.Recorded.ResolvedRevision) == "" {
			return DirectorySelection{}, fmt.Errorf("recorded Directory release %s sequence %d has no resolved package-source revision", request.Recorded.DistributionID, request.Recorded.ReleaseSequence)
		}
		if strings.TrimSpace(request.Recorded.ProductID) == "" || strings.TrimSpace(request.Recorded.DistributionID) == "" || request.Recorded.ReleaseSequence < 1 {
			return DirectorySelection{}, fmt.Errorf("recorded Directory release identity is incomplete")
		}
		if request.Recorded.ProductID != product.ID {
			return DirectorySelection{}, fmt.Errorf("recorded product %q does not match %q", request.Recorded.ProductID, product.ID)
		}
		qualified = request.Recorded.DistributionID
	}
	// Release sequences are distribution-local. Resolve an exact short selector
	// before eligibility checks, so an ineligible publisher cannot be substituted.
	if request.DesiredReleaseSequence != 0 && qualified == "" {
		distributions := orderedDistributions(snapshot, product)
		if len(distributions) > 1 {
			return DirectorySelection{}, fmt.Errorf("%w: exact release for %q requires a qualified distribution ID", ErrDirectoryAmbiguous, selector)
		}
		if len(distributions) == 1 {
			qualified = distributions[0].ID
		}
	}
	if qualified != "" {
		distribution := distributionByID(snapshot, product, qualified)
		if distribution == nil {
			return DirectorySelection{}, fmt.Errorf("%w: distribution %q", ErrDirectoryNotFound, qualified)
		}
		if err := validateRecordedDirectoryRelease(*distribution, request.Recorded); err != nil {
			return DirectorySelection{}, err
		}
		release, reasons := chooseDirectoryRelease(snapshot, product, *distribution, request)
		if release == nil {
			return DirectorySelection{}, ineligibleError(distribution.ID, reasons)
		}
		return selectionFrom(snapshot, product, *distribution, *release, false, nil), nil
	}
	diagnostics := []DirectoryDiagnostic{}
	for index, distribution := range orderedDistributions(snapshot, product) {
		release, reasons := chooseDirectoryRelease(snapshot, product, distribution, request)
		if release != nil {
			return selectionFrom(snapshot, product, distribution, *release, index > 0, diagnostics), nil
		}
		for _, reason := range reasons {
			diagnostics = append(diagnostics, DirectoryDiagnostic{DistributionID: distribution.ID, Code: reason.code, Message: reason.message})
		}
	}
	return DirectorySelection{}, fmt.Errorf("%w for %q: %s", ErrDirectoryIneligible, selector, joinDiagnosticMessages(diagnostics))
}

func chooseDirectoryRelease(snapshot DirectorySnapshot, product DirectoryProduct, distribution DirectoryDistribution, request DirectoryResolveRequest) (*DirectoryRelease, []eligibilityReason) {
	policies := map[uint64]DirectoryReleasePolicy{}
	for _, p := range distribution.ReleasePolicies {
		policies[p.ReleaseSequence] = p
	}
	releases := append([]DirectoryRelease(nil), distribution.Releases...)
	sort.SliceStable(releases, func(i, j int) bool { return releases[i].Sequence > releases[j].Sequence })
	reasons := []eligibilityReason{}
	for i := range releases {
		release := &releases[i]
		if request.DesiredReleaseSequence != 0 && release.Sequence != request.DesiredReleaseSequence {
			continue
		}
		policy, ok := policies[release.Sequence]
		if !ok {
			continue
		}
		if request.Purpose == DirectoryResolveContext7ChatGPTPreparation && !isContext7PreparationSource(distribution, *release) {
			reasons = append(reasons, eligibilityReason{"invalid_preparation_source", "Context7 preparation requires the canonical upstream source"})
			continue
		}
		exactRecorded := request.Recorded != nil && release.Sequence == request.Recorded.ReleaseSequence
		// Every recorded operation except an explicit update stays on the exact
		// immutable release. In particular, re-adding a data-retained installation
		// is not an implicit update merely because a later release exists.
		if request.Recorded != nil && request.Operation != DirectoryUpdate && !exactRecorded {
			continue
		}
		if request.Operation == DirectoryUpdate && request.Recorded != nil && release.Sequence <= request.Recorded.ReleaseSequence {
			continue
		}
		if request.Purpose == DirectoryResolveLockedRestore && distribution.Status != DistributionActive {
			reasons = append(reasons, eligibilityReason{"distribution_not_active", fmt.Sprintf("distribution is %s for locked restore", distribution.Status)})
			continue
		}
		if distribution.Status == DistributionSuspended && request.Operation != DirectoryRemove && request.Operation != DirectoryRepair && request.Operation != DirectoryRematerialize && request.Operation != DirectoryReproduce &&
			!(exactRecorded && (request.Operation == DirectoryInstall || request.Operation == DirectoryNewTarget)) {
			reasons = append(reasons, eligibilityReason{"distribution_suspended", "distribution is suspended for this operation"})
			continue
		}
		if request.Operation == DirectoryRemove && exactRecorded {
			return release, nil
		}
		if isRevoked(snapshot, distribution.ID, release.Sequence) || policy.Status == ReleaseRevoked ||
			(request.Purpose == DirectoryResolveLockedRestore && isRevoked(snapshot, distribution.ID, 0)) {
			reasons = append(reasons, eligibilityReason{"release_revoked", fmt.Sprintf("release %d is revoked", release.Sequence)})
			continue
		}
		if policy.Status == ReleaseSuperseded && !exactRecorded && request.Purpose != DirectoryResolveLockedRestore {
			reasons = append(reasons, eligibilityReason{"release_superseded", fmt.Sprintf("release %d is historical only", release.Sequence)})
			continue
		}
		if policy.Status != ReleaseActive && policy.Status != ReleaseSuperseded {
			continue
		}
		if reason := releaseEligibility(snapshot, product, distribution, *release, policy, request); reason != nil {
			reasons = append(reasons, *reason)
			continue
		}
		return release, reasons
	}
	if request.Operation == DirectoryUpdate && request.Recorded != nil && len(reasons) == 0 {
		reasons = append(reasons, eligibilityReason{"no_safe_update", "no later eligible non-revoked release exists in the recorded distribution"})
	}
	return nil, reasons
}

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
	for _, target := range directoryEligibilityTargets(request.Targets, request.Scope) {
		entry := targetByClient(policy.Targets, target)
		// Only absence of signed ChatGPT support can use the source-only lane.
		if !(entry == nil && request.Purpose == DirectoryResolveContext7ChatGPTPreparation) {
			if entry == nil || !containsScope(entry.Scopes, request.Scope) {
				return &eligibilityReason{"incomplete_targets", "release does not support complete target set; missing " + string(target)}
			}
			delivery, supported := ExpectedDirectoryDelivery(target)
			if !supported || entry.Delivery != delivery {
				return &eligibilityReason{"incompatible_delivery", fmt.Sprintf("release delivery %q is incompatible with %s; expected %q", entry.Delivery, target, delivery)}
			}
			if distribution.Kind == DistributionUpstream && !hasPassedUpstreamMaterialization(snapshot, distribution, release, policy, target, request.Scope) {
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
	for i := 0; i < 3; i++ {
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
