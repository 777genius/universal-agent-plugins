package domain

import (
	"fmt"
	"sort"
	"strings"
)

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
