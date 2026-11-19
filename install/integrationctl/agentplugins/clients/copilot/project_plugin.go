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

const copilotWholePluginInstalledAction = "whole Agent Plugin registered for this project without copying its components; open a new Copilot session to load it"

// Whole-plugin project registration is limited to admitted Agent Plugins 1.0.
// Existing loose-component bindings retain their previous behavior and never
// silently migrate across two independent native configuration surfaces.
func copilotWholePluginEligible(in clients.PlanInput, plan domain.DeliveryPlan) bool {
	if !copilotWholePluginContentEligible(in.Envelope, plan.Components) {
		return false
	}
	// Native capability was already established for a prior UAP-owned pair.
	// A failed optional version probe must not migrate an unchanged package
	// back to loose skills/MCP; new installations still require host evidence.
	if !copilotSupportsProjectPlugin(in.Client.Version) && !copilotHasProjectPlugin(in.PreviousNativeObjects) {
		return false
	}
	// Never silently convert an existing loose-component binding.
	for _, object := range in.PreviousNativeObjects {
		if object.Kind != "managed_package_directory" && object.Kind != copilotProjectPluginKind {
			return false
		}
	}
	return true
}

func copilotWholePluginContentEligible(envelope domain.PackageEnvelope, components []domain.ComponentDecision) bool {
	if !copilotWholePluginSourceEligible(envelope) || !copilotAllPluginComponentsNative(envelope, components) {
		return false
	}
	// Managed stdio contracts require UAP's launcher and cannot be
	// reinterpreted as native Copilot plugin command execution.
	for _, server := range envelope.MCP.Servers {
		if requirement := server.StdioRequirement; requirement != nil &&
			(requirement.Kind == domain.ExecutableBundled || requirement.UsesPluginData) {
			return false
		}
	}
	return true
}

func copilotWholePluginSourceEligible(envelope domain.PackageEnvelope) bool {
	if envelope.FormatID != domain.FormatIDAgentPluginsV1 || envelope.App.Present ||
		len(envelope.Commands) != 0 || len(envelope.InvalidCommands) != 0 ||
		len(envelope.MCP.InvalidServer) != 0 || len(envelope.Inventory.InvalidSkills) != 0 ||
		envelope.Inventory.InvalidSkillsRoot {
		return false
	}
	for _, diagnostic := range envelope.Diagnostics {
		if diagnostic.Severity == domain.SeverityError {
			return false
		}
	}
	return true
}

func copilotAllPluginComponentsNative(envelope domain.PackageEnvelope, components []domain.ComponentDecision) bool {
	if len(components) == 0 ||
		len(components) != len(envelope.Skills)+len(envelope.MCP.Servers)+len(envelope.Manifest.Extensions) {
		return false
	}
	for _, component := range components {
		if component.Support != domain.SupportNative || !copilotNativeComponentExists(envelope, component) {
			return false
		}
	}
	return true
}

func copilotNativeComponentExists(envelope domain.PackageEnvelope, component domain.ComponentDecision) bool {
	switch component.Kind {
	case domain.ComponentSkill:
		_, ok := envelope.Skills[component.Name]
		return ok
	case domain.ComponentMCPServer:
		_, ok := envelope.MCP.Servers[component.Name]
		return ok
	case domain.ComponentExtension:
		_, ok := envelope.Manifest.Extensions[component.Name]
		return ok
	default:
		return false
	}
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
		Plugin:      name,
		Directory:   filepath.ToSlash(filepath.Join(".uap", "managed", "clients", "copilot", physicalID)),
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
		Kind:     copilotProjectPluginKind, LogicalName: reg.Plugin + "@" + reg.Marketplace,
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
			return fmt.Errorf("copilot project plugin registration cannot mix loose native components")
		}
		registered = append(registered, object)
	}
	if len(registered) != 1 {
		return fmt.Errorf("copilot project plugin requires exactly one owned registration")
	}
	marketplace := shared.ManagedMarketplaceName(physicalID)
	suffix := "@" + marketplace
	if !strings.HasSuffix(registered[0].LogicalName, suffix) {
		return fmt.Errorf("copilot project plugin marketplace differs from binding")
	}
	plugin := strings.TrimSuffix(registered[0].LogicalName, suffix)
	// Receipt reconstruction uses only canonical binding facts; it does not
	// fabricate an ambient user profile or trust authority.
	expected, err := copilotProjectPluginObjectAt(root, physicalID, plugin)
	if err != nil {
		return err
	}
	if registered[0] != expected {
		return fmt.Errorf("copilot project plugin ownership differs from selected registration")
	}
	return nil
}

// Copilot's saved object receipt must also bind the exact declared plugin
// name. A same-marketplace request naming another plugin must never report a
// no-op native removal and retire the still-registered package directory.
func copilotValidateProjectPluginName(root, physicalID, declaredName string, objects []domain.NativeObjectOwnership) error {
	if err := copilotValidateProjectPluginObjects(root, physicalID, objects); err != nil {
		return err
	}
	expected, err := copilotProjectPluginObjectAt(root, physicalID, declaredName)
	if err != nil {
		return err
	}
	for _, object := range objects {
		if object.Kind == copilotProjectPluginKind {
			if object != expected {
				return fmt.Errorf("copilot native project plugin identity differs from saved ownership")
			}
			return nil
		}
	}
	return fmt.Errorf("copilot native project plugin receipt is missing")
}

