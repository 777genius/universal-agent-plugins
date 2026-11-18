package copilot

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

const copilotProjectPluginKind = "copilot_project_plugin_registration"

// Whole-plugin project registration is limited to admitted Agent Plugins 1.0.
// Existing loose-component bindings retain their previous behavior and never
// silently migrate across two independent native configuration surfaces.
func copilotWholePluginEligible(in clients.PlanInput, plan domain.DeliveryPlan) bool {
	if !copilotSupportsProjectPlugin(in.Client.Version) || in.Envelope.FormatID != domain.FormatIDAgentPluginsV1 || len(plan.Components) == 0 ||
		in.Envelope.App.Present || len(in.Envelope.Commands) != 0 || len(in.Envelope.InvalidCommands) != 0 ||
		len(in.Envelope.MCP.InvalidServer) != 0 || len(in.Envelope.Inventory.InvalidSkills) != 0 ||
		in.Envelope.Inventory.InvalidSkillsRoot {
		return false
	}
	for _, diagnostic := range in.Envelope.Diagnostics {
		if diagnostic.Severity == domain.SeverityError {
			return false
		}
	}
	// A full native plugin exposes every bundled component. Never use this
	// path when the plan filtered or rejected even one source component.
	if len(plan.Components) != len(in.Envelope.Skills)+len(in.Envelope.MCP.Servers)+len(in.Envelope.Manifest.Extensions) {
		return false
	}
	for _, component := range plan.Components {
		if component.Support != domain.SupportNative {
			return false
		}
		switch component.Kind {
		case domain.ComponentSkill:
			if _, ok := in.Envelope.Skills[component.Name]; !ok {
				return false
			}
		case domain.ComponentMCPServer:
			if _, ok := in.Envelope.MCP.Servers[component.Name]; !ok {
				return false
			}
		case domain.ComponentExtension:
			if _, ok := in.Envelope.Manifest.Extensions[component.Name]; !ok {
				return false
			}
		default:
			return false
		}
	}
	for _, object := range in.PreviousNativeObjects {
		if object.Kind != "managed_package_directory" && object.Kind != copilotProjectPluginKind {
			return false
		}
	}
	return true
}

func copilotProjectPluginSettingsPath(root string) (string, error) {
	path := filepath.Join(root, ".github", "copilot", "settings.local.json")
	if err := pathpolicy.RequireContainedChild(root, path); err != nil {
		return "", err
	}
	return path, nil
}

func copilotProjectRegistration(root, physicalID, name string) (nativeconfig.CopilotPluginRegistration, string, error) {
	if err := pathpolicy.ValidateLeafID(physicalID); err != nil {
		return nativeconfig.CopilotPluginRegistration{}, "", err
	}
	path, err := copilotProjectPluginSettingsPath(root)
	if err != nil {
		return nativeconfig.CopilotPluginRegistration{}, "", err
	}
	registration := nativeconfig.CopilotPluginRegistration{
		Marketplace: shared.ManagedMarketplaceName(physicalID),
		Plugin: name,
		Directory: filepath.ToSlash(filepath.Join(".uap", "managed", "clients", "copilot", physicalID)),
	}
	if err := registration.Validate(); err != nil {
		return nativeconfig.CopilotPluginRegistration{}, "", err
	}
	return registration, path, nil
}

func copilotProjectPluginObject(plan domain.DeliveryPlan) (domain.NativeObjectOwnership, error) {
	root, err := copilotPlanRoot(domain.DetectedClient{ConfigRoot: plan.TargetContext.UserConfigRoot}, plan)
	if err != nil {
		return domain.NativeObjectOwnership{}, err
	}
	return copilotProjectPluginObjectAt(root, plan.PhysicalArtifactID, plan.DeclaredName)
}

func copilotProjectPluginObjectAt(root, physicalID, name string) (domain.NativeObjectOwnership, error) {
	reg, path, err := copilotProjectRegistration(root, physicalID, name)
	if err != nil {
		return domain.NativeObjectOwnership{}, err
	}
	encoded, err := json.Marshal(reg)
	if err != nil {
		return domain.NativeObjectOwnership{}, err
	}
	sum := sha256.Sum256(append([]byte("copilot-project-registration-v1\x00"), encoded...))
	return domain.NativeObjectOwnership{
		ObjectID: copilotProjectPluginKind + ":" + reg.Plugin + "@" + reg.Marketplace,
		Kind: copilotProjectPluginKind, LogicalName: reg.Plugin + "@" + reg.Marketplace,
		Path: path, ManagedDigest: "sha256:" + hex.EncodeToString(sum[:]),
		ProtectionClass: "managed",
	}, nil
}

func copilotHasProjectPlugin(objects []domain.NativeObjectOwnership) bool {
	for _, object := range objects {
		if object.Kind == copilotProjectPluginKind {
			return true
		}
	}
	return false
}

