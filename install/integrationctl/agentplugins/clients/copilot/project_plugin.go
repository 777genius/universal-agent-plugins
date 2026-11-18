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
	if in.Envelope.FormatID != domain.FormatIDAgentPluginsV1 || len(plan.Components) == 0 {
		return false
	}
	for _, component := range plan.Components {
		if component.Support != domain.SupportNative || component.Kind == domain.ComponentApp {
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
		if managed.Scope != string(domain.ScopeProject) || managed.PhysicalArtifact != plan.PhysicalArtifactID || managed.TargetLocator != plan.ActivePath {
			return clients.RegistryIndeterminate, fmt.Errorf("Copilot project plugin binding differs from selected package")
		}
		if err := copilotValidateProjectPluginObjects(root, plan.PhysicalArtifactID, managed.NativeObjects); err != nil {
			return clients.RegistryIndeterminate, err
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
		TargetLocator: request.Plan.ActivePath, NativeObjects: request.Delivery.NativeObjects,
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
	if len(request.PreviousNativeObjects) > 0 && copilotHasProjectPlugin(request.PreviousNativeObjects) {
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
