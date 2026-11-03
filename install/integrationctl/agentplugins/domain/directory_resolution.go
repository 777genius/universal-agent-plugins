package domain

import (
	"fmt"
	"sort"
	"strings"
)

// ResolveDirectory is deterministic and side-effect free. It selects one
// distribution and one release for the complete target set; acquisition
// failure is intentionally not a fallback input.
func ResolveDirectory(snapshot DirectorySnapshot, request DirectoryResolveRequest) (DirectorySelection, error) {
	if err := validateDirectoryResolvePurpose(request); err != nil {
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

func validateRecordedDirectoryRelease(distribution DirectoryDistribution, recorded *RecordedDirectoryRelease) error {
	if recorded == nil {
		return nil
	}
	revision := strings.TrimSpace(recorded.ResolvedRevision)
	if revision == "" {
		return fmt.Errorf("recorded Directory release %s sequence %d has no resolved package-source revision", recorded.DistributionID, recorded.ReleaseSequence)
	}
	if !directoryEvidenceRevisionPattern.MatchString(revision) {
		return fmt.Errorf("recorded Directory release %s sequence %d has an invalid package-source revision; expected a full lowercase commit SHA", recorded.DistributionID, recorded.ReleaseSequence)
	}
	for _, release := range distribution.Releases {
		if release.Sequence != recorded.ReleaseSequence {
			continue
		}
		if release.PackageSource.Revision != revision {
			return fmt.Errorf("recorded Directory release %s sequence %d package-source revision does not match the current signed snapshot", recorded.DistributionID, recorded.ReleaseSequence)
		}
		checks := []struct {
			name     string
			recorded string
			current  string
		}{
			{name: "repository", recorded: recorded.Repository, current: release.PackageSource.Repository},
			{name: "path", recorded: recorded.Path, current: release.PackageSource.Path},
			{name: "tree digest algorithm", recorded: recorded.TreeDigestAlgorithm, current: release.TreeDigestAlgorithm},
			{name: "tree digest", recorded: recorded.TreeDigest, current: release.TreeDigest},
			{name: "manifest digest", recorded: recorded.ManifestDigest, current: release.ManifestDigest},
		}
		for _, check := range checks {
			if check.recorded != "" && check.recorded != check.current {
				return fmt.Errorf("recorded Directory release %s sequence %d package-source %s does not match the current signed snapshot", recorded.DistributionID, recorded.ReleaseSequence, check.name)
			}
		}
		return nil
	}
	return fmt.Errorf("%w: recorded release %s sequence %d", ErrDirectoryNotFound, recorded.DistributionID, recorded.ReleaseSequence)
}

type eligibilityReason struct{ code, message string }

func findDirectoryProduct(snapshot DirectorySnapshot, selector string) (DirectoryProduct, string, error) {
	if strings.Contains(selector, "/") {
		var match *DirectoryProduct
		for i := range snapshot.Products {
			if distributionByID(snapshot, snapshot.Products[i], selector) != nil {
				if match != nil {
					return DirectoryProduct{}, "", fmt.Errorf("%w: qualified distribution %q", ErrDirectoryAmbiguous, selector)
				}
				productCopy := snapshot.Products[i]
				match = &productCopy
			}
		}
		if match == nil {
			return DirectoryProduct{}, "", fmt.Errorf("%w: distribution %q", ErrDirectoryNotFound, selector)
		}
		return *match, selector, nil
	}
	matches := []DirectoryProduct{}
	for _, product := range snapshot.Products {
		if product.ID == selector || product.ManifestName == selector || containsString(product.Aliases, selector) {
			matches = append(matches, product)
		}
	}
	if len(matches) == 0 {
		return DirectoryProduct{}, "", fmt.Errorf("%w: %q", ErrDirectoryNotFound, selector)
	}
	if len(matches) != 1 {
		ids := make([]string, len(matches))
		for i := range matches {
			ids[i] = matches[i].ID
		}
		sort.Strings(ids)
		return DirectoryProduct{}, "", fmt.Errorf("%w: %q matches %s; use a qualified distribution ID", ErrDirectoryAmbiguous, selector, strings.Join(ids, ", "))
	}
	return matches[0], "", nil
}

func orderedDistributions(snapshot DirectorySnapshot, product DirectoryProduct) []DirectoryDistribution {
	result := []DirectoryDistribution{}
	seen := map[string]bool{}
	if item := distributionByID(snapshot, product, product.DefaultDistribution); item != nil {
		result = append(result, *item)
		seen[item.ID] = true
	}
	for _, kind := range []DistributionKind{DistributionUpstream, DistributionCommunityBridge, DistributionCommunity} {
		items := []DirectoryDistribution{}
		for _, id := range product.Distributions {
			if item := distributionByID(snapshot, product, id); item != nil && !seen[id] && item.Kind == kind {
				items = append(items, *item)
			}
		}
		sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
		for _, item := range items {
			result = append(result, item)
			seen[item.ID] = true
		}
	}
	return result
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
			(!exactRecorded || (request.Operation != DirectoryInstall && request.Operation != DirectoryNewTarget)) {
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