func copilotValidateProjectPluginObjects(root string, physicalID string, objects []domain.NativeObjectOwnership) error {
	var registered []domain.NativeObjectOwnership
	for _, object := range objects {
		if object.Kind == "managed_package_directory" {
			continue
		}
		if object.Kind != copilotProjectPluginKind {
			return fmt.Errorf("Copilot project plugin registration cannot mix loose native components")
		}
		registered = append(registered, object)
	}
	if len(registered) != 1 {
		return fmt.Errorf("Copilot project plugin requires exactly one owned registration")
	}
	marketplace := shared.ManagedMarketplaceName(physicalID)
	suffix := "@" + marketplace
	if !strings.HasSuffix(registered[0].LogicalName, suffix) {
		return fmt.Errorf("Copilot project plugin marketplace differs from binding")
	}
	plugin := strings.TrimSuffix(registered[0].LogicalName, suffix)
	// Receipt reconstruction uses only canonical binding facts; it does not
	// fabricate an ambient user profile or trust authority.
	expected, err := copilotProjectPluginObjectAt(root, physicalID, plugin)
	if err != nil {
		return err
	}
	if registered[0] != expected {
		return fmt.Errorf("Copilot project plugin ownership differs from selected registration")
	}
	return nil
}

func copilotPluginSettingsSnapshot(kernel nativeconfig.Kernel, path string) (nativeconfig.FileSnapshot, error) {
	file, err := kernel.ReadExactFileBounded(path, 1<<20)
	if err != nil {
		return file, err
	}
	if file.Exists && len(file.Body) == 0 {
		return file, fmt.Errorf("Copilot project settings is an empty existing file")
	}
	return file, nil
}

func copilotInspectWholePlugin(plan domain.DeliveryPlan, managed *domain.ClientBinding, kernel nativeconfig.Kernel) (clients.RegistryFinding, error) {
	root, err := copilotPlanRoot(domain.DetectedClient{ConfigRoot: plan.TargetContext.UserConfigRoot}, plan)
	if err != nil {
		return clients.RegistryIndeterminate, err
	}
	registration, path, err := copilotProjectRegistration(root, plan.PhysicalArtifactID, plan.DeclaredName)
	if err != nil {
		return clients.RegistryIndeterminate, err
	}
	if managed != nil {
		if managed.Scope != string(domain.ScopeProject) || managed.PhysicalArtifact != plan.PhysicalArtifactID ||
			managed.TargetLocator != plan.ActivePath || managed.NativeProfileRoot != "" && managed.NativeProfileRoot != root {
			return clients.RegistryIndeterminate, fmt.Errorf("Copilot project plugin binding differs from selected package")
		}
		if err := copilotValidateProjectPluginObjects(root, plan.PhysicalArtifactID, managed.NativeObjects); err != nil {
			return clients.RegistryIndeterminate, err
		}
		want, err := copilotProjectPluginObject(plan)
		if err != nil {
			return clients.RegistryIndeterminate, err
		}
		for _, object := range managed.NativeObjects {
			if object.Kind == copilotProjectPluginKind && object != want {
				return clients.RegistryIndeterminate, fmt.Errorf("Copilot project plugin binding name differs from selected package")
			}
		}
	}
	file, err := copilotPluginSettingsSnapshot(kernel, path)
	if err != nil {
		return clients.RegistryIndeterminate, err
	}
	present, exact, err := nativeconfig.InspectCopilotPluginSettings(file.Body, registration)
	if err != nil {
		return clients.RegistryIndeterminate, err
	}
	if managed == nil {
		if present {
			return clients.RegistryCollision, nil
		}
		return clients.RegistryClear, nil
	}
	if !exact {
		return clients.RegistryIndeterminate, fmt.Errorf("Copilot owned project plugin settings were changed or removed")
	}
	return clients.RegistryExpected, nil
}

func copilotVerifyWholePlugin(env clients.Env, request domain.ActivationRequest) error {
	finding, err := copilotInspectWholePlugin(request.Plan, &domain.ClientBinding{
		Scope: string(domain.ScopeProject), PhysicalArtifact: request.Plan.PhysicalArtifactID,
		TargetLocator: request.Plan.ActivePath, NativeProfileRoot: request.Plan.NativeRegistryRoot,
		NativeObjects: request.Delivery.NativeObjects,
	}, env.NativeConfig)
	if err != nil {
		return err
	}
	if finding != clients.RegistryExpected {
		return fmt.Errorf("Copilot project plugin registration is not installed")
	}
	return verifyCopilotTrust(env, request, true)
}

