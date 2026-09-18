package cline

import (
	"context"
	"fmt"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// AutomaticallyActivates reports that Activate will write managed Cline native
// objects for this exact request.
func (*Adapter) AutomaticallyActivates(_ clients.Env, request domain.ActivationRequest) bool {
	return request.Plan.InstallIntent == domain.InstallIntentAutomatic && shared.HasNativeConfigRoot(request)
}

// VerifierAvailable reports that an exact native configuration listing can be
// observed. It does not need a client executable.
func (*Adapter) VerifierAvailable(_ domain.DetectedClient, plan domain.DeliveryPlan, _ string) bool {
	return len(plan.Components) > 0
}

// Activate installs or verifies managed Cline skills and MCP servers through
// the native config kernel.
func (*Adapter) Activate(ctx context.Context, env clients.Env, request domain.ActivationRequest) (domain.ActivationOutcome, error) {
	if err := shared.ActivationIdentityMismatch(request); err != nil {
		return domain.ActivationOutcome{}, err
	}
	automatic := request.Plan.InstallIntent == domain.InstallIntentAutomatic && shared.HasNativeConfigRoot(request)
	return shared.CompleteNativeConfigActivation(ctx, request, shared.NativeConfigActivation{
		Automatic:         automatic,
		UnavailableAction: "install Cline and rerun add to register its skills and MCP servers",
		RepairAction:      "repair the managed Cline skills and MCP configuration",
		RetryAction:       "retry the managed Cline native installation",
		CompletedAction:   "reload the Cline MCP view in VS Code, or start a new Cline CLI process",
		Verify: func() error {
			return VerifyClineNativeObjects(request.Client.ConfigRoot, request.Delivery.NativeObjects, false)
		},
		Activate: func(ctx context.Context, request domain.ActivationRequest) error {
			return ActivateClineNativeWithKernel(ctx, request, env.NativeConfig)
		},
	})
}

// Deactivate removes managed Cline native objects.
func (*Adapter) Deactivate(ctx context.Context, env clients.Env, request domain.DeactivationRequest) (domain.DeactivationOutcome, error) {
	outcome := shared.StartedDeactivation()
	if len(ClineObjects(request.NativeObjects)) == 0 {
		return outcome, fmt.Errorf("managed Cline native ownership is missing")
	}
	if !request.Confirmed {
		outcome.UserActions = append(outcome.UserActions, "agentplugins will remove only its managed Cline skills and MCP entries")
		return outcome, nil
	}
	if err := DeactivateClineNativeWithKernel(ctx, request, env.NativeConfig); err != nil {
		if !shared.CommittedNativeDeactivationCleanup(&outcome, err) {
			return outcome, err
		}
	}
	outcome.ExternalRemovalComplete = true
	return outcome, nil
}
