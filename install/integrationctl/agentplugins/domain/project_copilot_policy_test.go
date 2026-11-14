package domain

import "testing"

func TestProjectCopilotUsesSeparateProjectBackendAndPreservesUserPolicy(t *testing.T) {
	definition, ok := ClientDefinitionFor(ClientCopilot)
	if !ok || !definition.ProjectPolicy.Valid() || definition.CommandSupport != SupportUnsupported {
		t.Fatalf("Copilot project declaration changed command policy: %+v", definition)
	}
	if !SameClientBackendForScope(ClientCopilot, ClientVSCode, ScopeUser) || SameClientBackendForScope(ClientCopilot, ClientVSCode, ScopeProject) {
		t.Fatal("Copilot project borrowed the shared user editor backend")
	}
	delivery := SelectedDelivery{}
	if delivery.EffectiveTraitsForScope(ClientCopilot, ScopeUser).LifecycleKind != LifecycleCLIRegistry || delivery.EffectiveTraitsForScope(ClientCopilot, ScopeProject).LifecycleKind != LifecycleNativeConfig {
		t.Fatal("Copilot project/user lifecycle separation changed")
	}
	if delivery.SharesBackendForScope(ClientCopilot, ScopeProject) || BackendFamilyForScope(ClientCopilot, ScopeProject) != "copilot" {
		t.Fatal("Copilot repository components have an ambiguous physical owner")
	}
}
