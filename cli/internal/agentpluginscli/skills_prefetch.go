package agentpluginscli

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/packagedigest"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

type rawSkillKey struct{ repository, commit, path string }

// rawSkillPrefetch moves the raw Skill acquisitions of one selection pass ahead
// of the sequential resolver.
type rawSkillPrefetch = orderedPrefetch[rawSkillKey, domain.PackageSnapshot]

func startRawSkillPrefetch(ctx context.Context, acquirer skillRawAcquirer, keys []rawSkillKey) *rawSkillPrefetch {
	acquire := func(ctx context.Context, key rawSkillKey) (domain.PackageSnapshot, error) {
		return acquirer.AcquireGitHubSkill(ctx, key.repository, key.commit, key.path)
	}
	release := func(snapshot domain.PackageSnapshot) { _ = packagedigest.Remove(snapshot) }
	return startOrderedPrefetch(ctx, keys, acquire, release)
}
