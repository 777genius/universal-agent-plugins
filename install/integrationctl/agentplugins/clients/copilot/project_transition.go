package copilot

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// One-directional native-to-component replacement is needed when a new
// package adds managed stdio or another feature unavailable to native
// whole-plugin delivery. The opposite direction is deliberately not an
// implicit migration of a legacy component binding.
func copilotSwitchesToComponents(request domain.ActivationRequest) bool {
	return copilotHasProjectPlugin(request.PreviousNativeObjects) &&
		!copilotHasProjectPlugin(request.Delivery.NativeObjects) &&
		!request.Plan.HasNativePluginRegistration()
}

func copilotPreflightComponentSwitch(env clients.Env, request domain.ActivationRequest, root string) error {
	if err := copilotValidateProjectPluginName(root, request.Plan.PhysicalArtifactID,
		request.Plan.DeclaredName, request.PreviousNativeObjects); err != nil {
		return err
	}
	layout, err := copilotComponents(root)
	if err != nil {
		return err
	}
	if !shared.OnlyNativeComponents(request.Plan.Components) {
		return fmt.Errorf("copilot native-to-component update requires supported skills or MCP components")
	}
	if shared.ComponentKindPresent(request.Plan.Components, domain.ComponentMCPServer) {
		if err := validateCopilotConfig(env.NativeConfig, layout.ConfigPaths.JSON); err != nil {
			return err
		}
	}
	if err := layout.ValidateObjects(copilotNativeObjects(request.Delivery.NativeObjects)); err != nil {
		return err
	}
	registration, path, err := copilotProjectRegistration(root, request.Plan.PhysicalArtifactID, request.Plan.DeclaredName)
	if err != nil {
		return err
	}
	file, err := copilotPluginSettingsSnapshot(env.NativeConfig, path)
	if err != nil {
		return err
	}
	_, _, err = nativeconfig.PatchCopilotPluginSettings(file.Body, registration, true, true)
	return err
}

func copilotApplyComponentSwitch(ctx context.Context, env clients.Env,
	request domain.ActivationRequest, root string, mutation shared.ProjectComponentMutation) error {
	if err := mutation.Preflight(ctx); err != nil {
		return &shared.NativeEffectError{Effect: domain.NativeEffectUnchanged, Err: err}
	}
	if err := verifyCopilotTrust(env, request, true); err != nil {
		return &shared.NativeEffectError{Effect: domain.NativeEffectUnchanged, Err: err}
	}
	// Remove only our exact settings pair before exposing loose skills/MCP.
	removed, err := copilotMutateProjectPluginResult(ctx, env.NativeConfig, root,
		request.Plan.PhysicalArtifactID, request.Plan.DeclaredName, true, true)
	if err != nil {
		// A committed unregister with failed lock cleanup is not proof of a
		// completed switch: new components have not been installed yet.
		if nativeconfig.IsCommittedCleanup(err) {
			return &shared.NativeEffectError{Effect: domain.NativeEffectUncertain, Err: err}
		}
		return err
	}
	if err := mutation.Apply(ctx); err != nil {
		if nativeconfig.IsCommittedCleanup(err) {
			// Components committed; let shared completion record their ownership.
			return err
		}
		var observed *shared.NativeEffectError
		if errors.As(err, &observed) && observed.Effect == domain.NativeEffectUnchanged {
			if !removed {
				return err // User already removed the pair; do not resurrect it.
			}
			// Cancellation of the caller must not cancel compensation. The
			// bounded cleanup context still checks receipt and foreign writes.
			restoreCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			defer cancel()
			restore := copilotMutateProjectPlugin(restoreCtx, env.NativeConfig, root,
				request.Plan.PhysicalArtifactID, request.Plan.DeclaredName, true, false)
			if restore == nil {
				return err
			}
			return &shared.NativeEffectError{Effect: domain.NativeEffectUncertain, Err: errors.Join(err, restore)}
		}
		// A partially committed component transaction has an uncertain effect.
		// Never resurrect the old plugin over potentially active new components.
		return &shared.NativeEffectError{Effect: domain.NativeEffectUncertain, Err: err}
	}
	return nil
}
