package domain

import "fmt"

// DirectoryResolvePurpose distinguishes normal target eligibility from the
// bounded acquisition of source for local preparation. The zero value retains
// normal resolution; callers must explicitly opt in after validating intent.
type DirectoryResolvePurpose string

// DirectoryResolveContext7ChatGPTPreparation permits only source acquisition
// for Context7 ChatGPT user-scope local preparation, pending mapping validation.
// Its selection is not signed ChatGPT compatibility or installation authority.
const DirectoryResolveContext7ChatGPTPreparation DirectoryResolvePurpose = "context7_chatgpt_preparation"

func validateDirectoryResolvePurpose(request DirectoryResolveRequest) error {
	if request.Purpose == "" {
		return nil
	}
	if request.Purpose != DirectoryResolveContext7ChatGPTPreparation || request.Scope != ScopeUser ||
		len(request.Targets) != 1 || request.Targets[0] != ClientChatGPT {
		return fmt.Errorf("%w: invalid Context7 ChatGPT user-scope preparation purpose", ErrDirectoryIneligible)
	}
	return nil
}

func isContext7PreparationSource(distribution DirectoryDistribution, release DirectoryRelease) bool {
	return distribution.ID == "upstash/context7" && distribution.ProductID == "context7" &&
		distribution.Kind == DistributionUpstream && release.ManifestName == "context7" &&
		release.PackageSource.Repository == "upstash/context7" && release.PackageSource.Path == "plugins/agent-plugins/context7"
}

func hasContext7PreparationPromotion(snapshot DirectorySnapshot, distribution DirectoryDistribution, release DirectoryRelease, policy DirectoryReleasePolicy) bool {
	for _, target := range policy.Targets {
		delivery, supported := ExpectedDirectoryDelivery(target.Client)
		if supported && target.Delivery == delivery && (containsScope(target.Scopes, ScopeUser) || containsScope(target.Scopes, ScopeProject)) &&
			hasPassedUpstreamMaterialization(snapshot, distribution, release, policy, target.Client) {
			return true
		}
	}
	return false
}
