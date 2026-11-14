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
	root := request.Client.ConfigRoot
	native := shared.HasNativeConfigRoot(request)
	if request.Plan.Scope == domain.ScopeProject {
		layout, err := openCodeActivationLayout(request)
		if err != nil {
			return false
		}
		root = layout.Root
		native = root != "" && shared.OnlyNativeComponents(request.Plan.Components)
	}
	cleanup := root != "" && len(request.Plan.Components) == 0 && len(OpenCodeObjects(request.PreviousNativeObjects)) > 0
	return request.Plan.InstallIntent == domain.InstallIntentAutomatic && (native || cleanup)
}

// VerifierAvailable reports that an exact native configuration listing can be
// observed. It does not need a client executable.
func (*Adapter) VerifierAvailable(_ domain.DetectedClient, plan domain.DeliveryPlan, _ string) bool {
	return len(plan.Components) > 0
}

// Activate installs or verifies managed OpenCode skills and MCP servers
// through the native config kernel.
func (adapter *Adapter) Activate(ctx context.Context, env clients.Env, request domain.ActivationRequest) (domain.ActivationOutcome, error) {
	if !request.VerifyOnly && request.Client.OpenCodeHost != nil {
		if err := request.Client.OpenCodeHost.ValidateNative(false, nil); err != nil {
			return domain.ActivationOutcome{}, err
		}
	}
	if err := shared.ActivationIdentityMismatch(request); err != nil {
		return domain.ActivationOutcome{}, err
	}
	layout, err := openCodeActivationLayout(request)
	if err != nil {
		return domain.ActivationOutcome{}, err
	}
	if err := adapter.PreflightActivation(env, request); err != nil {
		return domain.ActivationOutcome{}, err
	}
	automatic := automaticallyActivatesOpenCode(request)
	outcome, err := shared.CompleteNativeConfigActivation(ctx, request, shared.NativeConfigActivation{
		Automatic:         automatic,
		UnavailableAction: "rerun with a detected OpenCode config root",
		RepairAction:      "repair the managed OpenCode skills and MCP configuration",
		RetryAction:       "retry the managed OpenCode native installation",
		CompletedAction:   "restart OpenCode to load the installed plugin",
		Verify: func() error {
			return verifyOpenCodeNativeLayout(layout, request.Delivery.ActivePath, request.Delivery.NativeObjects, env.NativeConfig, false)
		},
		Activate: func(ctx context.Context, request domain.ActivationRequest) error {
			if crossOpenCodeDialect(request.PreviousNativeObjects, request.Delivery.NativeObjects) {
				return applyOpenCodeTransition(ctx, request, env.NativeConfig, env.OpenCodeTransitions)
			}
			return ActivateOpenCodeNativeWithKernel(ctx, request, env.NativeConfig)
		},
	})
	if err != nil {
		return outcome, err
	}
	return openCodeProjectStatus(request, outcome), nil
}

// Deactivate removes managed OpenCode native objects.
func (*Adapter) Deactivate(ctx context.Context, env clients.Env, request domain.DeactivationRequest) (domain.DeactivationOutcome, error) {
	outcome := shared.StartedDeactivation()
	if _, err := openCodeTargetLayout(request.Client, request.TargetContext, request.NativeRegistryRoot); err != nil {
		return outcome, err
	}
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
