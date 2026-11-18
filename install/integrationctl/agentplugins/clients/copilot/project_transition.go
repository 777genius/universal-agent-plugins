package copilot

import (
	"context"
	"errors"
	"fmt"

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
	if err := copilotMutateProjectPlugin(ctx, env.NativeConfig, root,
		request.Plan.PhysicalArtifactID, request.Plan.DeclaredName, true, true); err != nil {
		// Removal can commit while releasing the native writer lock fails.
		// Components have not started yet: never classify this partial
		// transition as a successfully installed new component projection.
		if nativeconfig.IsCommittedCleanup(err) {
			return &shared.NativeEffectError{Effect: domain.NativeEffectUncertain, Err: err}
		}
		return err
	}
	if err := mutation.Apply(ctx); err != nil {
		var observed *shared.NativeEffectError
		if errors.As(err, &observed) && observed.Effect == domain.NativeEffectUnchanged {
			// ProjectComponentMutation proved it did not change native state.
			// Conditional CAS restores only our original exact names.
			restore := copilotMutateProjectPlugin(ctx, env.NativeConfig, root,
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
