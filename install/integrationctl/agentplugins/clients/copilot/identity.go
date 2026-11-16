package copilot

import (
	"context"
	"fmt"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

var _ clients.RegistryInspector = (*Adapter)(nil)
var _ clients.PreparedRegistryInspector = (*Adapter)(nil)

func (*Adapter) UsesNativeRegistryExecutable() bool { return true }

func (*Adapter) InspectNativeRegistry(ctx context.Context, env clients.Env, client domain.DetectedClient, plan domain.DeliveryPlan, managed *domain.ClientBinding) (clients.RegistryFinding, error) {
	if err := ctx.Err(); err != nil {
		return clients.RegistryIndeterminate, err
	}
	if plan.Scope == domain.ScopeProject || plan.TargetContext.Scope == domain.ScopeProject {
		if _, err := copilotPlanRoot(client, plan); err != nil {
			return clients.RegistryIndeterminate, err
		}
		return inspectCopilotProject(plan, managed, env.NativeConfig)
	}
	if managed != nil {
		if err := requireRecordedUserProfile(managed.NativeProfileRoot, client.ConfigRoot); err != nil {
			return clients.RegistryIndeterminate, err
		}
	}
	bound, err := userProfileEnv(env, client.ConfigRoot, plan.NativeRegistryRoot, plan.TargetContext.UserConfigRoot)
	if err != nil {
		return clients.RegistryIndeterminate, err
	}
	return shared.InspectCopilotRegistryWithListingRecognition(ctx, bound, plan, managed, recognizeCopilotUserRegistry)
}

// requireRecordedUserProfile refuses lifecycle work on an owned user binding
// from a different Copilot profile than the one it was installed into (for
// example after COPILOT_HOME was unset). Otherwise an update registered a
// second, untracked copy in the newly selected profile that remove could not
// clean, and a repeat add downgraded the tracked activation after listing the
// wrong profile. Legacy bindings without a recorded profile keep the old
// admission.
func requireRecordedUserProfile(recordedRoot, selectedRoot string) error {
	if recordedRoot == "" || recordedRoot == selectedRoot {
		return nil
	}
	return fmt.Errorf("copilot plugin is installed in a different user profile than the selected one; set COPILOT_HOME to the profile used at install and retry")
}

func (*Adapter) InspectPreparedRegistry(plan domain.DeliveryPlan, name string, owned bool) (clients.RegistryFinding, error) {
	if plan.Scope != domain.ScopeProject && plan.TargetContext.Scope != domain.ScopeProject {
		return shared.InspectUnqualifiedPluginRoot(plan.TargetRoot, name, plan.ActivePath, owned)
	}
	if _, err := copilotPlanRoot(domain.DetectedClient{ConfigRoot: plan.TargetContext.UserConfigRoot}, plan); err != nil {
		return clients.RegistryIndeterminate, err
	}
	var managed *domain.ClientBinding
	if owned {
		managed = &domain.ClientBinding{Scope: string(domain.ScopeProject), NativeProfileRoot: plan.NativeRegistryRoot,
			PhysicalArtifact: plan.PhysicalArtifactID, TargetLocator: plan.ActivePath, NativeObjects: plan.PreviousNativeObjects}
	}
	return inspectCopilotProject(plan, managed, nativeconfig.New())
}
