package domain

import "testing"

func TestClaudeProjectNativeRegistrationKeepsUserPolicy(t *testing.T) {
	user := (SelectedDelivery{}).EffectiveTraitsForScope(ClientClaude, ScopeUser)
	project := (SelectedDelivery{}).EffectiveTraitsForScope(ClientClaude, ScopeProject)
	if user.TracksNativeEffects || user.RequiresActiveNativeProjection {
		t.Fatalf("project registration changed user policy: %+v", user)
	}
	if !project.TracksNativeEffects || !project.RequiresActiveNativeProjection || project.LifecycleKind != LifecycleCLIRegistry {
		t.Fatalf("project registration lost native effect or projection ownership: %+v", project)
	}
	definition, _ := ClientDefinitionFor(ClientClaude)
	for _, scope := range []InstallScope{ScopeUser, ScopeProject} {
		capabilities := definition.ScopedCapabilities(scope)
		if capabilities.SkillSupport != SupportProjected || capabilities.MCPTransports["stdio"] != SupportProjected {
			t.Fatalf("native registration changed planned component support: %+v", capabilities)
		}
	}
}