func copilotPluginSettingsSnapshot(kernel nativeconfig.Kernel, path string) (nativeconfig.FileSnapshot, error) {
	file, err := kernel.ReadExactFileBounded(path, 1<<20)
	if err != nil {
		return file, err
	}
	if file.Exists && len(file.Body) == 0 {
		return file, fmt.Errorf("copilot project settings is an empty existing file")
	}
	return file, nil
}

// A settings entry alone cannot attest to a usable native plugin. The staged
// package must still contain an ordinary Agent Plugins manifest and a matching
// local marketplace that points only at its own root.
func copilotVerifyProjectPluginSource(root string, plan domain.DeliveryPlan, kernel nativeconfig.Kernel) error {
	manifestPath := filepath.Join(plan.ActivePath, "plugin.json")
	marketplacePath := filepath.Join(plan.ActivePath, ".github", "plugin", "marketplace.json")
	read := func(path string) (map[string]any, error) {
		if err := pathpolicy.RequireContainedChild(root, path); err != nil {
			return nil, err
		}
		// Use the shared bounded, no-follow native file reader. Lstat followed
		// by ReadFile could follow a replaced symlink between the two calls.
		file, err := kernel.ReadExactFileBounded(path, 1<<20)
		if err != nil {
			return nil, err
		}
		if !file.Exists || len(file.Body) < 2 {
			return nil, fmt.Errorf("copilot native plugin document is absent or empty")
		}
		return shared.DecodeStrictJSONObject(file.Body)
	}
	manifest, err := read(manifestPath)
	if err != nil {
		return fmt.Errorf("copilot project Agent Plugins manifest: %w", err)
	}
	if manifest["name"] != plan.DeclaredName || manifest["$schema"] != domain.PluginSchemaV1 {
		return fmt.Errorf("copilot project plugin manifest must match the selected Agent Plugins 1.0 identity and schema")
	}
	marketplace, err := read(marketplacePath)
	if err != nil {
		return fmt.Errorf("copilot project marketplace: %w", err)
	}
	if marketplace["name"] != shared.ManagedMarketplaceName(plan.PhysicalArtifactID) {
		return fmt.Errorf("copilot project marketplace name differs from the selected package")
	}
	plugins, ok := marketplace["plugins"].([]any)
	if !ok || len(plugins) != 1 {
		return fmt.Errorf("copilot project marketplace must contain exactly one local plugin")
	}
	entry, ok := plugins[0].(map[string]any)
	if !ok || entry["name"] != plan.DeclaredName || entry["source"] != "." {
		return fmt.Errorf("copilot project marketplace must point at the selected plugin root")
	}
	return nil
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
			return clients.RegistryIndeterminate, fmt.Errorf("copilot project plugin binding differs from selected package")
		}
		if err := copilotValidateProjectPluginName(root, plan.PhysicalArtifactID, plan.DeclaredName, managed.NativeObjects); err != nil {
			return clients.RegistryIndeterminate, err
		}
		if err := copilotVerifyProjectPluginSource(root, plan, kernel); err != nil {
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
	if !present {
		// Both entries disappeared. Retain the verified binding authority so
		// repair can re-create this exact registration without adopting a
		// partially replaced or unrelated namespace.
		return clients.RegistryClear, nil
	}
	if !exact {
		return clients.RegistryIndeterminate, fmt.Errorf("copilot owned project plugin settings were changed or partially removed")
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
		return fmt.Errorf("copilot project plugin registration is not installed")
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
	// Group preflight runs before staging: it has the selected plan but no
	// delivery or request-level name/receipts yet. Check the prospective
	// namespace now, and require exact staged receipts at activation time.
	preview := request.Delivery.ClientID == "" && request.Delivery.ActivePath == "" && len(request.Delivery.NativeObjects) == 0
	name, previous := request.DeclaredName, request.PreviousNativeObjects
	if preview {
		if !request.Plan.HasNativePluginRegistration() {
			return fmt.Errorf("copilot native project preview requires a selected whole-plugin plan")
		}
		name, previous = request.Plan.DeclaredName, request.Plan.PreviousNativeObjects
	}
	reg, path, err := copilotProjectRegistration(root, request.Plan.PhysicalArtifactID, name)
	if err != nil {
		return err
	}
	if !preview {
		if err := copilotValidateProjectPluginName(root, request.Plan.PhysicalArtifactID, name, request.Delivery.NativeObjects); err != nil {
			return err
		}
	}
	if len(previous) > 0 && !copilotHasProjectPlugin(previous) {
		return fmt.Errorf("copilot project plugin cannot adopt a loose-component binding")
	}
	if copilotHasProjectPlugin(previous) {
		if err := copilotValidateProjectPluginName(root, request.Plan.PhysicalArtifactID, name, previous); err != nil {
			return err
		}
	}
	file, err := copilotPluginSettingsSnapshot(env.NativeConfig, path)
	if err != nil {
		return err
	}
	_, _, err = nativeconfig.PatchCopilotPluginSettings(file.Body, reg, copilotHasProjectPlugin(previous), false)
	return err
}

// The exact-file kernel provides native writer locks, no-follow reads, CAS,
// readback and conditional rollback. Do not make direct settings writes here.
func copilotMutateProjectPlugin(ctx context.Context, kernel nativeconfig.Kernel, root, physicalID, name string, owned, remove bool) error {
	_, err := copilotMutateProjectPluginResult(ctx, kernel, root, physicalID, name, owned, remove)
	return err
}

// The returned change bit belongs only to a verified committed settings edit;
// an unchanged removal must never grant later compensation ownership.
func copilotMutateProjectPluginResult(ctx context.Context, kernel nativeconfig.Kernel, root, physicalID, name string, owned, remove bool) (bool, error) {
	unchanged := func(err error) (bool, error) {
		return false, &shared.NativeEffectError{Effect: domain.NativeEffectUnchanged, Err: err}
	}
	if err := ctx.Err(); err != nil {
		return unchanged(err)
	}
	reg, path, err := copilotProjectRegistration(root, physicalID, name)
	if err != nil {
		return unchanged(err)
	}
	if !remove {
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			return unchanged(err)
		}
	}
	if err := pathpolicy.RequireContainedChild(root, path); err != nil {
		return unchanged(err)
	}
	file, err := kernel.BeginExactFileBounded(path, 1<<20)
	if err != nil {
		return unchanged(err)
	}
	original := file.Original()
	if original.Exists && len(original.Body) == 0 {
		return unchanged(errors.Join(fmt.Errorf("copilot project settings is an empty existing file"), file.Close()))
	}
	next, changed, err := nativeconfig.PatchCopilotPluginSettings(original.Body, reg, owned, remove)
	if err == nil {
		err = ctx.Err()
	}
	if err != nil {
		return unchanged(errors.Join(err, file.Close()))
	}
	if changed {
		if err := file.Apply(next); err != nil {
			rollback := file.Rollback()
			closeErr := file.Close()
			effect := domain.NativeEffectUnchanged
			if rollback != nil || file.Effect() != nativeconfig.FileUnchanged {
				effect = domain.NativeEffectUncertain
			}
			return false, &shared.NativeEffectError{Effect: effect, Err: errors.Join(err, rollback, closeErr)}
		}
	}
	if err := file.Close(); err != nil {
		if changed {
			return true, &nativeconfig.CommittedCleanupError{Err: err}
		}
		return unchanged(err)
	}
	return changed, nil
}