func copilotPreflightWholePlugin(env clients.Env, request domain.ActivationRequest) error {
	root, err := copilotPlanRoot(request.Client, request.Plan)
	if err != nil {
		return err
	}
	if err := env.NativeConfig.RequireFileIO(); err != nil {
		return err
	}
	reg, path, err := copilotProjectRegistration(root, request.Plan.PhysicalArtifactID, request.Plan.DeclaredName)
	if err != nil {
		return err
	}
	if err := copilotValidateProjectPluginObjects(root, request.Plan.PhysicalArtifactID, request.Delivery.NativeObjects); err != nil {
		return err
	}
	if len(request.PreviousNativeObjects) > 0 && !copilotHasProjectPlugin(request.PreviousNativeObjects) {
		return fmt.Errorf("Copilot project plugin cannot adopt a loose-component binding")
	}
	if copilotHasProjectPlugin(request.PreviousNativeObjects) {
		if err := copilotValidateProjectPluginObjects(root, request.Plan.PhysicalArtifactID, request.PreviousNativeObjects); err != nil {
			return err
		}
	}
	file, err := copilotPluginSettingsSnapshot(env.NativeConfig, path)
	if err != nil {
		return err
	}
	_, _, err = nativeconfig.PatchCopilotPluginSettings(file.Body, reg, copilotHasProjectPlugin(request.PreviousNativeObjects), false)
	return err
}

// The exact-file kernel provides native writer locks, no-follow reads, CAS,
// readback and conditional rollback. Do not make direct settings writes here.
func copilotMutateProjectPlugin(ctx context.Context, kernel nativeconfig.Kernel, root, physicalID, name string, owned, remove bool) error {
	if err := ctx.Err(); err != nil {
		return &shared.NativeEffectError{Effect: domain.NativeEffectUnchanged, Err: err}
	}
	reg, path, err := copilotProjectRegistration(root, physicalID, name)
	if err != nil {
		return &shared.NativeEffectError{Effect: domain.NativeEffectUnchanged, Err: err}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return &shared.NativeEffectError{Effect: domain.NativeEffectUnchanged, Err: err}
	}
	file, err := kernel.BeginExactFile(path)
	if err != nil {
		return &shared.NativeEffectError{Effect: domain.NativeEffectUnchanged, Err: err}
	}
	original := file.Original()
	if original.Exists && len(original.Body) == 0 {
		return &shared.NativeEffectError{Effect: domain.NativeEffectUnchanged,
			Err: errors.Join(fmt.Errorf("Copilot project settings is an empty existing file"), file.Close())}
	}
	next, changed, err := nativeconfig.PatchCopilotPluginSettings(original.Body, reg, owned, remove)
	if err == nil {
		err = ctx.Err()
	}
	if err != nil {
		return &shared.NativeEffectError{Effect: domain.NativeEffectUnchanged, Err: errors.Join(err, file.Close())}
	}
	if changed {
		if err := file.Apply(next); err != nil {
			rollback := file.Rollback()
			closeErr := file.Close()
			effect := domain.NativeEffectUnchanged
			if rollback != nil || file.Effect() != nativeconfig.FileUnchanged {
				effect = domain.NativeEffectUncertain
			}
			return &shared.NativeEffectError{Effect: effect, Err: errors.Join(err, rollback, closeErr)}
		}
	}
	closeErr := file.Close()
	if closeErr != nil {
		if changed {
			return &nativeconfig.CommittedCleanupError{Err: closeErr}
		}
		return &shared.NativeEffectError{Effect: domain.NativeEffectUnchanged, Err: closeErr}
	}
	return nil
}

func activateCopilotProjectPlugin(ctx context.Context, env clients.Env, request domain.ActivationRequest, root string) (domain.ActivationOutcome, error) {
	outcome, err := shared.CompleteNativeConfigActivation(ctx, request, shared.NativeConfigActivation{
		Automatic: request.Plan.InstallIntent == domain.InstallIntentAutomatic && request.Plan.NativePluginRegistration,
		UnavailableAction: "select an admitted Agent Plugins 1.0 package for native project registration",
		RepairAction: "repair the exact UAP-owned Copilot project marketplace registration",
		RetryAction: "retry Copilot project plugin registration",
		CompletedAction: "whole Agent Plugin registered for this project without copying its components; open a new Copilot session to load it",
		Verify: func() error {
			return copilotVerifyWholePlugin(env, request)
		},
		Activate: func(ctx context.Context, request domain.ActivationRequest) error {
			register := func() error {
				return copilotMutateProjectPlugin(ctx, env.NativeConfig, root,
					request.Plan.PhysicalArtifactID, request.Plan.DeclaredName,
					copilotHasProjectPlugin(request.PreviousNativeObjects), false)
			}
			if request.ProjectTrustConsent {
				return withCopilotTrust(ctx, env, request, register)
			}
			// A repair or dependency sync may use existing trust, but must not
			// grant permanent trust anew after the user revoked it.
			if err := verifyCopilotTrust(env, request, true); err != nil {
				return &shared.NativeEffectError{Effect: domain.NativeEffectUnchanged, Err: err}
			}
			return register()
		},
	})
	if err == nil && outcome.Verification == domain.VerificationInstalled {
		// Settings and source integrity are proven here; Copilot's runtime
		// discovery still requires a new session and a compatible native CLI.
		outcome.Activation, outcome.ActivationAttested = domain.ActivationPrepared, false
	}
	return outcome, err
}
