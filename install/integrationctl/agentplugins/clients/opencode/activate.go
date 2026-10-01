package opencode

import (
	"context"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// AutomaticallyActivates reports that Activate will write managed OpenCode
// native objects for this exact request.
func (*Adapter) AutomaticallyActivates(_ clients.Env, request domain.ActivationRequest) bool {
	return automaticallyActivatesOpenCode(request)
}

func automaticallyActivatesOpenCode(request domain.ActivationRequest) bool {
	// Removing the final declarations still reconciles owned native objects.
	// Keep empty first installs manual and preserve non-native plan restrictions.
	cleanup := request.Client.ConfigRoot != "" && len(request.Plan.Components) == 0 && len(OpenCodeObjects(request.PreviousNativeObjects)) > 0
	return request.Plan.InstallIntent == domain.InstallIntentAutomatic && (shared.HasNativeConfigRoot(request) || cleanup)
}

// VerifierAvailable reports that an exact native configuration listing can be
// observed. It does not need a client executable.
func (*Adapter) VerifierAvailable(_ domain.DetectedClient, plan domain.DeliveryPlan, _ string) bool {
	return len(plan.Components) > 0
}

// Activate installs or verifies managed OpenCode skills and MCP servers
// through the native config kernel.
func (*Adapter) Activate(ctx context.Context, env clients.Env, request domain.ActivationRequest) (domain.ActivationOutcome, error) {
	if !request.VerifyOnly && request.Client.OpenCodeHost != nil {
		if err := request.Client.OpenCodeHost.ValidateNative(false, nil); err != nil {
			return domain.ActivationOutcome{}, err
		}
	}
	if err := shared.ActivationIdentityMismatch(request); err != nil {
		return domain.ActivationOutcome{}, err
	}
	automatic := automaticallyActivatesOpenCode(request)
	return shared.CompleteNativeConfigActivation(ctx, request, shared.NativeConfigActivation{
		Automatic:         automatic,
		UnavailableAction: "rerun with a detected OpenCode config root",
		RepairAction:      "repair the managed OpenCode skills and MCP configuration",
		RetryAction:       "retry the managed OpenCode native installation",
		CompletedAction:   "restart OpenCode to load the installed plugin",
		Verify: func() error {
			return VerifyOpenCodeNativeObjects(request.Client.ConfigRoot, request.Delivery.ActivePath, request.Delivery.NativeObjects, env.NativeConfig, false)
		},
		Activate: func(ctx context.Context, request domain.ActivationRequest) error {
			return ActivateOpenCodeNativeWithKernel(ctx, request, env.NativeConfig)
		},
	})
}

// Deactivate removes managed OpenCode native objects.
func (*Adapter) Deactivate(ctx context.Context, env clients.Env, request domain.DeactivationRequest) (domain.DeactivationOutcome, error) {
	outcome := shared.StartedDeactivation()
	if !request.Confirmed {
		outcome.UserActions = append(outcome.UserActions, "agentplugins will remove its managed OpenCode skills and MCP entries automatically")
		return outcome, nil
	}
	if err := DeactivateOpenCodeNativeWithKernel(ctx, request, env.NativeConfig); err != nil {
		if !shared.CommittedNativeDeactivationCleanup(&outcome, err) {
			return outcome, err
		}
	}
	outcome.ExternalRemovalComplete = true
	return outcome, nil
}
