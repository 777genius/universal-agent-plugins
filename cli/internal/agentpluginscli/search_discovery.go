package agentpluginscli

import (
	"sort"
	"strings"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/discoveryv1"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func discoverySearchResult(record discoveryv1.Record, query, owner string, components map[string]struct{}, client domain.ClientID, auth string) (searchResult, bool) {
	score, matched := discoveryMatchScore(query, record)
	if !matched || (strings.TrimSpace(owner) != "" && strings.ToLower(strings.TrimSpace(owner)) != record.Owner) {
		return searchResult{}, false
	}
	availableComponents := []string{}
	if record.Components.Extensions > 0 {
		availableComponents = append(availableComponents, "extensions")
	}
	if record.Components.MCP > 0 {
		availableComponents = append(availableComponents, "mcp")
	}
	if record.Components.Skills > 0 {
		availableComponents = append(availableComponents, "skills")
	}
	if !searchComponentsMatch(components, availableComponents) || (auth != "all" && auth != record.Authentication) {
		return searchResult{}, false
	}
	compatibility := make([]searchCompatibility, 0, len(record.CompatibleClients))
	for _, compatible := range record.CompatibleClients {
		clientID := domain.ClientID(compatible)
		if client != "" && clientID != client {
			continue
		}
		compatibility = append(compatibility, searchCompatibility{Client: clientID, Authentication: domain.AuthenticationRequirement(record.Authentication), Delivery: "portable-package"})
	}
	if client != "" && len(compatibility) == 0 {
		return searchResult{}, false
	}
	sort.Slice(compatibility, func(i, j int) bool { return compatibility[i].Client < compatibility[j].Client })
	version := ""
	if record.Version != nil {
		version = *record.Version
	}
	return searchResult{
		TrustState: "unreviewed", InstallSelector: record.Slug, ProductID: record.Name, DisplayName: record.Name,
		Description: record.Description, ManifestName: record.Name, Status: record.Availability, PackageVersion: version,
		Repository: record.Repository, Revision: record.Revision, PackagePath: record.PackagePath,
		SchemaURI: "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json", Components: availableComponents,
		Compatibility: compatibility, RuntimeReviewed: false, Stars: record.Stars, matchScore: score,
	}, true
}

func discoveryMatchScore(query string, record discoveryv1.Record) (int, bool) {
	needle := strings.ToLower(strings.TrimSpace(query))
	identities := []string{record.Slug, record.Name, record.Repository}
	if record.PackagePath != "" {
		identities = append(identities, record.Repository+"//"+record.PackagePath)
	}
	for _, identity := range identities {
		if strings.ToLower(identity) == needle {
			return 2, true
		}
	}
	for _, identity := range identities {
		if strings.HasPrefix(strings.ToLower(identity), needle) {
			return 3, true
		}
	}
	for _, identity := range identities {
		if strings.Contains(strings.ToLower(identity), needle) {
			return 4, true
		}
	}
	if strings.Contains(strings.ToLower(record.Description), needle) {
		return 5, true
	}
	return 0, false
}

func currentSearchRelease(snapshot domain.DirectorySnapshot, distribution domain.DirectoryDistribution) (*domain.DirectoryRelease, *domain.DirectoryReleasePolicy) {
	policies := make(map[uint64]domain.DirectoryReleasePolicy, len(distribution.ReleasePolicies))
	for _, policy := range distribution.ReleasePolicies {
		policies[policy.ReleaseSequence] = policy
	}
	releases := append([]domain.DirectoryRelease(nil), distribution.Releases...)
	sort.SliceStable(releases, func(i, j int) bool { return releases[i].Sequence > releases[j].Sequence })
	for index := range releases {
		policy, ok := policies[releases[index].Sequence]
		if !ok || policy.Status == domain.ReleaseRevoked || releaseSearchRevoked(snapshot, distribution.ID, releases[index].Sequence) {
			continue
		}
		return &releases[index], &policy
	}
	return nil, nil
}
