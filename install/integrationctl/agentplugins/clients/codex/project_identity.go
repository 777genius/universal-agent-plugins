package codex

import (
	"fmt"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func inspectPreparedCodexProject(plan domain.DeliveryPlan, owned bool) (clients.RegistryFinding, error) {
	var managed *domain.ClientBinding
	if owned {
		managed = &domain.ClientBinding{Scope: string(domain.ScopeProject), NativeProfileRoot: plan.NativeRegistryRoot, NativeObjects: plan.PreviousNativeObjects}
	}
	return inspectCodexProject(plan, managed, nativeconfig.New())
}

func inspectCodexProject(plan domain.DeliveryPlan, managed *domain.ClientBinding, kernel nativeconfig.Kernel) (clients.RegistryFinding, error) {
	if managed != nil {
		if managed.PhysicalArtifact != "" && managed.PhysicalArtifact != plan.PhysicalArtifactID {
			return clients.RegistryIndeterminate, fmt.Errorf("codex project binding artifact differs from plan")
		}
		if managed.Scope != string(domain.ScopeProject) {
			return clients.RegistryIndeterminate, fmt.Errorf("codex project binding scope differs from plan")
		}
		if err := ValidateBindingProfile(plan.NativeRegistryRoot, *managed); err != nil {
			return clients.RegistryIndeterminate, err
		}
		selected := *managed
		selected.NativeObjects = codexProjectOwned(managed.NativeObjects)
		managed = &selected
	}
	return codexProjectLayout(plan.NativeRegistryRoot).Inspect(plan.Components, managed, kernel)
}

func verifyCodexProjectObjects(root string, objects []domain.NativeObjectOwnership, allowMissing bool, kernel nativeconfig.Kernel) error {
	if err := validateCodexProjectObjects(root, objects); err != nil {
		return err
	}
	return codexProjectLayout(root).Verify(codexProjectOwned(objects), allowMissing, kernel)
}
