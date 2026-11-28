package agentpluginscli

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/sourcespec"
)

// Check original selectors before resolution can replace them with immutable
// coordinates. The full envelope is checked again before any installation.
func (app App) checkEnterpriseRequirement(ctx context.Context, req environment.Requirement, targets []string) error {
	if app.InstallationPolicy == nil || req.Registry != nil {
		return nil
	}
	request := domain.InstallPolicyRequest{OriginMode: domain.OriginModeDirect, Scope: app.scope()}
	for _, target := range targets {
		request.Targets = append(request.Targets, domain.ClientID(target))
	}
	switch {
	case req.Git != nil:
		request.Source = domain.SourceIdentity{SourceKind: "git", SourceURL: req.Git.URL, Repository: req.Git.Repository, PackageSubpath: req.Git.Path}
		pinned := req.Git.Selector.Kind == environment.SelectorCommit
		request.GitPinned = &pinned
	case req.Archive != nil:
		request.Source = domain.SourceIdentity{SourceKind: "archive", SourceURL: req.Archive.URL, PackageSubpath: req.Archive.Path}
		pinned := sourcespec.ValidSHA256(req.Archive.SHA256)
		request.ArchivePinned = &pinned
	case req.Local != nil:
		request.Source = domain.SourceIdentity{SourceKind: "local", SourceURL: req.Local.Path}
	case req.Web != nil:
		request.Source = domain.SourceIdentity{SourceKind: "web", SourceURL: req.Web.URL}
	}
	_, err := app.checkEnterprisePolicy(ctx, request)
	return err
}

func (app App) checkEnterpriseLocal(ctx context.Context, requested string) error {
	path, err := filepath.Abs(requested)
	if err != nil {
		return err
	}
	return app.checkEnterpriseSource(ctx, domain.SourceIdentity{SourceKind: "local", SourceURL: path, RequestedSource: requested}, domain.OriginModeDirect)
}

func (app App) checkEnterpriseGit(ctx context.Context, source sourcespec.Git) error {
	return app.checkEnterpriseGitTargets(ctx, source, nil)
}

func (app App) checkEnterpriseGitTargets(ctx context.Context, source sourcespec.Git, targets []string) error {
	selector := environment.Selector{Kind: environment.SelectorDefault}
	if commit := requestedPolicyGitCommit(source); commit != "" {
		selector = environment.Selector{Kind: environment.SelectorCommit, Value: commit}
	} else if source.Ref != "" {
		selector = environment.Selector{Kind: environment.SelectorRef, Value: source.Ref}
	}
	return app.checkEnterpriseRequirement(ctx, environment.Requirement{Git: &environment.GitSource{
		URL: source.URL, Repository: source.Repository, Path: source.Path, Selector: selector,
	}}, targets)
}

// Browser selectors remain unresolved in sourcespec, but a full commit in their
// first segment is already exact intent. Match the resolver without consulting
// a resolved revision or guessing a branch/path split.
func requestedPolicyGitCommit(source sourcespec.Git) string {
	if source.Commit != "" {
		return source.Commit
	}
	if source.Ref != "" || source.Browser == nil {
		return ""
	}
	commit, _, _ := strings.Cut(source.Browser.Suffix, "/")
	if sourcespec.ValidCommit(commit) {
		return commit
	}
	return ""
}

func (app App) checkEnterpriseArchive(ctx context.Context, source sourcespec.Archive) error {
	if !strings.Contains(source.URL, "://") {
		absolute, err := filepath.Abs(source.URL)
		if err != nil {
			return err
		}
		source.URL = absolute
	}
	return app.checkEnterpriseRequirement(ctx, environment.Requirement{Archive: &environment.ArchiveSource{
		URL: source.URL, Path: source.Path, SHA256: source.SHA256,
	}}, nil)
}

func (app App) acquirePolicyDirectorySnapshot(ctx context.Context, selection domain.DirectorySelection) (domain.PackageSnapshot, error) {
	source := domain.SourceIdentity{Repository: selection.Source.Repository, ResolvedRevision: selection.Source.Revision, PackageSubpath: selection.Source.Path}
	if err := app.checkEnterpriseSource(ctx, source, domain.OriginModeDirectory); err != nil {
		return domain.PackageSnapshot{}, err
	}
	return app.SourceAcquirer.AcquireGitHubVerified(ctx, selection.Source.Repository, selection.Source.Revision, selection.Source.Path, selection.TreeDigest)
}

// Locked replay may fetch missing bytes while verifying declarations. Admit
// every known source first, then let the existing verifier retain ownership of
// graph integrity. Safe graph reduction uses its read-only verifier directly.
func verifyInstallationDependencyDeclarations(ctx context.Context, app App, lock environment.Lock) error {
	for _, pin := range lock.Packages {
		if _, err := app.checkEnterprisePolicy(ctx, lockedPolicyRequest(pin, app.scope())); err != nil {
			return err
		}
	}
	return verifyDependencyDeclarations(ctx, app, lock)
}
