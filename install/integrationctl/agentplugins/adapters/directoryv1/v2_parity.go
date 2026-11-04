package directoryv1

import (
	"unicode/utf8"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// validateV2RecordParity keeps the signed v2 record contract no weaker than
// v1 for portable identities. It adds no new dependency resolver or policy.
func validateV2RecordParity(snapshot domain.DirectorySnapshot) error {
	if err := validateV2ProductSelectors(snapshot.Products); err != nil {
		return err
	}
	if err := validateV2ReleaseComponents(snapshot.Distributions); err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, item := range snapshot.Revocations {
		key := releaseKey(item.DistributionID, item.ReleaseSequence)
		if seen[key] || item.ReleaseSequence == 0 {
			return strictError("duplicate or invalid v2 revocation %q", key)
		}
		seen[key] = true
	}
	return nil
}

func validateV2ProductSelectors(products []domain.DirectoryProduct) error {
	owners := map[string]string{}
	for _, product := range products {
		if utf8.RuneCountInString(product.DisplayName) > 100 ||
			utf8.RuneCountInString(product.Description) > 500 ||
			len(product.Aliases) == 0 || len(product.ReservedAliases) == 0 ||
			len(product.Categories) == 0 || len(product.Categories) > 20 {
			return strictError("invalid v2 product display or selector arrays")
		}
		if err := uniqueSimpleIDs(product.ReservedAliases, product.ID+" reserved aliases"); err != nil {
			return err
		}
		if err := uniqueSimpleIDs(product.Categories, product.ID+" categories"); err != nil {
			return err
		}
		if product.Icon != nil && (!iconPathPattern.MatchString(product.Icon.Path) ||
			!digestPattern.MatchString(product.Icon.Digest)) {
			return strictError("invalid v2 product icon")
		}
		selectors := append([]string{product.ID, product.ManifestName}, product.Aliases...)
		selectors = append(selectors, product.ReservedAliases...)
		for _, selector := range selectors {
			if owner := owners[selector]; owner != "" && owner != product.ID {
				return strictError("v2 selector %q belongs to both %q and %q", selector, owner, product.ID)
			}
			owners[selector] = product.ID
		}
	}
	return nil
}

func validateV2ReleaseComponents(distributions []domain.DirectoryDistribution) error {
	for _, distribution := range distributions {
		if !simpleIDPattern.MatchString(distribution.Packager) ||
			(distribution.Kind != domain.DistributionUpstream &&
				distribution.Kind != domain.DistributionCommunityBridge &&
				distribution.Kind != domain.DistributionCommunity) {
			return strictError("invalid v2 distribution kind or packager")
		}
		for _, release := range distribution.Releases {
			if release.BuildProvenance != nil &&
				(!repositoryPattern.MatchString(release.BuildProvenance.UpstreamRepository) ||
					!shaPattern.MatchString(release.BuildProvenance.UpstreamRevision)) {
				return strictError("invalid v2 build provenance")
			}
			seen := map[string]bool{}
			for _, component := range release.Components {
				if seen[component] || component != "skills" && component != "mcp" && component != "extensions" {
					return strictError("invalid or duplicate v2 component %q", component)
				}
				seen[component] = true
			}
		}
	}
	return nil
}
