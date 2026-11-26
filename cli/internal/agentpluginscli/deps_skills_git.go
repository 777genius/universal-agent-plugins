package agentpluginscli

import (
	"context"
	"errors"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/skillsource"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/sourcespec"
)

func (app App) acquireRawGitSkill(ctx context.Context, source environment.GitSource, commit, digest string) (domain.PackageSnapshot, error) {
	acquirer, err := app.rawSkillAcquirer()
	if err != nil {
		return domain.PackageSnapshot{}, err
	}
	return acquireRawGitSkill(ctx, acquirer, source, commit, digest)
}

func acquireRawGitSkill(ctx context.Context, acquirer skillRawAcquirer, source environment.GitSource, commit, digest string) (domain.PackageSnapshot, error) {
	if native, ok := acquirer.(interface {
		AcquireGitSkill(context.Context, sourcespec.Git, string, string) (domain.PackageSnapshot, error)
	}); ok {
		return native.AcquireGitSkill(ctx, gitSourceSpec(source), commit, digest)
	}
	if source.URL != "" {
		return domain.PackageSnapshot{}, errors.New("native raw Git Skill acquisition is unavailable")
	}
	if digest != "" {
		return acquirer.AcquireGitHubSkillVerified(ctx, source.Repository, commit, source.Path, digest)
	}
	return acquirer.AcquireGitHubSkill(ctx, source.Repository, commit, source.Path)
}

func (app App) resolveRawSkillPath(ctx context.Context, source environment.GitSource, commit, previous string) (string, error) {
	if finder, ok := app.SourceAcquirer.(interface {
		ResolveGitSkill(context.Context, skillsource.Source, string, string, string) (string, error)
	}); ok {
		return finder.ResolveGitSkill(ctx, skillsource.Source{Repository: source.Repository, URL: source.URL, Commit: commit}, source.Skill, source.Path, previous)
	}
	if source.URL == "" {
		if finder, ok := app.SourceAcquirer.(interface {
			ResolveGitHubSkill(context.Context, string, string, string, string, string) (string, error)
		}); ok {
			return finder.ResolveGitHubSkill(ctx, source.Repository, commit, source.Skill, source.Path, previous)
		}
	}
	acquirer, err := app.skillSourceAcquirer("")
	if err != nil {
		// Older embedders without discovery retain exact-path acquisition.
		if source.URL == "" {
			return source.Path, nil
		}
		return "", err
	}
	return acquirer.ResolveGitSkill(ctx, skillsource.Source{Repository: source.Repository, URL: source.URL, Commit: commit}, source.Skill, source.Path, previous)
}
