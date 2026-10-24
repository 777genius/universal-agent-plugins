package agentpluginscli

import (
	"context"
	"errors"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/sourceacquisition"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/sourcespec"
)

func (app App) directAcquirer() (sourceacquisition.DirectAcquirer, error) {
	acquirer, ok := app.SourceAcquirer.(sourceacquisition.DirectAcquirer)
	if !ok {
		return nil, errors.New("direct Git and archive source acquisition is unavailable")
	}
	return acquirer, nil
}

func gitSourceSpec(source environment.GitSource) sourcespec.Git {
	return sourcespec.Git{URL: source.Endpoint(), Repository: source.Repository, Path: source.Path}
}

// Historical Repository pins keep the exact old identity, even when transport
// acquisition uses native credentials. Identity is selected by the lock, never
// by the transport implementation or the first writer of a shared store tree.
func legacyGitIdentity(repository, commit, packagePath, hint string) domain.SourceIdentity {
	identity := githubIdentity(repository, commit, packagePath)
	return domain.SourceIdentity{RequestedSource: identity, CanonicalSource: identity, Repository: repository,
		PackageSubpath: packagePath, ResolvedRevision: commit, SourceBindingHint: hint}
}

func identityFromLocked(pin environment.LockedPackage) domain.SourceIdentity {
	if source := pin.Source.Git; source != nil {
		if source.Kind == environment.SkillKind {
			return skillSnapshotSource(pin)
		}
		if source.Repository != "" {
			return legacyGitIdentity(source.Repository, source.Commit, source.Path, "uap-dependency-git")
		}
		return domain.SourceIdentity{SourceKind: "git", SourceURL: source.URL, RequestedSource: source.URL,
			CanonicalSource: sourcespec.Key(sourcespec.KindGit, source.URL, source.Path), PackageSubpath: source.Path,
			ResolvedRevision: source.Commit, SourceBindingHint: "direct-git"}
	}
	if source := pin.Source.Archive; source != nil {
		return domain.SourceIdentity{SourceKind: "archive", SourceURL: source.URL, ArchiveSHA256: source.SHA256,
			RequestedSource: source.URL, CanonicalSource: sourcespec.Key(sourcespec.KindArchive, source.URL, source.Path),
			PackageSubpath: source.Path, ResolvedRevision: "sha256:" + source.SHA256, SourceBindingHint: "direct-archive"}
	}
	if source := pin.Source.Registry; source != nil {
		return legacyGitIdentity(source.Repository, source.Commit, source.Path, "uap-dependency-registry")
	}
	return domain.SourceIdentity{}
}

func (app App) acquireDependencyGit(ctx context.Context, source environment.GitSource, commit, digest string) (domain.PackageSnapshot, error) {
	if direct, ok := app.SourceAcquirer.(sourceacquisition.DirectAcquirer); ok {
		return direct.AcquireGit(ctx, gitSourceSpec(source), commit, digest)
	}
	if source.URL != "" {
		return domain.PackageSnapshot{}, errors.New("git URL acquisition is unavailable")
	}
	if digest != "" {
		return app.SourceAcquirer.AcquireGitHubVerified(ctx, source.Repository, commit, source.Path, digest)
	}
	return app.SourceAcquirer.AcquireGitHub(ctx, source.Repository, commit, source.Path)
}

func (app App) acquirePinnedSource(ctx context.Context, pin environment.LockedPackage) (domain.PackageSnapshot, error) {
	if source := pin.Source.Git; source != nil {
		if source.Kind == environment.SkillKind {
			return app.acquireLockedSkill(ctx, pin)
		}
		return app.acquireDependencyGit(ctx, environment.GitSource{Repository: source.Repository, URL: source.URL, Path: source.Path}, source.Commit, pin.TreeDigest)
	}
	if source := pin.Source.Archive; source != nil {
		direct, err := app.directAcquirer()
		if err != nil {
			return domain.PackageSnapshot{}, err
		}
		return direct.AcquireArchive(ctx, sourcespec.Archive{URL: source.URL, Path: source.Path}, source.SHA256, pin.TreeDigest)
	}
	if source := pin.Source.Registry; source != nil {
		// Signed Directory remains on its anonymous, immutable GitHub contract.
		return app.SourceAcquirer.AcquireGitHubVerified(ctx, source.Repository, source.Commit, source.Path, pin.TreeDigest)
	}
	return domain.PackageSnapshot{}, errors.New("locked package has no source")
}

func (app App) resolveDependencyGit(ctx context.Context, source environment.GitSource) (sourceacquisition.RefResolution, error) {
	if direct, ok := app.SourceAcquirer.(sourceacquisition.DirectAcquirer); ok {
		selector := source.Selector.Value
		if source.Selector.Kind == environment.SelectorDefault {
			selector = "default"
		}
		return direct.ResolveGit(ctx, gitSourceSpec(source), selector, source.Selector.Kind != environment.SelectorDefault)
	}
	named, ok := app.SourceAcquirer.(interface {
		ResolveGitHubSelector(context.Context, string, string) (sourceacquisition.RefResolution, error)
		ResolveGitHubRef(context.Context, string, string) (sourceacquisition.RefResolution, error)
	})
	if !ok || source.URL != "" {
		return sourceacquisition.RefResolution{}, errors.New("source acquirer cannot resolve Git refs")
	}
	if source.Selector.Kind == environment.SelectorDefault {
		return named.ResolveGitHubSelector(ctx, source.Repository, "default")
	}
	return named.ResolveGitHubRef(ctx, source.Repository, source.Selector.Value)
}
