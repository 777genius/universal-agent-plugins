package windsurf

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// AutomaticallyActivates reports that Activate will write owned Windsurf MCP
// entries for this exact request.
func (*Adapter) AutomaticallyActivates(_ clients.Env, request domain.ActivationRequest) bool {
	return request.Plan.InstallIntent == domain.InstallIntentAutomatic &&
		strings.TrimSpace(request.Client.ConfigRoot) != "" &&
		len(WindsurfObjects(request.Delivery.NativeObjects)) > 0
}

// VerifierAvailable reports that an exact native configuration listing can be
// observed. It does not need a client executable.
func (*Adapter) VerifierAvailable(_ domain.DetectedClient, plan domain.DeliveryPlan, _ string) bool {
	return len(plan.Components) > 0
}

// Activate installs or verifies owned Windsurf MCP entries, or leaves a
// channel-selection follow-up when automatic installation is unavailable.
func (*Adapter) Activate(ctx context.Context, env clients.Env, request domain.ActivationRequest) (domain.ActivationOutcome, error) {
	if err := shared.ActivationIdentityMismatch(request); err != nil {
		return domain.ActivationOutcome{}, err
	}
	outcome := shared.StartedActivation(request)
	automatic := request.Plan.InstallIntent == domain.InstallIntentAutomatic &&
		strings.TrimSpace(request.Client.ConfigRoot) != "" &&
		len(WindsurfObjects(request.Delivery.NativeObjects)) > 0
	if !automatic {
		outcome.Activation = domain.ActivationManual
		if shared.HasSupportedMCP(request.Plan.Components) {
			outcome.UserActions = append(outcome.UserActions, "select one legacy Windsurf channel and add the prepared MCP servers manually")
			outcome.LocalActions = append(outcome.LocalActions, fmt.Sprintf("Windsurf: import MCP servers from %s; Devin cloud-synced configuration is never changed automatically", filepath.Join(request.Delivery.ActivePath, "mcp.json")))
			return outcome, nil
		}
		outcome.UserActions = append(outcome.UserActions, "use the prepared skills manually in Windsurf; agentplugins does not claim automatic skill activation")
		outcome.LocalActions = append(outcome.LocalActions, fmt.Sprintf("Windsurf: prepared skills remain at %s; Devin cloud-synced configuration is never changed automatically", filepath.Join(request.Delivery.ActivePath, "skills")))
		return outcome, nil
	}
	if request.VerifyOnly {
		if err := VerifyWindsurfNativeObjects(request.Client.ConfigRoot, request.Delivery.ActivePath, request.Delivery.NativeObjects, false); err != nil {
			return shared.FailedActivation(outcome, "repair the managed Windsurf MCP configuration", err)
		}
		outcome.Activation = domain.ActivationActive
		outcome.Verification = domain.VerificationInstalled
		return outcome, nil
	}
	if err := ActivateWindsurfNativeWithKernel(ctx, request, env.NativeConfig); err != nil {
		if !shared.CommittedNativeCleanup(&outcome, err) {
			return shared.FailedActivation(outcome, "retry the managed Windsurf MCP installation", err)
		}
	}
	outcome.Activation = domain.ActivationActive
	outcome.Verification = domain.VerificationInstalled
	outcome.UserActions = append(outcome.UserActions, "refresh MCP servers in Windsurf before first use")
	return outcome, nil
}

// Deactivate removes owned Windsurf MCP entries, or asks the operator to
// finish a leftover channel first.
func (*Adapter) Deactivate(ctx context.Context, env clients.Env, request domain.DeactivationRequest) (domain.DeactivationOutcome, error) {
	outcome := shared.StartedDeactivation()
	if len(WindsurfObjects(request.NativeObjects)) == 0 {
		return shared.RequireExternalUninstall(outcome, request.ExternalUninstalled, "remove the prepared package manually, then rerun remove with `--external-uninstalled`"), nil
	}
	if strings.TrimSpace(request.Client.ConfigRoot) == "" {
		return shared.RequireExternalUninstall(outcome, request.ExternalUninstalled, "select exactly one installed Windsurf channel, then rerun remove"), nil
	}
	if !request.Confirmed {
		outcome.UserActions = append(outcome.UserActions, "agentplugins will remove only its owned Windsurf MCP entries")
		return outcome, nil
	}
	if err := DeactivateWindsurfNativeWithKernel(ctx, request, env.NativeConfig); err != nil {
		if !shared.CommittedNativeDeactivationCleanup(&outcome, err) {
			return outcome, err
		}
	}
	outcome.ExternalRemovalComplete = true
	return outcome, nil
}
