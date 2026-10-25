package vscode

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

var (
	_ clients.Lifecycle                 = (*ProjectAdapter)(nil)
	_ clients.AutomaticActivator        = (*ProjectAdapter)(nil)
	_ clients.ReadOnlyVerifier          = (*ProjectAdapter)(nil)
	_ clients.ActivationPreflighter     = (*ProjectAdapter)(nil)
	_ clients.RegistryInspector         = (*ProjectAdapter)(nil)
	_ clients.PreparedRegistryInspector = (*ProjectAdapter)(nil)
)

func (*ProjectAdapter) AutomaticallyActivates(_ clients.Env, req domain.ActivationRequest) bool {
	return req.Plan.Scope == domain.ScopeProject && req.Plan.InstallIntent == domain.InstallIntentAutomatic && shared.OnlyNativeComponents(req.Plan.Components)
}

func (*ProjectAdapter) VerifierAvailable(_ domain.DetectedClient, plan domain.DeliveryPlan, _ string) bool {
	return plan.Scope == domain.ScopeProject && len(plan.Components) > 0
}

func (*ProjectAdapter) PreflightActivation(env clients.Env, req domain.ActivationRequest) error {
	root, err := workspacePlanRoot(req.Client, req.Plan)
	if err != nil {
		return err
	}
	if req.ActivationComplete {
		return fmt.Errorf("VS Code workspace discovery cannot be established by activation attestation")
	}
	if req.Plan.Status == domain.PlanUnsupported || !shared.OnlyNativeComponents(req.Plan.Components) {
		return fmt.Errorf("VS Code project delivery requires supported skills and MCP components")
	}
	if req.DeclaredName != "" && req.DeclaredName != req.Plan.DeclaredName || req.Delivery.ClientID != "" && req.Delivery.ClientID != domain.ClientVSCode || req.Delivery.ActivePath != "" && req.Delivery.ActivePath != req.Plan.ActivePath || req.Delivery.OwnedBase != "" && req.Delivery.OwnedBase != req.Plan.TargetRoot {
		return fmt.Errorf("VS Code delivered identity differs from workspace plan")
	}
	layout := workspaceComponents(root)
	if err := layout.ValidateObjects(workspaceOwned(req.PreviousNativeObjects)); err != nil {
		return err
	}
	if err := layout.ValidateObjects(workspaceOwned(req.Delivery.NativeObjects)); err != nil {
		return err
	}
	return env.NativeConfig.RequireFileIO()
}

func (adapter *ProjectAdapter) Activate(ctx context.Context, env clients.Env, req domain.ActivationRequest) (domain.ActivationOutcome, error) {
	if err := shared.ActivationIdentityMismatch(req); err != nil {
		return domain.ActivationOutcome{}, err
	}
	if err := adapter.PreflightActivation(env, req); err != nil {
		return domain.ActivationOutcome{}, err
	}
	if req.Delivery.ActivePath != req.Plan.ActivePath || req.Delivery.OwnedBase != req.Plan.TargetRoot || req.DeclaredName != req.Plan.DeclaredName {
		return domain.ActivationOutcome{}, fmt.Errorf("VS Code workspace delivery is incomplete")
	}
	if err := workspaceSelection(req.Plan, req.Delivery.NativeObjects); err != nil {
		return domain.ActivationOutcome{}, err
	}
	layout := workspaceComponents(req.Plan.TargetContext.ProjectRoot)
	out, err := shared.CompleteNativeConfigActivation(ctx, req, shared.NativeConfigActivation{
		Automatic:         adapter.AutomaticallyActivates(env, req),
		UnavailableAction: "select supported VS Code project components", RepairAction: "repair exact owned VS Code project components",
		RetryAction: "retry VS Code workspace component preparation", CompletedAction: workspaceAction(req.Plan.TargetContext.ProjectRoot),
		Verify: func() error {
			return layout.Verify(workspaceOwned(req.Delivery.NativeObjects), false, env.NativeConfig)
		},
		Activate: func(ctx context.Context, r domain.ActivationRequest) error {
			objects := workspaceOwned(r.Delivery.NativeObjects)
			servers, err := workspaceServers(r.Delivery.ActivePath, objects)
			if err != nil {
				return &shared.NativeEffectError{Effect: domain.NativeEffectUnchanged, Err: err}
			}
			return (shared.ProjectComponentMutation{Layout: layout, ActivePath: r.Delivery.ActivePath, Previous: workspaceOwned(r.PreviousNativeObjects), Desired: objects, Servers: servers, Kernel: env.NativeConfig}).Apply(ctx)
		},
	})
	if err == nil && out.Verification == domain.VerificationInstalled {
		out.Activation, out.ActivationAttested = domain.ActivationPrepared, false
	}
	return out, err
}

