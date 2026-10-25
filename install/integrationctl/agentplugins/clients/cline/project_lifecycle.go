package cline

import (
	"context"
	"fmt"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

var _ clients.ActivationPreflighter = (*Adapter)(nil)

func (*Adapter) PreflightActivation(env clients.Env, request domain.ActivationRequest) error {
	if !clineProjectSelected(request.Plan) {
		return nil
	}
	root, err := clineProjectPlanRoot(request.Client, request.Plan)
	if err != nil {
		return err
	}
	if request.DeclaredName != "" && request.DeclaredName != request.Plan.DeclaredName {
		return fmt.Errorf("cline delivered name differs")
	}
	if request.ActivationComplete {
		return fmt.Errorf("cline project session discovery cannot be established by activation attestation")
	}
	if request.Plan.Status == domain.PlanUnsupported || !shared.OnlyNativeComponents(request.Plan.Components) {
		return fmt.Errorf("cline project requires supported skills and MCP components")
	}
	if request.Delivery.ClientID != "" && request.Delivery.ClientID != domain.ClientCline {
		return fmt.Errorf("cline delivered client differs")
	}
	if request.Delivery.ActivePath != "" && request.Delivery.ActivePath != request.Plan.ActivePath {
		return fmt.Errorf("cline delivered package differs")
	}
	if request.Delivery.OwnedBase != "" && request.Delivery.OwnedBase != request.Plan.TargetRoot {
		return fmt.Errorf("cline delivered base differs")
	}
	layout := clineProjectComponents(root)
	if err := layout.ValidateObjects(clineProjectOwned(request.PreviousNativeObjects)); err != nil {
		return err
	}
	if err := layout.ValidateObjects(clineProjectOwned(request.Delivery.NativeObjects)); err != nil {
		return err
	}
	return env.NativeConfig.RequireFileIO()
}

func activateClineProject(ctx context.Context, env clients.Env, request domain.ActivationRequest) (domain.ActivationOutcome, error) {
	if err := shared.ActivationIdentityMismatch(request); err != nil {
		return domain.ActivationOutcome{}, err
	}
	if err := New().PreflightActivation(env, request); err != nil {
		return domain.ActivationOutcome{}, err
	}
	if request.Delivery.ActivePath != request.Plan.ActivePath || request.Delivery.OwnedBase != request.Plan.TargetRoot {
		return domain.ActivationOutcome{}, fmt.Errorf("cline project delivery is incomplete or differs from plan")
	}
	if request.DeclaredName != request.Plan.DeclaredName {
		return domain.ActivationOutcome{}, fmt.Errorf("cline delivered name differs")
	}
	if err := validateClineProjectSelection(request.Plan, request.Delivery.NativeObjects); err != nil {
		return domain.ActivationOutcome{}, err
	}
	layout := clineProjectComponents(request.Plan.TargetContext.ProjectRoot)
	out, err := shared.CompleteNativeConfigActivation(ctx, request, shared.NativeConfigActivation{
		Automatic:         request.Plan.InstallIntent == domain.InstallIntentAutomatic,
		UnavailableAction: "select supported Cline project components", RepairAction: "repair exact owned Cline project components", RetryAction: "retry Cline project component preparation", CompletedAction: clineProjectAction(request.Plan),
		Verify: func() error {
			return layout.Verify(clineProjectOwned(request.Delivery.NativeObjects), false, env.NativeConfig)
		},
		Activate: func(ctx context.Context, r domain.ActivationRequest) error {
			objects := clineProjectOwned(r.Delivery.NativeObjects)
			servers, err := clineProjectServers(r.Delivery.ActivePath, objects)
			if err != nil {
				return &shared.NativeEffectError{Effect: domain.NativeEffectUnchanged, Err: err}
			}
			return (shared.ProjectComponentMutation{Layout: layout, ActivePath: r.Delivery.ActivePath, Previous: clineProjectOwned(r.PreviousNativeObjects), Desired: objects, Servers: servers, Kernel: env.NativeConfig}).Apply(ctx)
		},
	})
	if err == nil && out.Verification == domain.VerificationInstalled {
		out.Activation, out.ActivationAttested = domain.ActivationPrepared, false
	}
	return out, err
}

func deactivateClineProject(ctx context.Context, env clients.Env, request domain.DeactivationRequest) (domain.DeactivationOutcome, error) {
	out := shared.StartedDeactivation()
	root, err := clineProjectRoot(request.Client, request.TargetContext, request.NativeRegistryRoot)
	if err != nil {
		return out, err
	}
	binding := domain.ClientBinding{ClientID: "cline", Scope: "project", NativeProfileRoot: root, PhysicalArtifact: request.PhysicalArtifactID, TargetLocator: request.ManagedArtifactPath, NativeObjects: request.NativeObjects}
	if err := New().ValidateBindingProfile(root, binding); err != nil {
		return out, err
	}
	if !request.Confirmed {
		out.UserActions = []string{"agentplugins will remove only exact owned Cline project skill directories and prepared MCP members"}
		return out, nil
	}
	if err := (shared.ProjectComponentMutation{Layout: clineProjectComponents(root), Previous: clineProjectOwned(request.NativeObjects), Kernel: env.NativeConfig}).Apply(ctx); err != nil && !shared.CommittedNativeDeactivationCleanup(&out, err) {
		return out, err
	}
	out.ExternalRemovalComplete = true
	out.UserActions = []string{"Start a fresh Cline CLI process to observe removal; existing sessions may retain previously selected MCP configuration"}
	return out, nil
}
