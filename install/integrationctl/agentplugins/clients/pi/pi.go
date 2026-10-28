// Package pi materializes a managed package directly in Pi's extension discovery
// directory. The shared directory transaction owns every filesystem effect.
package pi

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

const Namespace = "com.universal-agent-plugins.pi"
const DescriptorName = ".agentplugins-pi.json"
const BootstrapVersion = 1

type Adapter struct{}

func New() *Adapter                  { return &Adapter{} }
func (*Adapter) ID() domain.ClientID { return domain.ClientPi }

var (
	_ clients.HostDetector              = (*Adapter)(nil)
	_ clients.ProfileResolver           = (*Adapter)(nil)
	_ clients.ProfileBindingValidator   = (*Adapter)(nil)
	_ clients.VersionProbeEnvironment   = (*Adapter)(nil)
	_ clients.TargetLayout              = (*Adapter)(nil)
	_ clients.StagingLayout             = (*Adapter)(nil)
	_ clients.ExtensionInterpreter      = (*Adapter)(nil)
	_ clients.PlanPrecondition          = (*Adapter)(nil)
	_ clients.PlanRefiner               = (*Adapter)(nil)
	_ clients.Projector                 = (*Adapter)(nil)
	_ clients.ActiveNativeProjector     = (*Adapter)(nil)
	_ clients.Lifecycle                 = (*Adapter)(nil)
	_ clients.ActivationPreflighter     = (*Adapter)(nil)
	_ clients.AutomaticActivator        = (*Adapter)(nil)
	_ clients.ReadOnlyVerifier          = (*Adapter)(nil)
	_ clients.RegistryInspector         = (*Adapter)(nil)
	_ clients.PreparedRegistryInspector = (*Adapter)(nil)
)

func cleanRoot(root string) error {
	if root == "" || !utf8.ValidString(root) || strings.TrimSpace(root) != root || strings.IndexFunc(root, unicode.IsControl) >= 0 || !filepath.IsAbs(root) || filepath.Clean(root) != root || filepath.Dir(root) == root {
		return fmt.Errorf("Pi agent directory must be a clean absolute directory")
	}
	return nil
}

func (*Adapter) ResolveProfileRoot(root string) (string, error) {
	if err := cleanRoot(root); err != nil {
		return "", err
	}
	return clients.CanonicalDirectory(root, os.Lstat, filepath.EvalSymlinks)
}