func (adapter *ProjectAdapter) Deactivate(ctx context.Context, env clients.Env, req domain.DeactivationRequest) (domain.DeactivationOutcome, error) {
	out := shared.StartedDeactivation()
	root, err := workspaceRoot(req.Client, req.TargetContext, req.NativeRegistryRoot)
	if err != nil {
		return out, err
	}
	if req.NativeRegistryRoot != filepath.Join(root, ".vscode") {
		return out, fmt.Errorf("VS Code removal native workspace authority is missing")
	}
	binding := domain.ClientBinding{ClientID: "vscode", Scope: "project", NativeProfileRoot: filepath.Join(root, ".vscode"), PhysicalArtifact: req.PhysicalArtifactID, TargetLocator: req.ManagedArtifactPath, NativeObjects: req.NativeObjects}
	if err := adapter.ValidateBindingProfile(filepath.Join(root, ".vscode"), binding); err != nil {
		return out, err
	}
	if !req.Confirmed {
		out.UserActions = []string{"agentplugins will remove only exact owned VS Code workspace skills and MCP members"}
		return out, nil
	}
	err = (shared.ProjectComponentMutation{Layout: workspaceComponents(root), Previous: workspaceOwned(req.NativeObjects), Kernel: env.NativeConfig}).Apply(ctx)
	if err != nil && !shared.CommittedNativeDeactivationCleanup(&out, err) {
		return out, err
	}
	out.ExternalRemovalComplete = true
	return out, nil
}

func (*ProjectAdapter) UsesNativeRegistryExecutable() bool { return false }

func (adapter *ProjectAdapter) InspectNativeRegistry(ctx context.Context, env clients.Env, client domain.DetectedClient, plan domain.DeliveryPlan, managed *domain.ClientBinding) (clients.RegistryFinding, error) {
	if err := ctx.Err(); err != nil {
		return clients.RegistryIndeterminate, err
	}
	root, err := workspacePlanRoot(client, plan)
	if err != nil {
		return clients.RegistryIndeterminate, err
	}
	if managed != nil {
		if managed.PhysicalArtifact != plan.PhysicalArtifactID || managed.TargetLocator != plan.ActivePath {
			return clients.RegistryIndeterminate, fmt.Errorf("VS Code binding differs from selected physical artifact")
		}
		if err := adapter.ValidateBindingProfile(filepath.Join(root, ".vscode"), *managed); err != nil {
			return clients.RegistryIndeterminate, err
		}
		selected := *managed
		selected.NativeObjects = workspaceOwned(managed.NativeObjects)
		managed = &selected
	}
	return workspaceComponents(root).Inspect(plan.Components, managed, env.NativeConfig)
}

func (*ProjectAdapter) InspectPreparedRegistry(plan domain.DeliveryPlan, name string, _ bool) (clients.RegistryFinding, error) {
	_, err := workspacePlanRoot(domain.DetectedClient{ClientID: domain.ClientVSCode, ConfigRoot: plan.TargetContext.UserConfigRoot}, plan)
	if err != nil {
		return clients.RegistryIndeterminate, err
	}
	if name != plan.DeclaredName {
		return clients.RegistryIndeterminate, fmt.Errorf("VS Code prepared workspace identity differs")
	}
	return clients.RegistryClear, nil
}
