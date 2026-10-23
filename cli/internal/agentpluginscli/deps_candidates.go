package agentpluginscli

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

func dependencyDirectoryRequest(app App, distribution string, sequence uint64, targets []string) domain.DirectoryResolveRequest {
	host := directoryEnvironment(nil)
	request := domain.DirectoryResolveRequest{Selector: distribution, DesiredReleaseSequence: sequence, Scope: domain.ScopeUser, Operation: domain.DirectoryInstall, SchemaVersion: "1.0.0", InstallerVersion: app.Version, OS: host.OS, Architecture: host.Architecture}
	for _, target := range targets {
		request.Targets = append(request.Targets, domain.ClientID(target))
	}
	return request
}

func (resolver dependencyResolver) Candidates(ctx context.Context, requirement environment.Requirement, targets []string) ([]environment.Candidate, error) {
	if resolver.app.DirectoryClient == nil {
		return nil, errors.New("signed Directory client is required")
	}
	bundle, err := resolver.app.DirectoryClient.Load(ctx, 0)
	if err != nil {
		return nil, err
	}
	candidates := []environment.Candidate{}
	for _, distribution := range bundle.Snapshot.Distributions {
		if !strings.EqualFold(distribution.ID, requirement.Registry.Distribution) {
			continue
		}
		for _, release := range distribution.Releases {
			if !environment.SelectorMatchesVersion(requirement.Registry.Selector, release.PackageVersion) {
				continue
			}
			_, err := domain.ResolveDirectory(bundle.Snapshot, dependencyDirectoryRequest(resolver.app, distribution.ID, release.Sequence, targets))
			if errors.Is(err, domain.ErrDirectoryIneligible) {
				continue
			}
			if err != nil {
				return nil, err
			}
			candidates = append(candidates, environment.Candidate{Version: release.PackageVersion, ReleaseSequence: release.Sequence})
		}
	}
	return candidates, nil
}

func (resolver dependencyResolver) Eligible(ctx context.Context, pkg environment.LockedPackage, targets []string) (bool, error) {
	if pkg.Source.Registry == nil {
		return true, nil
	}
	// Once bytes are known, late target unions must retain their executable
	// identity as well as the host dimensions used during candidate filtering.
	pkg.Targets = append([]string{}, targets...)
	loaded, err := materializeLockedPackage(ctx, resolver.app, pkg, false, nil)
	if loaded.cleanup != nil {
		defer func() { _ = loaded.cleanup() }()
	}
	if errors.Is(err, domain.ErrDirectoryIneligible) {
		return false, nil
	}
	return err == nil, err
}

func (resolver dependencyResolver) AcquireCandidate(ctx context.Context, requirement environment.Requirement, candidate environment.Candidate, targets []string) (environment.ResolvedPackage, error) {
	request := packageResolutionRequest{StandardOnly: true, Operation: domain.DirectoryInstall, DesiredReleaseSequence: candidate.ReleaseSequence}
	for _, target := range targets {
		request.Targets = append(request.Targets, domain.ClientID(target))
	}
	loaded, err := resolver.app.acquireDirectory(ctx, requirement.Registry.Distribution, request)
	if err != nil {
		if errors.Is(err, domain.ErrDirectoryIneligible) {
			// Immutable MCP metadata can establish ineligibility only after fetch.
			return environment.ResolvedPackage{}, fmt.Errorf("%w: %w", environment.ErrCandidateIneligible, err)
		}
		return environment.ResolvedPackage{}, err
	}
	if loaded.envelope.Manifest.Version != candidate.Version {
		_ = loaded.cleanup()
		return environment.ResolvedPackage{}, fmt.Errorf("directory package version %s differs from manifest version %s", candidate.Version, loaded.envelope.Manifest.Version)
	}
	pkg, err := registryLockedPackage(requirement, loaded)
	if err != nil {
		_ = loaded.cleanup()
		return environment.ResolvedPackage{}, err
	}
	pkg.Alias = dependencyManifestAlias(loaded.envelope.Manifest.Name)
	facts, err := packageDependencyFacts(loaded.envelope)
	if err != nil {
		_ = loaded.cleanup()
		return environment.ResolvedPackage{}, err
	}
	result := environment.ResolvedPackage{Package: pkg, Dependencies: facts}
	return result, resolver.finishLoaded(loaded, pkg)
}

func dependencyManifestAlias(name string) string {
	alias := strings.ReplaceAll(strings.ToLower(name), ".", "-")
	if alias == "" || alias[0] < 'a' || alias[0] > 'z' {
		alias = "plugin-" + alias
	}
	if len(alias) > 64 {
		alias = alias[:64]
	}
	return alias
}

func (resolver dependencyResolver) DescribePinned(ctx context.Context, pin environment.LockedPackage) (environment.ResolvedPackage, error) {
	return readReplayDependency(ctx, resolver.app, pin)
}

// Read pinned source facts locally, without checking eligibility for a new install.
func describePinnedDependency(ctx context.Context, app App, pin environment.LockedPackage) (environment.ResolvedPackage, error) {
	pkg, err := app.dependencyStore().Get(ctx, pin.DigestAlgorithm, pin.TreeDigest)
	if err != nil {
		return environment.ResolvedPackage{}, err
	}
	input := domain.LoadInput{SnapshotRoot: pkg.Root, TreeDigest: pkg.Metadata.TreeDigest, ExecutableFiles: append([]string{}, pkg.Metadata.ExecutableFiles...), Source: identityFromLocked(pin)}
	envelope, err := app.loadStandardEnvelope(ctx, input)
	if err != nil {
		return environment.ResolvedPackage{}, err
	}
	if envelope.LoaderKind != domain.LoaderKindAgentPlugins || envelope.TreeDigest != pin.TreeDigest || envelope.ManifestDigest != pin.ManifestDigest {
		return environment.ResolvedPackage{}, errors.New("stored dependency failed strict standard-loader identity checks")
	}
	if pin.Version != "" && pin.Version != envelope.Manifest.Version {
		return environment.ResolvedPackage{}, errors.New("stored dependency version differs from uap.lock")
	}
	pin.Version = envelope.Manifest.Version
	facts, err := packageDependencyFacts(envelope)
	if err != nil {
		return environment.ResolvedPackage{}, err
	}
	return environment.ResolvedPackage{Package: pin, Dependencies: facts}, nil
}
