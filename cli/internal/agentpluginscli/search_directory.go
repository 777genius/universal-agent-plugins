package agentpluginscli

import (
	"fmt"
	"sort"
	"strings"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func directorySearchResult(snapshot domain.DirectorySnapshot, product domain.DirectoryProduct, distribution domain.DirectoryDistribution, release domain.DirectoryRelease, policy domain.DirectoryReleasePolicy, query, owner string, components map[string]struct{}, client domain.ClientID, auth string) (searchResult, bool) {
	score, matched := searchMatchScore(query, product, distribution, release)
	if !matched || !searchOwnerMatches(owner, distribution, release) || !searchComponentsMatch(components, release.Components) {
		return searchResult{}, false
	}
	compatibility := make([]searchCompatibility, 0, len(policy.Targets))
	for _, target := range policy.Targets {
		if client != "" && target.Client != client {
			continue
		}
		if auth != "all" && string(target.Authentication) != auth {
			continue
		}
		compatibility = append(compatibility, searchCompatibility{Client: target.Client, Authentication: target.Authentication, Delivery: target.Delivery})
	}
	if (client != "" || auth != "all") && len(compatibility) == 0 {
		return searchResult{}, false
	}
	sort.Slice(compatibility, func(i, j int) bool { return compatibility[i].Client < compatibility[j].Client })
	selector := distribution.ID
	if distribution.ID == product.DefaultDistribution && distribution.Status == domain.DistributionActive {
		selector = product.ID
	}
	status := "available"
	if distribution.Status != domain.DistributionActive || policy.Status != domain.ReleaseActive {
		status = "unavailable"
	}
	return searchResult{
		TrustState: "reviewed", InstallSelector: selector, ProductID: product.ID, DisplayName: product.DisplayName,
		Description: product.Description, ManifestName: product.ManifestName, DistributionID: distribution.ID,
		DistributionKind: distribution.Kind, Status: status, PackageVersion: release.PackageVersion,
		Repository: release.PackageSource.Repository, Revision: release.PackageSource.Revision, PackagePath: release.PackageSource.Path,
		SchemaURI: release.AgentPluginsSchema, Components: append([]string(nil), release.Components...), Compatibility: compatibility,
		RuntimeReviewed: searchHasRuntimeEvidence(snapshot, distribution.ID, release.Sequence, policy, client), matchScore: score,
	}, true
}

func searchMatchScore(query string, product domain.DirectoryProduct, distribution domain.DirectoryDistribution, release domain.DirectoryRelease) (int, bool) {
	needle := strings.ToLower(strings.TrimSpace(query))
	identities := []string{product.ID, product.ManifestName, distribution.ID, release.ManifestName}
	identities = append(identities, product.Aliases...)
	for _, identity := range identities {
		if strings.ToLower(identity) == needle {
			return 0, true
		}
	}
	for _, identity := range identities {
		if strings.HasPrefix(strings.ToLower(identity), needle) {
			return 1, true
		}
	}
	for _, identity := range identities {
		if strings.Contains(strings.ToLower(identity), needle) {
			return 2, true
		}
	}
	if strings.Contains(strings.ToLower(product.DisplayName+" "+product.Description), needle) {
		return 3, true
	}
	return 0, false
}

func searchOwnerMatches(owner string, distribution domain.DirectoryDistribution, release domain.DirectoryRelease) bool {
	owner = strings.ToLower(strings.TrimSpace(owner))
	if owner == "" {
		return true
	}
	for _, value := range []string{distribution.ID, release.PackageSource.Repository} {
		parts := strings.SplitN(strings.ToLower(value), "/", 2)
		if len(parts) == 2 && parts[0] == owner {
			return true
		}
	}
	return false
}

func searchComponentsMatch(required map[string]struct{}, available []string) bool {
	if len(required) == 0 {
		return true
	}
	seen := map[string]struct{}{}
	for _, value := range available {
		seen[strings.ToLower(strings.TrimSpace(value))] = struct{}{}
	}
	for value := range required {
		if _, ok := seen[value]; !ok {
			return false
		}
	}
	return true
}

func parseSearchValues(value string) (map[string]struct{}, error) {
	result := map[string]struct{}{}
	if strings.TrimSpace(value) == "" {
		return result, nil
	}
	for _, item := range strings.Split(value, ",") {
		item = strings.ToLower(strings.TrimSpace(item))
		if item == "" {
			return nil, fmt.Errorf("empty value")
		}
		result[item] = struct{}{}
	}
	return result, nil
}

func releaseSearchRevoked(snapshot domain.DirectorySnapshot, distribution string, sequence uint64) bool {
	for _, revocation := range snapshot.Revocations {
		if revocation.DistributionID == distribution && revocation.ReleaseSequence == sequence {
			return true
		}
	}
	return false
}

func searchHasRuntimeEvidence(snapshot domain.DirectorySnapshot, distribution string, sequence uint64, policy domain.DirectoryReleasePolicy, client domain.ClientID) bool {
	current := make(map[string]struct{}, len(policy.CurrentEvidence))
	for _, id := range policy.CurrentEvidence {
		current[id] = struct{}{}
	}
	for _, evidence := range snapshot.Evidence {
		_, active := current[evidence.ID]
		if active && evidence.DistributionID == distribution && evidence.ReleaseSequence == sequence &&
			(client == "" || evidence.Client == client) &&
			evidence.Level == "runtime" && evidence.Outcome == "passed" && evidence.HasTrustedEligibilityProvenanceAtSequence(snapshot.Sequence) {
			return true
		}
	}
	return false
}
