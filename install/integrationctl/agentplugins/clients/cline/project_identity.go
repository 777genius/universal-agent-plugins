package cline

import (
	"fmt"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

var _ clients.PreparedRegistryInspector = (*Adapter)(nil)

func inspectClineProject(client domain.DetectedClient, plan domain.DeliveryPlan, managed *domain.ClientBinding, kernel nativeconfig.Kernel) (clients.RegistryFinding, error) {
	root, err := clineProjectPlanRoot(client, plan)
	if err != nil {
		return clients.RegistryIndeterminate, err
	}
	if managed != nil {
		if managed.Scope != "project" || managed.ClientID != "cline" || managed.PhysicalArtifact != plan.PhysicalArtifactID || managed.TargetLocator != plan.ActivePath {
			return clients.RegistryIndeterminate, fmt.Errorf("Cline project binding differs from selected artifact")
		}
		if err := New().ValidateBindingProfile(root, *managed); err != nil {
			return clients.RegistryIndeterminate, err
		}
		selected := *managed
		selected.NativeObjects = clineProjectOwned(managed.NativeObjects)
		managed = &selected
	}
	return clineProjectComponents(root).Inspect(plan.Components, managed, kernel)
}

func (*Adapter) InspectPreparedRegistry(plan domain.DeliveryPlan, name string, owned bool) (clients.RegistryFinding, error) {
	if !clineProjectSelected(plan) {
		return shared.InspectUnqualifiedPluginRoot(plan.TargetRoot, name, plan.ActivePath, owned)
	}
	if _, err := clineProjectPlanRoot(domain.DetectedClient{ClientID: domain.ClientCline, ConfigRoot: plan.TargetContext.UserConfigRoot}, plan); err != nil {
		return clients.RegistryIndeterminate, err
	}
	if name != plan.DeclaredName {
		return clients.RegistryIndeterminate, fmt.Errorf("Cline project prepared name differs")
	}
	// Native file targets are inspected by InspectNativeRegistry with complete
	// receipts; the core independently verifies the managed package digest.
	return clients.RegistryClear, nil
}
