package domain

import "fmt"

// DirectoryResolvePurpose distinguishes normal eligibility from bounded source
// preparation and exact lock restoration. The zero value retains normal
// resolution; callers must explicitly opt in after validating intent.
type DirectoryResolvePurpose string

// DirectoryResolveContext7ChatGPTPreparation permits only source acquisition
// for Context7 ChatGPT user-scope local preparation, pending mapping validation.
// Its selection is not signed ChatGPT compatibility or installation authority.
const DirectoryResolveContext7ChatGPTPreparation DirectoryResolvePurpose = "context7_chatgpt_preparation"

func validateDirectoryResolvePurpose(snapshot DirectorySnapshot, request DirectoryResolveRequest) error {
	switch request.Purpose {
	case "":
		return nil
	case DirectoryResolveLockedRestore:
		// V1 is user-only. A verified V2 policy may restore exact project pins,
		// but must still pass all current target and revocation checks.
		validScope := request.Scope == ScopeUser || (request.Scope == ScopeProject && snapshot.SnapshotSchemaVersion == 2)
		if !validScope || request.DesiredReleaseSequence == 0 || request.Recorded != nil || len(request.Targets) == 0 ||
			(request.Operation != "" && request.Operation != DirectoryInstall) {
			return fmt.Errorf("%w: locked restore requires an exact authorized-scope new install without recorded state", ErrDirectoryIneligible)
		}
		return nil
	case DirectoryResolveContext7ChatGPTPreparation:
		if request.Scope != ScopeUser || len(request.Targets) != 1 || !declaresDirectoryPreparation(request.Targets[0], request.Purpose) {
			return fmt.Errorf("%w: invalid Context7 ChatGPT user-scope preparation purpose", ErrDirectoryIneligible)
		}
		return nil
	default:
		return fmt.Errorf("%w: unknown Directory resolve purpose %q", ErrDirectoryIneligible, request.Purpose)
	}
}

// declaresDirectoryPreparation reports whether this client is the one the
// bounded purpose was declared for. An unknown purpose matches no client, which
// is how a purpose nobody declared is rejected.
func declaresDirectoryPreparation(id ClientID, purpose DirectoryResolvePurpose) bool {
	definition, ok := ClientDefinitionFor(id)
	return ok && definition.DirectoryPreparationPurpose == purpose
}

func isContext7PreparationSource(distribution DirectoryDistribution, release DirectoryRelease) bool {
	return distribution.ID == "upstash/context7" && distribution.ProductID == "context7" &&
		distribution.Kind == DistributionUpstream && release.ManifestName == "context7" &&
		release.PackageSource.Repository == "upstash/context7" && release.PackageSource.Path == "plugins/agent-plugins/context7"
}

func hasContext7PreparationPromotion(snapshot DirectorySnapshot, distribution DirectoryDistribution, release DirectoryRelease, policy DirectoryReleasePolicy) bool {
	// Source-only preparation is never allowed when a matching ChatGPT-purpose
	// target is already declared, including one in another installation scope.
	for _, target := range policy.Targets {
		if declaresDirectoryPreparation(target.Client, DirectoryResolveContext7ChatGPTPreparation) {
			return false
		}
	}
	for _, target := range policy.Targets {
		delivery, supported := ExpectedDirectoryDelivery(target.Client)
		// Schema-2 grants carry one explicit scope; schema-1 carries the
		// historical scopes list. Do not infer a grant from the other wire.
		grants := func(scope InstallScope) bool {
			if snapshot.SnapshotSchemaVersion == 2 {
				return target.Scope == scope && len(target.Scopes) == 0
			}
			return target.Scope == "" && containsScope(target.Scopes, scope)
		}
		passed := grants(ScopeUser) && hasPassedUpstreamMaterialization(snapshot, distribution, release, policy, target.Client, ScopeUser) ||
			grants(ScopeProject) && hasPassedUpstreamMaterialization(snapshot, distribution, release, policy, target.Client, ScopeProject)
		if supported && target.Delivery == delivery && passed {
			return true
		}
	}
	return false
}