func copilotPluginNativeObjects(objects []domain.NativeObjectOwnership) []domain.NativeObjectOwnership {
	selected := make([]domain.NativeObjectOwnership, 0, 1)
	for _, object := range objects {
		if object.Kind == copilotProjectPluginKind {
			selected = append(selected, object)
		}
	}
	return selected
}

func activateCopilotProjectPlugin(ctx context.Context, env clients.Env, request domain.ActivationRequest, root string) (domain.ActivationOutcome, error) {
	// An intact saved registration needs verification, not a second native
	// mutation. Reuse the same lifecycle policy as Cursor and OpenCode.
	projected := request
	previous, desired := copilotPluginNativeObjects(request.PreviousNativeObjects), copilotPluginNativeObjects(request.Delivery.NativeObjects)
	if shared.IntactNativeReplacement(request, previous, desired, func() error {
		return copilotVerifyWholePlugin(env, request)
	}) {
		projected.VerifyOnly = true
	}
	outcome, err := shared.CompleteNativeConfigActivation(ctx, projected, shared.NativeConfigActivation{
		Automatic:         request.Plan.InstallIntent == domain.InstallIntentAutomatic && (request.Plan.HasNativePluginRegistration() || copilotHasProjectPlugin(request.PreviousNativeObjects) || copilotHasProjectPlugin(request.Delivery.NativeObjects)),
		UnavailableAction: "select an admitted Agent Plugins 1.0 package for native project registration",
		RepairAction:      "repair the exact UAP-owned Copilot project marketplace registration",
		RetryAction:       "retry Copilot project plugin registration",
		CompletedAction:   copilotWholePluginInstalledAction,
		Verify: func() error {
			return copilotVerifyWholePlugin(env, request)
		},
		Activate: func(ctx context.Context, request domain.ActivationRequest) error {
			// Preflight only checks native settings: the selected package becomes
			// readable after staging commits. Verify the installed manifest and
			// marketplace immediately before touching project/user settings.
			if err := copilotVerifyProjectPluginSource(root, request.Plan, env.NativeConfig); err != nil {
				return &shared.NativeEffectError{Effect: domain.NativeEffectUnchanged, Err: err}
			}
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
		// Registration alone stays Prepared. Pure Skills may additionally prove
		// ordinary runtime discovery; mixed packages retain session guidance.
		outcome.Activation, outcome.ActivationAttested = domain.ActivationPrepared, false
		return observeCopilotWholePluginSkills(ctx, env, request, outcome)
	}
	return outcome, err
}
