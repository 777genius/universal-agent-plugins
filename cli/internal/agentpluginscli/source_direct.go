package agentpluginscli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packagedigest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/sourceacquisition"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/sourcespec"
)

type directPackage struct {
	loaded      loadedPackage
	requirement environment.Requirement
	pin         environment.LockedPackage
}

func localArchiveInput(raw string) bool {
	spec, err := sourcespec.Parse(raw)
	if err != nil || spec.Kind != sourcespec.KindLocalArchive {
		return false
	}
	info, err := os.Stat(spec.Archive.URL)
	return err != nil || !info.IsDir()
}

func (app App) acquireDirectPackage(ctx context.Context, spec sourcespec.Spec) (directPackage, error) {
	direct, err := app.directAcquirer()
	if err != nil {
		return directPackage{}, err
	}
	if spec.Archive != nil {
		snapshot, archiveErr := direct.AcquireArchive(ctx, *spec.Archive, spec.Archive.SHA256, "")
		if archiveErr == nil {
			return app.loadDirectArchive(ctx, snapshot, *spec.Archive)
		}
		if spec.Kind != sourcespec.KindHTTPSAuto || !errors.Is(archiveErr, sourceacquisition.ErrNotArchive) {
			return directPackage{}, archiveErr
		}
		endpoint, repository, normalizeErr := sourcespec.NormalizeGitURL(spec.Archive.URL)
		if normalizeErr != nil {
			return directPackage{}, normalizeErr
		}
		spec.Git = &sourcespec.Git{URL: endpoint, Repository: repository}
	}
	if spec.Git == nil {
		return directPackage{}, errors.New("direct source has no acquisition coordinates")
	}
	return app.loadDirectGit(ctx, *spec.Git)
}

func (app App) loadDirectArchive(ctx context.Context, snapshot domain.PackageSnapshot, requested sourcespec.Archive) (directPackage, error) {
	loaded, err := app.loadStandardSnapshot(ctx, snapshot, domain.OriginModeDirect, nil, nil)
	if err != nil {
		return directPackage{}, err
	}
	source := loaded.envelope.Source
	requirement := environment.Requirement{Archive: &environment.ArchiveSource{URL: source.SourceURL, Path: source.PackageSubpath, SHA256: requested.SHA256}}
	pin := environment.LockedPackage{Source: environment.LockedSource{Archive: &environment.LockedArchiveSource{URL: source.SourceURL, Path: source.PackageSubpath, RequestedSHA256: requested.SHA256, SHA256: source.ArchiveSHA256}}}
	return directPackage{loaded: loaded, requirement: requirement, pin: pin}, nil
}

func (app App) loadDirectGit(ctx context.Context, source sourcespec.Git) (directPackage, error) {
	direct, err := app.directAcquirer()
	if err != nil {
		return directPackage{}, err
	}
	resolved, err := direct.ResolveGitSource(ctx, source)
	if err != nil {
		return directPackage{}, err
	}
	git := resolved.Git
	snapshot, err := app.acquireDirectGitRoot(ctx, git)
	if err != nil {
		return directPackage{}, err
	}
	git.Path = snapshot.Source.PackageSubpath
	selector := environment.Selector{Kind: environment.SelectorDefault}
	if git.Ref != "" {
		selector = environment.Selector{Kind: environment.SelectorRef, Value: git.Ref}
	}
	if source.Commit != "" || (source.Browser != nil && git.Ref == "") {
		selector = environment.Selector{Kind: environment.SelectorCommit, Value: git.Commit}
	}
	requirement := environment.Requirement{Git: &environment.GitSource{Repository: git.Repository, Path: git.Path, Selector: selector}}
	if git.Repository == "" {
		requirement.Git.URL = git.URL
	}
	pin := environment.LockedPackage{Source: environment.LockedSource{Git: &environment.LockedGitSource{Repository: requirement.Git.Repository, URL: requirement.Git.URL, Path: git.Path, Commit: git.Commit, Requested: selector, ResolvedRef: resolved.Resolution.ResolvedRef, DefaultBranch: resolved.Resolution.DefaultBranch}}}
	snapshot.Source = identityFromLocked(pin)
	loaded, err := app.loadStandardSnapshot(ctx, snapshot, domain.OriginModeDirect, nil, nil)
	if err != nil {
		return directPackage{}, err
	}
	return directPackage{loaded: loaded, requirement: requirement, pin: pin}, nil
}

func (app App) acquireDirectGitRoot(ctx context.Context, git sourcespec.Git) (domain.PackageSnapshot, error) {
	direct, err := app.directAcquirer()
	if err != nil {
		return domain.PackageSnapshot{}, err
	}
	snapshot, err := direct.AcquireGit(ctx, git, git.Commit, "")
	if err != nil {
		return domain.PackageSnapshot{}, err
	}
	if _, rootErr := os.Stat(filepath.Join(snapshot.Root, "plugin.json")); !os.IsNotExist(rootErr) || git.Path != "" {
		return snapshot, nil
	}
	_ = packagedigest.Remove(snapshot)
	paths, err := direct.DiscoverGitPackages(ctx, git, git.Commit)
	if err != nil {
		return domain.PackageSnapshot{}, err
	}
	if len(paths) != 1 {
		return domain.PackageSnapshot{}, fmt.Errorf("choose an explicit package path; standard package candidates: %v", paths)
	}
	git.Path = paths[0]
	return direct.AcquireGit(ctx, git, git.Commit, "")
}

func (app App) acquireDirectInput(ctx context.Context, spec sourcespec.Spec) (loadedPackage, error) {
	result, err := app.acquireDirectPackage(ctx, spec)
	if err != nil {
		return loadedPackage{}, err
	}
	result.loaded.envelope.Source.RequestedSource = spec.Requested
	app.warnDirectRevocation(result.loaded.envelope.TreeDigest)
	return result.loaded, nil
}
