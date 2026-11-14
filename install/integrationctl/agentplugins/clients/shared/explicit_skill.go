package shared

import "github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"

// WithExplicitInvocationField returns skill with the explicit-invocation
// header field set to true appended, without sharing the caller's slices.
// Every explicit skill decorator starts from it.
func WithExplicitInvocationField(skill domain.RenderedSkill) domain.RenderedSkill {
	skill.Frontmatter = append(append([]domain.FrontmatterField(nil), skill.Frontmatter...), domain.FrontmatterField{Key: domain.ExplicitInvocationField, Value: true})
	return skill
}
