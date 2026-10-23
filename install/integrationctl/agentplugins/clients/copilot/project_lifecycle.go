package copilot

import (
	"context"
	"fmt"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

var _ clients.ActivationPreflighter = (*Adapter)(nil)

func (*Adapter) PreflightActivation(env clients.Env, request domain.ActivationRequest) error {
	if request.Plan.Scope != domain.ScopeProject && request.Plan.TargetContext.Scope != domain.ScopeProject {
		return nil
	}
	root, err := copilotPlanRoot(request.Client, request.Plan)
	if err != nil {
		return err
	}
	if request.ActivationComplete {
		return fmt.Errorf("copilot project native discovery cannot be established by activation attestation; inspect the selected trusted project")
	}
	if err := env.NativeConfig.RequireFileIO(); err != nil {
		return err
	}
	if shared.ComponentKindPresent(request.Plan.Components, domain.ComponentMCPServer) {
		layout, err := copilotComponents(root)
		if err != nil {
			return err
		}
		return validateCopilotConfig(env.NativeConfig, layout.ConfigPaths.JSON)
	}
	return nil
}

func activateCopilotProject(ctx context.Context, env clients.Env, request domain.ActivationRequest) (domain.ActivationOutcome, error) {
	root, err := copilotPlanRoot(request.Client, request.Plan)
	if err != nil {
		return domain.ActivationOutcome{}, err
	}
	if err := New().PreflightActivation(env, request); err != nil {
		return domain.ActivationOutcome{}, err
	}
	layout, err := copilotComponents(root)
	if err != nil {
		return domain.ActivationOutcome{}, err
	}
	outcome, err := shared.CompleteNativeConfigActivation(ctx, request, shared.NativeConfigActivation{
		Automatic:         request.Plan.InstallIntent == domain.InstallIntentAutomatic && shared.OnlyNativeComponents(request.Plan.Components),
		UnavailableAction: "select supported Copilot project skills and MCP components",
		RepairAction:      "repair exact managed Copilot project components", RetryAction: "retry Copilot project component installation",
		CompletedAction: "Copilot project components are configured; native discovery, folder trust, plugin/session/admin precedence, disabled selection and MCP initialization remain unobserved",
		Verify: func() error {
			return layout.Verify(copilotNativeObjects(request.Delivery.NativeObjects), false, env.NativeConfig)
		},
		Activate: func(ctx context.Context, request domain.ActivationRequest) error {
			servers := map[string]nativeconfig.Server{}
			for _, object := range copilotNativeObjects(request.Delivery.NativeObjects) {
				if object.Kind == copilotProjectMCPKind {
					server, err := readCopilotProjectServer(request.Delivery.ActivePath, object.LogicalName)
					if err != nil {
						return &shared.NativeEffectError{Effect: domain.NativeEffectUnchanged, Err: err}
					}
					servers[object.LogicalName] = server
				}
			}
			return (shared.ProjectComponentMutation{Layout: layout, ActivePath: request.Delivery.ActivePath,
				Previous: copilotNativeObjects(request.PreviousNativeObjects), Desired: copilotNativeObjects(request.Delivery.NativeObjects), Servers: servers, Kernel: env.NativeConfig}).Apply(ctx)
		},
	})
	if err == nil && outcome.Verification == domain.VerificationInstalled {
		outcome.Activation, outcome.ActivationAttested = domain.ActivationPrepared, false
	}
	return outcome, err
}

func deactivateCopilotProject(ctx context.Context, env clients.Env, request domain.DeactivationRequest) (domain.DeactivationOutcome, error) {
	outcome := shared.StartedDeactivation()
	root, err := copilotProjectRoot(request.Client, request.TargetContext, request.NativeRegistryRoot)
	if err != nil {
		return outcome, err
	}
	if !request.Confirmed {
		outcome.UserActions = append(outcome.UserActions, "agentplugins will remove only exact owned Copilot repository skills and MCP members")
		return outcome, nil
	}
	layout, err := copilotComponents(root)
	if err != nil {
		return outcome, err
	}
	if err := (shared.ProjectComponentMutation{Layout: layout, Previous: copilotNativeObjects(request.NativeObjects), Kernel: env.NativeConfig}).Apply(ctx); err != nil && !shared.CommittedNativeDeactivationCleanup(&outcome, err) {
		return outcome, err
	}
	outcome.ExternalRemovalComplete = true
	return outcome, nil
}
