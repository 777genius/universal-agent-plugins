package claude

import (
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

var _ clients.ExplicitSkillDecorator = (*Adapter)(nil)

// DecorateExplicitSkill relies on Claude Code's own skill field:
// disable-model-invocation: true keeps the skill out of the model's choice
// while /plugin:skill still runs it.
func (*Adapter) DecorateExplicitSkill(skill domain.RenderedSkill) (domain.RenderedSkill, error) {
	return shared.WithExplicitInvocationField(skill), nil
}
