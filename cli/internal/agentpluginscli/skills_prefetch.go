package agentpluginscli

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packagedigest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/environment"
)

type rawSkillKey struct{ repository, url, commit, path string }

// rawSkillPrefetch moves the raw Skill acquisitions of one selection pass ahead
// of the sequential resolver.
type rawSkillPrefetch = orderedPrefetch[rawSkillKey, domain.PackageSnapshot]

func startRawSkillPrefetch(ctx context.Context, acquirer skillRawAcquirer, keys []rawSkillKey, guard prefetchGuard) *rawSkillPrefetch {
	acquire := func(ctx context.Context, key rawSkillKey) (domain.PackageSnapshot, error) {
		return acquireRawGitSkill(ctx, acquirer, environment.GitSource{Repository: key.repository, URL: key.url, Path: key.path}, key.commit, "")
	}
	release := func(snapshot domain.PackageSnapshot) { _ = packagedigest.Remove(snapshot) }
	return startOrderedPrefetch(ctx, keys, acquire, release, guard)
}
