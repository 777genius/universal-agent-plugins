package codex

import (
	"maps"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

var _ clients.ExplicitSkillDecorator = (*Adapter)(nil)

// explicitSkillPolicy is the skill metadata Codex reads for invocation policy.
// With allow_implicit_invocation false Codex never picks the skill itself, and
// an explicit $skill still runs it.
const (
	explicitSkillPolicyPath = "agents/openai.yaml"
	explicitSkillPolicy     = "policy:\n  allow_implicit_invocation: false\n"
)

// DecorateExplicitSkill writes the Codex policy file. The shared header field
// is kept as well: Codex ignores fields it does not know, and the stager
// requires it on every explicit skill.
func (*Adapter) DecorateExplicitSkill(skill domain.RenderedSkill) (domain.RenderedSkill, error) {
	skill = shared.WithExplicitInvocationField(skill)
	skill.Files = maps.Clone(skill.Files)
	if skill.Files == nil {
		skill.Files = map[string][]byte{}
	}
	skill.Files[explicitSkillPolicyPath] = []byte(explicitSkillPolicy)
	return skill, nil
}