func (*Adapter) DetectSurfaces(host clients.Host) clients.Detection {
	root := host.Env("PI_CODING_AGENT_DIR")
	if root == "" {
		root = filepath.Join(host.HomeDir(), ".pi", "agent")
	}
	if root == "~" {
		root = host.HomeDir()
	} else if strings.HasPrefix(root, "~/") || host.GOOS() == "windows" && strings.HasPrefix(root, `~\`) {
		root = filepath.Join(host.HomeDir(), root[2:])
	}
	if !filepath.IsAbs(root) {
		if filepath.VolumeName(root) != "" || !filepath.IsAbs(host.WorkingDir()) {
			return clients.Detection{Err: fmt.Errorf("relative Pi agent directory requires an unambiguous original working directory")}
		}
		root = filepath.Join(host.WorkingDir(), root)
	}
	if err := cleanRoot(root); err != nil {
		return clients.Detection{Err: err}
	}
	root, err := host.CanonicalDirectory(root)
	if err != nil {
		return clients.Detection{Err: err}
	}
	executable := host.LookPath("pi")
	return clients.Detection{ConfigRoot: root, ExecutablePath: executable, Surfaces: []domain.ClientSurface{host.ResolvedBinarySurface("pi_cli", executable), host.DirectorySurface("pi_config", root)}}
}

func (*Adapter) VersionProbeEnvironment(root string) ([]string, error) {
	if err := validateProfile(root); err != nil {
		return nil, err
	}
	return []string{"PI_CODING_AGENT_DIR=" + root}, nil
}

func validateProfile(root string) error {
	actual, err := New().ResolveProfileRoot(root)
	if err != nil {
		return err
	}
	if actual != root {
		return fmt.Errorf("Pi profile must be canonical before mutation")
	}
	return nil
}

func (*Adapter) TargetRoot(client domain.DetectedClient, _ domain.PackageMode, _ string) (string, string, error) {
	if err := validateProfile(client.ConfigRoot); err != nil {
		return "", "", err
	}
	return client.ConfigRoot, filepath.Join(client.ConfigRoot, "extensions"), nil
}
func (*Adapter) StagingBase(plan domain.DeliveryPlan) string         { return plan.TargetAnchor }
func (*Adapter) ValidateTargetLayout(plan domain.DeliveryPlan) error { return validatePlan(plan) }

func validatePlan(plan domain.DeliveryPlan) error {
	if plan.ClientID != domain.ClientPi || plan.Scope != domain.ScopeUser || plan.TargetRoot != filepath.Join(plan.TargetAnchor, "extensions") || plan.NativeRegistryRoot != plan.TargetAnchor || plan.ActivePath != filepath.Join(plan.TargetRoot, plan.PhysicalArtifactID) {
		return fmt.Errorf("Pi package layout differs from selected user profile")
	}
	if err := validateProfile(plan.TargetAnchor); err != nil {
		return err
	}
	return pathpolicy.RequireContainedChild(plan.TargetRoot, plan.ActivePath)
}

func (*Adapter) CheckPlanPrecondition(in clients.PlanInput, _ *domain.DeliveryPlan) error {
	if err := validateProfile(in.Client.ConfigRoot); err != nil {
		return err
	}
	return checkDiscoveryRoot(filepath.Join(in.Client.ConfigRoot, "extensions"))
}

func (*Adapter) RefinePlan(_ context.Context, in clients.PlanInput, plan *domain.DeliveryPlan) error {
	shared.PromoteNativeReady(plan, in.Client.ConfigRoot, shared.OnlyNativeComponents(plan.Components))
	plan.UserActions = shared.AppendUnique(plan.UserActions, "Reload resources or restart Pi to load the managed extension; installed files do not prove runtime activation.")
	return nil
}

func (*Adapter) ValidateBindingProfile(root string, binding domain.ClientBinding) error {
	if err := validateProfile(root); err != nil {
		return err
	}
	if binding.ClientID != string(domain.ClientPi) || binding.Scope != string(domain.ScopeUser) || binding.NativeProfileRoot != root || filepath.Dir(binding.TargetLocator) != filepath.Join(root, "extensions") {
		return fmt.Errorf("Pi binding differs from recorded physical profile")
	}
	return pathpolicy.RequireContainedChild(root, binding.TargetLocator)
}

func (*Adapter) UsesNativeRegistryExecutable() bool { return false }
func (*Adapter) InspectNativeRegistry(ctx context.Context, _ clients.Env, _ domain.DetectedClient, plan domain.DeliveryPlan, managed *domain.ClientBinding) (clients.RegistryFinding, error) {
	if err := ctx.Err(); err != nil {
		return clients.RegistryIndeterminate, err
	}
	return inspectPackage(plan, managed != nil && managed.TargetLocator == plan.ActivePath)
}
func (*Adapter) InspectPreparedRegistry(plan domain.DeliveryPlan, _ string, owned bool) (clients.RegistryFinding, error) {
	return inspectPackage(plan, owned)
}
func inspectPackage(plan domain.DeliveryPlan, owned bool) (clients.RegistryFinding, error) {
	if err := validatePlan(plan); err != nil {
		return clients.RegistryIndeterminate, err
	}
	if err := checkDiscoveryRoot(plan.TargetRoot); err != nil {
		return clients.RegistryIndeterminate, err
	}
	info, err := os.Lstat(plan.ActivePath)
	if os.IsNotExist(err) {
		return clients.RegistryClear, nil
	}
	if err != nil {
		return clients.RegistryIndeterminate, err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return clients.RegistryCollision, nil
	}
	if !owned {
		return clients.RegistryCollision, nil
	}
	return clients.RegistryExpected, nil
}

func (*Adapter) AutomaticallyActivates(_ clients.Env, request domain.ActivationRequest) bool {
	return request.Plan.InstallIntent == domain.InstallIntentAutomatic
}
func (*Adapter) VerifierAvailable(_ domain.DetectedClient, plan domain.DeliveryPlan, _ string) bool {
	return validatePlan(plan) == nil
}
func (*Adapter) PreflightActivation(_ clients.Env, request domain.ActivationRequest) error {
	if err := validatePlan(request.Plan); err != nil {
		return err
	}
	if request.Client.ClientID != domain.ClientPi || request.Client.ConfigRoot != request.Plan.TargetAnchor {
		return fmt.Errorf("Pi activation profile differs")
	}
	return checkDiscoveryRoot(request.Plan.TargetRoot)
}
func (a *Adapter) Activate(ctx context.Context, env clients.Env, request domain.ActivationRequest) (domain.ActivationOutcome, error) {
	out := shared.StartedActivation(request)
	if err := ctx.Err(); err != nil {
		return out, err
	}
	if err := a.PreflightActivation(env, request); err != nil {
		return out, err
	}
	if _, err := a.ProjectActiveNative(ctx, request.Delivery.ActivePath, domain.PackageEnvelope{}, request.Plan, ""); err != nil {
		return out, err
	}
	out.Activation, out.Verification = domain.ActivationActive, domain.VerificationInstalled
	out.NativeObjects = append([]domain.NativeObjectOwnership(nil), request.Delivery.NativeObjects...)
	out.NativeEffect = domain.NativeEffectUnchanged
	out.UserActions = []string{"Reload resources or restart Pi to load the installed extension."}
	return out, nil
}
func (*Adapter) Deactivate(ctx context.Context, _ clients.Env, request domain.DeactivationRequest) (domain.DeactivationOutcome, error) {
	out := shared.StartedDeactivation()
	if err := ctx.Err(); err != nil {
		return out, err
	}
	if request.Client.ClientID != domain.ClientPi || validateProfile(request.NativeRegistryRoot) != nil || filepath.Dir(request.ManagedArtifactPath) != filepath.Join(request.NativeRegistryRoot, "extensions") {
		return out, fmt.Errorf("Pi removal profile differs")
	}
	out.ExternalRemovalComplete = true // The common directory transaction performs removal.
	return out, nil
}

func checkDiscoveryRoot(root string) error {
	for _, name := range []string{"index.ts", "index.js"} {
		if _, err := os.Lstat(filepath.Join(root, name)); err == nil {
			return fmt.Errorf("Pi extensions root contains a foreign entrypoint masking child discovery")
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	return rejectPiManifest(root)
}

func rejectPiManifest(root string) error {
	path := filepath.Join(root, "package.json")
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Size() > 65536 {
		return fmt.Errorf("Pi package manifest is not a bounded regular file")
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil {
		return fmt.Errorf("Pi package manifest is unreadable: %w", err)
	}
	if _, exists := fields["pi"]; exists {
		return fmt.Errorf("Pi package manifest overrides selected extension discovery")
	}
	return nil
}
