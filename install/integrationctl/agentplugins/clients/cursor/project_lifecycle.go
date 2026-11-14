package cursor

import (
	"context"
	"fmt"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

var (
	_ clients.AutomaticActivator    = (*Adapter)(nil)
	_ clients.ReadOnlyVerifier      = (*Adapter)(nil)
	_ clients.ActivationPreflighter = (*Adapter)(nil)
)

func (*Adapter) AutomaticallyActivates(_ clients.Env, request domain.ActivationRequest) bool {
	return request.Plan.Scope == domain.ScopeProject && request.Plan.InstallIntent == domain.InstallIntentAutomatic && shared.OnlyNativeComponents(request.Plan.Components)
}

func (*Adapter) VerifierAvailable(_ domain.DetectedClient, plan domain.DeliveryPlan, _ string) bool {
	return plan.Scope == domain.ScopeProject && len(plan.Components) > 0
}

func (*Adapter) PreflightActivation(env clients.Env, request domain.ActivationRequest) error {
	if request.Plan.Scope != domain.ScopeProject && request.Plan.TargetContext.Scope != domain.ScopeProject {
		return nil
	}
	if _, err := cursorPlanRoot(request.Client, request.Plan); err != nil {
		return err
	}
	if request.ActivationComplete {
		return fmt.Errorf("cursor project native discovery cannot be established by activation attestation; inspect skills and MCP in the selected project")
	}
	return env.NativeConfig.RequireFileIO()
}

func (adapter *Adapter) activateProject(ctx context.Context, env clients.Env, request domain.ActivationRequest) (domain.ActivationOutcome, error) {
	root, err := cursorPlanRoot(request.Client, request.Plan)
	if err != nil {
		return domain.ActivationOutcome{}, err
	}
	if err := adapter.PreflightActivation(env, request); err != nil {
		return domain.ActivationOutcome{}, err
	}
	layout := cursorComponents(root)
	outcome, err := shared.CompleteNativeConfigActivation(ctx, request, shared.NativeConfigActivation{
		Automatic:         adapter.AutomaticallyActivates(env, request),
		UnavailableAction: "select only supported Cursor project skills and MCP servers",
		RepairAction:      "repair managed Cursor project skills and MCP entries",
		RetryAction:       "retry managed Cursor project component installation",
		CompletedAction:   "open the selected project in Cursor and inspect skills and MCP servers",
		Verify: func() error {
			return layout.Verify(cursorNativeObjects(request.Delivery.NativeObjects), false, env.NativeConfig)
		},
		Activate: func(ctx context.Context, request domain.ActivationRequest) error {
			servers, err := cursorDesiredServers(request.Delivery.ActivePath, cursorNativeObjects(request.Delivery.NativeObjects))
			if err != nil {
				return &shared.NativeEffectError{Effect: domain.NativeEffectUnchanged, Err: err}
			}
			return (shared.ProjectComponentMutation{Layout: layout, ActivePath: request.Delivery.ActivePath,
				Previous: cursorNativeObjects(request.PreviousNativeObjects), Desired: cursorNativeObjects(request.Delivery.NativeObjects), Servers: servers, Kernel: env.NativeConfig}).Apply(ctx)
		},
	})
	if err == nil && outcome.Verification == domain.VerificationInstalled {
		outcome.Activation, outcome.ActivationAttested = domain.ActivationPrepared, false
		outcome.UserActions = shared.AppendUnique(outcome.UserActions, "Cursor project configuration is installed; native discovery, aliases, enablement, approvals, team policy and MCP initialization remain unobserved")
	}
	return outcome, err
}

func deactivateCursorProject(ctx context.Context, env clients.Env, request domain.DeactivationRequest) (domain.DeactivationOutcome, error) {
	outcome := shared.StartedDeactivation()
	root, err := cursorNativeRoot(request.Client, request.TargetContext, request.NativeRegistryRoot)
	if err != nil {
		return outcome, err
	}
	if !request.Confirmed {
		outcome.UserActions = append(outcome.UserActions, "agentplugins will remove its exact owned Cursor project skill directories and MCP members")
		return outcome, nil
	}
	err = (shared.ProjectComponentMutation{Layout: cursorComponents(root), Previous: cursorNativeObjects(request.NativeObjects), Kernel: env.NativeConfig}).Apply(ctx)
	if err != nil && !shared.CommittedNativeDeactivationCleanup(&outcome, err) {
		return outcome, err
	}
	outcome.ExternalRemovalComplete = true
	return outcome, nil
}
