package all_test

import (
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/all"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/vscode"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestProjectRegistrySelectsDistinctVSCodeBackend(t *testing.T) {
	user, ok := all.Default().Lookup(domain.ClientVSCode)
	if !ok {
		t.Fatal("user VS Code adapter missing")
	}
	if _, ok := user.(*vscode.Adapter); !ok {
		t.Fatal("default VS Code backend changed")
	}
	project, ok := all.Project().Lookup(domain.ClientVSCode)
	if !ok {
		t.Fatal("project VS Code adapter missing")
	}
	if _, ok := project.(*vscode.ProjectAdapter); !ok {
		t.Fatal("project registry reused user or Local adapter")
	}
	if clients.SupportsScope(all.Default(), domain.ClientVSCode, domain.ScopeProject) || !clients.SupportsScope(all.Project(), domain.ClientVSCode, domain.ScopeProject) {
		t.Fatal("static scope declaration bypassed explicit project composition")
	}
	inspector, ok := project.(clients.RegistryInspector)
	if !ok || inspector.UsesNativeRegistryExecutable() {
		t.Fatal("project registry requires a vendor/Copilot command")
	}
	if domain.SameClientBackendForScope(domain.ClientVSCode, domain.ClientCopilot, domain.ScopeProject) || !domain.SameClientBackendForScope(domain.ClientVSCode, domain.ClientCopilot, domain.ScopeUser) {
		t.Fatal("workspace composition changed user backend sharing")
	}
	for _, id := range domain.SupportedClientIDs() {
		if id == domain.ClientVSCode {
			continue
		}
		original, _ := all.Default().Lookup(id)
		selected, _ := all.Project().Lookup(id)
		if original.ID() != selected.ID() {
			t.Fatal("project registry lost another supported client", id)
		}
	}
}
