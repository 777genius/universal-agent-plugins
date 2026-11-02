package main

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/skillsource"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// SkillsAcquirer preserves the configured Git runner/repository mapping while
// putting discovery and unaccepted review bytes in their caller-owned scratch.
// Merely constructing a CLI or querying help must not create AGENTPLUGINS_HOME.
func (acquirer lazySourceAcquirer) SkillsAcquirer(scratch string) skillsource.Acquirer {
	source := acquirer.acquirer
	if scratch != "" {
		source.TempRoot = scratch
	}
	return skillsource.Acquirer{Acquirer: source}
}

func (acquirer lazySourceAcquirer) ResolveGitHubSkill(ctx context.Context, repository, revision, expected, requested, previous string) (string, error) {
	if err := acquirer.prepare(); err != nil {
		return "", err
	}
	return acquirer.SkillsAcquirer("").ResolveGitHubSkill(ctx, repository, revision, expected, requested, previous)
}

// AcquireGitHubSkill keeps the lazy persistent boundary while requiring complete
// raw Skill resources, including rejection of unsupported root gitlinks.
func (acquirer lazySourceAcquirer) AcquireGitHubSkill(ctx context.Context, repository, revision, subpath string) (domain.PackageSnapshot, error) {
	if err := acquirer.prepare(); err != nil {
		return domain.PackageSnapshot{}, err
	}
	return acquirer.acquirer.AcquireGitHubSkill(ctx, repository, revision, subpath)
}

func (acquirer lazySourceAcquirer) AcquireGitHubSkillVerified(ctx context.Context, repository, revision, subpath, digest string) (domain.PackageSnapshot, error) {
	if err := acquirer.prepare(); err != nil {
		return domain.PackageSnapshot{}, err
	}
	return acquirer.acquirer.AcquireGitHubSkillVerified(ctx, repository, revision, subpath, digest)
}
