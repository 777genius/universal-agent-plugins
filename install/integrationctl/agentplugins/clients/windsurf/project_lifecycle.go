package windsurf

import (
	"context"
	"fmt"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

var _ clients.ActivationPreflighter = (*Adapter)(nil)

func (*Adapter) PreflightActivation(env clients.Env, request domain.ActivationRequest) error {
	if request.Plan.Scope != domain.ScopeProject && request.Plan.TargetContext.Scope != domain.ScopeProject {
		return nil
	}
	root, err := devinProjectPlanRoot(request.Client, request.Plan)
	if err != nil {
		return err
	}
	if err := requireCurrentDevin(request.Client); err != nil {
		return err
	}
	if request.Delivery.ActivePath != "" && request.Delivery.ActivePath != request.Plan.ActivePath {
		return fmt.Errorf("devin delivered package differs from selected project")
	}
	if request.Plan.ClientID != request.Client.ClientID {
		return fmt.Errorf("devin project plan client identity mismatch")
	}
	if request.Delivery.ClientID != "" {
		if err := shared.ActivationIdentityMismatch(request); err != nil {
			return err
		}
	}
	if request.ActivationComplete {
		return fmt.Errorf("devin project native discovery cannot be established by activation attestation")
	}
	if !shared.OnlyNativeComponents(request.Plan.Components) {
		return fmt.Errorf("devin project delivery requires supported skills and MCP servers only")
	}
	layout := devinComponents(root)
	if err := layout.ValidateObjects(devinNativeObjects(request.PreviousNativeObjects)); err != nil {
		return err
	}
	if err := layout.ValidateObjects(devinNativeObjects(request.Delivery.NativeObjects)); err != nil {
		return err
	}
	return checkDevinProjectPolicy(root, request.Plan.Components, env.NativeConfig)
}

func (adapter *Adapter) activateDevinProject(ctx context.Context, env clients.Env, request domain.ActivationRequest) (domain.ActivationOutcome, error) {
	if err := shared.ActivationIdentityMismatch(request); err != nil {
		return domain.ActivationOutcome{}, err
	}
	if request.Delivery.ActivePath != request.Plan.ActivePath {
		return domain.ActivationOutcome{}, fmt.Errorf("devin delivered package differs from selected project")
	}
	if err := adapter.PreflightActivation(env, request); err != nil {
		return domain.ActivationOutcome{}, err
	}
	layout := devinComponents(request.Plan.NativeRegistryRoot)
	outcome, err := shared.CompleteNativeConfigActivation(ctx, request, shared.NativeConfigActivation{
		Automatic:         adapter.AutomaticallyActivates(env, request),
		UnavailableAction: "select only supported Devin project skills and MCP servers",
		RepairAction:      "repair managed Devin project skills and MCP entries",
		RetryAction:       "retry managed Devin project component installation",
		CompletedAction:   DevinProjectAction,
		Verify: func() error {
			return layout.Verify(devinNativeObjects(request.Delivery.NativeObjects), false, env.NativeConfig)
		},
		Activate: func(ctx context.Context, request domain.ActivationRequest) error {
			servers, err := devinDesiredServers(request.Delivery.ActivePath, devinNativeObjects(request.Delivery.NativeObjects))
			if err != nil {
				return &shared.NativeEffectError{Effect: domain.NativeEffectUnchanged, Err: err}
			}
			return (shared.ProjectComponentMutation{Layout: layout, ActivePath: request.Delivery.ActivePath,
				Previous: devinNativeObjects(request.PreviousNativeObjects), Desired: devinNativeObjects(request.Delivery.NativeObjects), Servers: servers, Kernel: env.NativeConfig}).Apply(ctx)
		},
	})
	if err == nil && outcome.Verification == domain.VerificationInstalled {
		outcome.Activation, outcome.ActivationAttested = domain.ActivationPrepared, false
		outcome.UserActions = shared.AppendUnique(outcome.UserActions, DevinProjectAction)
	}
	return outcome, err
}

// Exact recorded removal has no current executable/version prerequisite and
// never invokes native CLI remove (whose default scope is local).
func deactivateDevinProject(ctx context.Context, env clients.Env, request domain.DeactivationRequest) (domain.DeactivationOutcome, error) {
	outcome := shared.StartedDeactivation()
	root, err := devinProjectRoot(request.Client, request.TargetContext, request.NativeRegistryRoot)
	if err != nil {
		return outcome, err
	}
	layout := devinComponents(root)
	if err := layout.ValidateObjects(devinNativeObjects(request.NativeObjects)); err != nil {
		return outcome, err
	}
	if err := env.NativeConfig.RequireFileIO(); err != nil {
		return outcome, err
	}
	if !request.Confirmed {
		outcome.UserActions = []string{"agentplugins will remove its exact owned Devin project skill directories and MCP members"}
		return outcome, nil
	}
	err = (shared.ProjectComponentMutation{Layout: layout, Previous: devinNativeObjects(request.NativeObjects), Kernel: env.NativeConfig}).Apply(ctx)
	if err != nil && !shared.CommittedNativeDeactivationCleanup(&outcome, err) {
		return outcome, err
	}
	outcome.ExternalRemovalComplete = true
	return outcome, nil
}
