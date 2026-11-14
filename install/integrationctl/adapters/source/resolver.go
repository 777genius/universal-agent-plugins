package source

import (
	"context"
	"os"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/packagesnapshot"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type Resolver struct {
	Runner          ports.ProcessRunner
	SnapshotBuilder packagesnapshot.Builder
	DisableAliases  bool
}

func (r Resolver) Resolve(ctx context.Context, ref domain.IntegrationRef) (ports.ResolvedSource, error) {
	raw := strings.TrimSpace(ref.Raw)
	if raw == "" {
		return ports.ResolvedSource{}, domain.NewError(domain.ErrUsage, "source is required", nil)
	}
	if p, ok := resolveLocal(raw); ok {
		snapshot, err := r.SnapshotBuilder.Build(ctx, p)
		if err != nil {
			return ports.ResolvedSource{}, domain.NewError(domain.ErrSourceResolve, "snapshot local source", err)
		}
		return ports.ResolvedSource{
			Kind:            "local_path",
			Requested:       domain.RequestedSourceRef{Kind: "local_path", Value: p},
			Resolved:        domain.ResolvedSourceRef{Kind: "local_path", Value: p},
			LocalPath:       snapshot.Root,
			CanonicalSource: p,
			Cleanup:         snapshot.Close,
			SourceDigest:    snapshot.Digest,
			ExecutableFiles: append([]string(nil), snapshot.ExecutableFiles...),
			ImportRoots:     []string{snapshot.Root},
		}, nil
	}
	if !r.DisableAliases {
		if resolvedAlias, ok := resolveFirstPartySourceAlias(raw); ok {
			raw = resolvedAlias
		}
	}
	if ownerRepo, gitRef, subdir, ok := parseGitHubRef(raw); ok {
		cloned, err := r.cloneGitHub(ctx, ownerRepo, subdir, gitRef)
		if err != nil {
			return ports.ResolvedSource{}, domain.NewError(domain.ErrSourceResolve, "resolve github source", err)
		}
		snapshot, err := r.SnapshotBuilder.Build(ctx, cloned.PackageRoot)
		if err != nil {
			_ = os.RemoveAll(cloned.CleanupRoot)
			return ports.ResolvedSource{}, domain.NewError(domain.ErrSourceResolve, "snapshot github source", err)
		}
		_ = cloned.cleanup()
		return ports.ResolvedSource{
			Kind:             "github_repo_path",
			Requested:        domain.RequestedSourceRef{Kind: "github_repo_path", Value: raw},
			Resolved:         domain.ResolvedSourceRef{Kind: "git_commit", Value: "https://github.com/" + ownerRepo + "@" + cloned.Commit},
			LocalPath:        snapshot.Root,
			CanonicalSource:  canonicalGitHubSource(ownerRepo, subdir),
			Repository:       ownerRepo,
			PackageSubpath:   subdir,
			ResolvedRevision: cloned.Commit,
			Cleanup:          snapshot.Close,
			SourceDigest:     snapshot.Digest,
			ExecutableFiles:  append([]string(nil), snapshot.ExecutableFiles...),
			ImportRoots:      []string{snapshot.Root},
		}, nil
	}
	if repoURL, gitRef, ok := parseGitURLRef(raw); ok {
		cloned, err := r.cloneURL(ctx, repoURL, gitRef)
		if err != nil {
			return ports.ResolvedSource{}, domain.NewError(domain.ErrSourceResolve, "resolve git url", err)
		}
		snapshot, err := r.SnapshotBuilder.Build(ctx, cloned.PackageRoot)
		if err != nil {
			_ = os.RemoveAll(cloned.CleanupRoot)
			return ports.ResolvedSource{}, domain.NewError(domain.ErrSourceResolve, "snapshot git url source", err)
		}
		_ = cloned.cleanup()
		return ports.ResolvedSource{
			Kind:             "git_url",
			Requested:        domain.RequestedSourceRef{Kind: "git_url", Value: raw},
			Resolved:         domain.ResolvedSourceRef{Kind: "git_commit", Value: raw + "@" + cloned.Commit},
			LocalPath:        snapshot.Root,
			CanonicalSource:  repoURL,
			ResolvedRevision: cloned.Commit,
			Cleanup:          snapshot.Close,
			SourceDigest:     snapshot.Digest,
			ExecutableFiles:  append([]string(nil), snapshot.ExecutableFiles...),
			ImportRoots:      []string{snapshot.Root},
		}, nil
	}
	return ports.ResolvedSource{}, domain.NewError(domain.ErrSourceResolve, "unsupported source format", nil)
}

func canonicalGitHubSource(repository, subdir string) string {
	value := "https://github.com/" + repository
	if subdir != "" {
		value += "//" + subdir
	}
	return value
}
