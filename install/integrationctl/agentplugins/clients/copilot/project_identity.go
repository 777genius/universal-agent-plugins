package copilot

import (
	"fmt"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func inspectCopilotProject(plan domain.DeliveryPlan, managed *domain.ClientBinding, kernel nativeconfig.Kernel) (clients.RegistryFinding, error) {
	layout, err := copilotComponents(plan.NativeRegistryRoot)
	if err != nil {
		return clients.RegistryIndeterminate, err
	}
	if shared.ComponentKindPresent(plan.Components, domain.ComponentMCPServer) || copilotHasOwnedMCP(plan.PreviousNativeObjects) {
		if err := validateCopilotConfig(kernel, layout.ConfigPaths.JSON); err != nil {
			return clients.RegistryIndeterminate, err
		}
	}
	if managed != nil {
		if managed.Scope != string(domain.ScopeProject) || managed.PhysicalArtifact != plan.PhysicalArtifactID || managed.TargetLocator != plan.ActivePath {
			return clients.RegistryIndeterminate, fmt.Errorf("copilot project binding scope, artifact or locator differs from plan")
		}
		if err := New().ValidateBindingProfile(plan.NativeRegistryRoot, *managed); err != nil {
			return clients.RegistryIndeterminate, err
		}
		selected := *managed
		selected.NativeObjects = copilotNativeObjects(managed.NativeObjects)
		managed = &selected
	}
	return layout.Inspect(plan.Components, managed, kernel)
}

func copilotHasOwnedMCP(objects []domain.NativeObjectOwnership) bool {
	for _, object := range objects {
		if object.Kind == copilotProjectMCPKind {
			return true
		}
	}
	return false
}
