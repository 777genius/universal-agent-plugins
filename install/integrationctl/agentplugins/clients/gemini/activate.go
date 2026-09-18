package gemini

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// AutomaticallyActivates reports that Activate will write managed Gemini native
// objects for this exact request.
func (*Adapter) AutomaticallyActivates(_ clients.Env, request domain.ActivationRequest) bool {
	return request.Plan.InstallIntent == domain.InstallIntentAutomatic && shared.HasNativeConfigRoot(request)
}

// VerifierAvailable reports that an exact native configuration listing can be
// observed. It does not need a client executable.
func (*Adapter) VerifierAvailable(_ domain.DetectedClient, plan domain.DeliveryPlan, _ string) bool {
	return len(plan.Components) > 0
}

// Activate installs or verifies managed Gemini CLI skills and MCP servers
// through the native config kernel.
func (*Adapter) Activate(ctx context.Context, env clients.Env, request domain.ActivationRequest) (domain.ActivationOutcome, error) {
	if err := shared.ActivationIdentityMismatch(request); err != nil {
		return domain.ActivationOutcome{}, err
	}
	automatic := request.Plan.InstallIntent == domain.InstallIntentAutomatic && shared.HasNativeConfigRoot(request)
	return shared.CompleteNativeConfigActivation(ctx, request, shared.NativeConfigActivation{
		Automatic:         automatic,
		UnavailableAction: "install Gemini CLI and rerun add with an isolated writable Gemini config root",
		RepairAction:      "repair the managed Gemini CLI skills and MCP configuration",
		RetryAction:       "retry the managed Gemini CLI native installation",
		CompletedAction:   "in a running Gemini CLI session use `/mcp reload` and `/skills reload`, or restart Gemini CLI",
		Verify: func() error {
			return VerifyGeminiNativeObjects(request.Client.ConfigRoot, request.Delivery.NativeObjects, false)
		},
		Activate: func(ctx context.Context, request domain.ActivationRequest) error {
			return ActivateGeminiNativeWithKernel(ctx, request, env.NativeConfig)
		},
	})
}

// Deactivate removes managed Gemini native objects.
func (*Adapter) Deactivate(ctx context.Context, env clients.Env, request domain.DeactivationRequest) (domain.DeactivationOutcome, error) {
	outcome := shared.StartedDeactivation()
	if !request.Confirmed {
		outcome.UserActions = append(outcome.UserActions, "agentplugins will remove its managed Gemini CLI skills and MCP entries automatically")
		return outcome, nil
	}
	if err := DeactivateGeminiNativeWithKernel(ctx, request, env.NativeConfig); err != nil {
		if !shared.CommittedNativeDeactivationCleanup(&outcome, err) {
			return outcome, err
		}
	}
	outcome.ExternalRemovalComplete = true
	return outcome, nil
}
