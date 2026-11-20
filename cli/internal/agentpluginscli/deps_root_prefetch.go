package agentpluginscli

import (
	"context"
	"sort"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packagedigest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/sourceacquisition"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

// gitRootFetch is the ref decision and raw tree that resolveGit obtains for
// one fresh Git root, plugin or Skill.
type gitRootFetch struct {
	resolution sourceacquisition.RefResolution
	snapshot   domain.PackageSnapshot
}

// gitRootPrefetch moves the independent remote reads of fresh Git roots ahead
// of the sequential graph solver. Loading, scanning, store writes and every
// graph decision stay with the solver.
type gitRootPrefetch = orderedPrefetch[environment.GitSource, gitRootFetch]

// resolveLock resolves the declared graph with the reads of fresh Git roots
// started ahead of the sequential solver.
func (resolver dependencyResolver) resolveLock(ctx context.Context, previous environment.Lock, updateAlias string) (environment.Lock, error) {
	resolver.gitRoots = resolver.prefetchGitRoots(ctx, previous)
	defer resolver.gitRoots.close()
	return (environment.Coordinator{EnvironmentID: resolver.app.environmentID(), Resolver: resolver}).Resolve(ctx, resolver.requirements, previous, updateAlias)
}

// prefetchGitRoots starts the ref resolution and raw acquisition that
// resolveGit performs for each declared Git root without a previous pin, in the
// solver's source key order. Roots that resolveGit would reject before
// downloading, or would answer from the previous lock, are not fetched.
func (resolver dependencyResolver) prefetchGitRoots(ctx context.Context, previous environment.Lock) *gitRootPrefetch {
	pinned := map[string]bool{}
	for _, pkg := range previous.Packages {
		pinned[pkg.SourceKey] = true
	}
	skills, skillsErr := resolver.app.rawSkillAcquirer()
	type root struct {
		key    string
		source environment.GitSource
	}
	roots := []root{}
	for _, requirement := range resolver.requirements.Dependencies {
		source := requirement.Git
		if source == nil {
			continue
		}
		key, err := requirement.SourceKey()
		if err != nil || pinned[key] {
			continue
		}
		if source.Kind == environment.SkillKind && (skillsErr != nil || skillDirectoryMismatch(source.Skill, source.Path)) {
			continue
		}
		roots = append(roots, root{key: key, source: *source})
	}
	if len(roots) < 2 {
		return nil
	}
	sort.Slice(roots, func(i, j int) bool { return roots[i].key < roots[j].key })
	keys := make([]environment.GitSource, 0, len(roots))
	for _, root := range roots {
		keys = append(keys, root.source)
	}
	acquire := func(ctx context.Context, source environment.GitSource) (gitRootFetch, error) {
		resolution, err := resolver.resolveGitSelector(ctx, &source)
		if err != nil {
			return gitRootFetch{}, err
		}
		// An earlier key's failure cancels this one; do not start its tree.
		if err := ctx.Err(); err != nil {
			return gitRootFetch{}, err
		}
		var snapshot domain.PackageSnapshot
		if source.Kind == environment.SkillKind {
			snapshot, err = skills.AcquireGitHubSkill(ctx, source.Repository, resolution.Commit, source.Path)
		} else {
			snapshot, err = resolver.app.acquireDependencyGit(ctx, source, resolution.Commit, "")
		}
		return gitRootFetch{resolution: resolution, snapshot: snapshot}, err
	}
	release := func(fetch gitRootFetch) { _ = packagedigest.Remove(fetch.snapshot) }
	return startOrderedPrefetch(ctx, keys, acquire, release, resolver.app.prefetchGuard())
}
