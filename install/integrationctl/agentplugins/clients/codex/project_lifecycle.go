package codex

import (
	"context"
	"fmt"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func activateCodexProject(ctx context.Context, env clients.Env, request domain.ActivationRequest) (domain.ActivationOutcome, error) {
	root, err := codexProjectPlanRoot(request.Client, request.Plan)
	if err != nil {
		return domain.ActivationOutcome{}, err
	}
	if err := New().PreflightActivation(env, request); err != nil {
		return domain.ActivationOutcome{}, err
	}
	outcome, err := shared.CompleteNativeConfigActivation(ctx, request, shared.NativeConfigActivation{
		Automatic:         request.Plan.InstallIntent == domain.InstallIntentAutomatic && shared.OnlyNativeComponents(request.Plan.Components),
		UnavailableAction: "prepare the supported Codex project skills and MCP components",
		RepairAction:      "repair the exact managed Codex project components",
		RetryAction:       "retry Codex project component installation",
		CompletedAction:   codexProjectDiscoveryAction,
		Verify: func() error {
			return verifyCodexProjectObjects(root, request.Delivery.NativeObjects, false, env.NativeConfig)
		},
		Activate: func(ctx context.Context, request domain.ActivationRequest) error {
			return mutateCodexProject(ctx, env, root, request.Delivery.ActivePath, request.PreviousNativeObjects, request.Delivery.NativeObjects)
		},
	})
	if err != nil {
		return outcome, err
	}
	outcome.Activation, outcome.ActivationAttested = domain.ActivationPrepared, false
	conditions, err := codexProjectConditions(env.NativeConfig, request)
	if err != nil {
		return outcome, fmt.Errorf("codex project configuration was installed but restrictions could not be inspected: %w", err)
	}
	for _, condition := range conditions {
		outcome.UserActions = shared.AppendUnique(outcome.UserActions, condition)
	}
	return outcome, nil
}

func mutateCodexProject(ctx context.Context, env clients.Env, root, active string, previous, desired []domain.NativeObjectOwnership) error {
	if err := validateCodexProjectObjects(root, previous); err != nil {
		return err
	}
	if err := validateCodexProjectObjects(root, desired); err != nil {
		return err
	}
	servers := make(map[string]nativeconfig.Server)
	for _, object := range codexProjectOwned(desired) {
		if object.Kind != codexProjectMCPKind {
			continue
		}
		server, err := readCodexProjectServer(active, object.LogicalName)
		if err != nil {
			return err
		}
		servers[object.LogicalName] = server
	}
	return (shared.ProjectComponentMutation{Layout: codexProjectLayout(root), ActivePath: active,
		Previous: codexProjectOwned(previous), Desired: codexProjectOwned(desired), Servers: servers, Kernel: env.NativeConfig}).Apply(ctx)
}

func deactivateCodexProject(ctx context.Context, env clients.Env, request domain.DeactivationRequest) (domain.DeactivationOutcome, error) {
	outcome := shared.StartedDeactivation()
	root, err := codexProjectRoot(request.Client, request.TargetContext, request.NativeRegistryRoot)
	if err != nil {
		return outcome, err
	}
	if !request.Confirmed {
		outcome.UserActions = append(outcome.UserActions, "agentplugins will remove only its exact managed Codex project skill directories and MCP entries")
		return outcome, nil
	}
	if err := mutateCodexProject(ctx, env, root, "", request.NativeObjects, nil); err != nil {
		if !shared.CommittedNativeDeactivationCleanup(&outcome, err) {
			return outcome, err
		}
	}
	outcome.ExternalRemovalComplete = true
	return outcome, nil
}
