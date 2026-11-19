package hermes

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

type hermesSkillMetadata struct {
	path, digest string
}

func hermesSkillObservationAllowed(env clients.Env, request domain.ActivationRequest, outcome domain.ActivationOutcome) bool {
	// The installer facade carries an explicitly selected executable without
	// detector surfaces. Its native protocol must still prove the CLI below.
	if request.VerifyOnly || env.Runner == nil || env.ParseSkill == nil || env.Snapshots == nil || !hermesSkillCLISelected(request.Client) ||
		request.ActivationComplete || request.ConfirmPluginEnable ||
		outcome.Verification != domain.VerificationInstalled || outcome.Policy != domain.PolicyAllowed ||
		outcome.ActivationAttested || outcome.AuthenticationAttested || outcome.NativeEffect == domain.NativeEffectUncertain {
		return false
	}
	return hermesPureSkillPlan(request.Plan)
}

func hermesSkillCLISelected(client domain.DetectedClient) bool {
	return client.Status == domain.DetectionDetected && cleanRoot(client.ExecutablePath) &&
		(len(client.Surfaces) == 0 || hermesCLISurface(client.Surfaces))
}

func hermesPureSkillPlan(plan domain.DeliveryPlan) bool {
	if plan.Scope != plan.TargetContext.Scope || plan.Status != domain.PlanReady ||
		plan.InstallIntent != domain.InstallIntentAutomatic || plan.Policy != domain.PolicyAllowed || len(plan.Components) == 0 {
		return false
	}
	seen := map[string]bool{}
	for _, component := range plan.Components {
		key := shared.FoldJSONKey(component.Name)
		if component.Kind != domain.ComponentSkill || component.Support != domain.SupportNative ||
			component.Source != "" || component.SourceName != "" || !domain.ValidSkillName(component.Name) || seen[key] {
			return false
		}
		seen[key] = true
	}
	return true
}

func completeHermesSkills(ctx context.Context, env clients.Env, request domain.ActivationRequest, outcome domain.ActivationOutcome, action string) (domain.ActivationOutcome, error) {
	if !hermesSkillObservationAllowed(env, request, outcome) {
		return outcome, nil
	}
	before, cwd, err := hermesSkillPolicy(env.NativeConfig, request)
	if err != nil {
		return outcome, nil
	}
	expected, err := hermesOwnedSkills(ctx, env, request)
	if err != nil {
		return outcome, err
	}
	bounded, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	observed, probeErr := observeHermesSkills(bounded, env, request, cwd, expected)
	if domain.IsUnclosedClientProbe(probeErr) {
		outcome.NativeEffect = domain.NativeEffectUncertain
		return outcome, probeErr
	}
	if ctx.Err() != nil {
		return outcome, ctx.Err()
	}
	after, afterCWD, err := hermesSkillPolicy(env.NativeConfig, request)
	if err != nil || cwd != afterCWD || !reflect.DeepEqual(before, after) {
		return outcome, fmt.Errorf("hermes Skill discovery policy changed during native observation")
	}
	afterSkills, err := hermesOwnedSkills(ctx, env, request)
	if err != nil || !reflect.DeepEqual(expected, afterSkills) {
		return outcome, fmt.Errorf("owned Hermes Skill contents changed during native observation")
	}
	if probeErr != nil || !observed {
		return outcome, nil
	}
	outcome.Activation = domain.ActivationActive
	outcome.UserActions = slices.DeleteFunc(outcome.UserActions, func(value string) bool { return value == action })
	if outcome.Authentication == domain.AuthenticationNotChecked &&
		shared.ObservedSkillInstallationAuthentication(request.Plan) == domain.AuthenticationNotRequired {
		outcome.Authentication = domain.AuthenticationNotRequired
	}
	return outcome, nil
}

func hermesSkillPolicy(kernel nativeconfig.Kernel, request domain.ActivationRequest) (map[string]nativeconfig.FileSnapshot, string, error) {
	profile, plan := request.Client.ConfigRoot, request.Plan
	values := hermesEnvironmentValues(os.Environ())
	selected, err := selectedProfileRoot(clients.NewHost(clients.HostProbes{
		HomeDir: hermesHome(values), GOOS: runtime.GOOS, Environment: values,
	}))
	if err == nil {
		selected, err = New().ResolveProfileRoot(selected)
	}
	if err != nil || selected != profile || !cleanRoot(profile) ||
		plan.TargetContext.UserConfigRoot != "" && plan.TargetContext.UserConfigRoot != profile {
		return nil, "", fmt.Errorf("hermes native profile authority differs")
	}
	cwd := filepath.VolumeName(profile) + string(filepath.Separator)
	if plan.Scope == domain.ScopeProject {
		cwd = plan.TargetContext.ProjectRoot
	} else if plan.Scope != domain.ScopeUser || plan.NativeRegistryRoot != profile {
		return nil, "", fmt.Errorf("hermes native Skill scope differs")
	}
	if plan.Scope == domain.ScopeProject && !cleanRoot(cwd) || hermesTrustPolicyRefusal(profile, cwd, values) != "" {
		return nil, "", fmt.Errorf("hermes native Skill policy is unavailable")
	}
	files := map[string]nativeconfig.FileSnapshot{}
	for _, name := range []string{"config.yaml", ".env", ".op.env", "active_profile"} {
		path := filepath.Join(profile, name)
		file, err := kernel.ReadExactFileBounded(path, hermesTrustFileLimit)
		if err != nil {
			return nil, "", err
		}
		files[path] = file
	}
	return files, cwd, nil
}

func hermesOwnedSkills(ctx context.Context, env clients.Env, request domain.ActivationRequest) (map[string]hermesSkillMetadata, error) {
	plan, delivery := request.Plan, request.Delivery
	if err := hermesSkillPackageReceipt(request); err != nil {
		return nil, err
	}
	layout, err := components(plan, delivery.ActivePath)
	if err != nil {
		return nil, err
	}
	objects, err := ownedObjects(plan.Scope, delivery.NativeObjects)
	if err != nil || len(objects) != len(plan.Components) {
		return nil, fmt.Errorf("hermes native Skill receipt coverage differs")
	}
	for _, object := range objects {
		if object.Kind != layout.SkillKind || object.ProtectionClass != "managed" || object.Path != filepath.Join(layout.SkillsRoot, object.LogicalName) ||
			!slices.ContainsFunc(plan.Components, func(component domain.ComponentDecision) bool { return component.Name == object.LogicalName }) {
			return nil, fmt.Errorf("hermes native Skill receipt selection differs")
		}
	}
	if err := layout.Verify(objects, false, env.NativeConfig); err != nil {
		return nil, err
	}
	digest, err := env.Snapshots.Digest(ctx, delivery.ActivePath)
	if err != nil || digest != delivery.ArtifactDigest {
		return nil, fmt.Errorf("hermes Skill package contents differ")
	}
	result := map[string]hermesSkillMetadata{}
	for _, object := range objects {
		path := filepath.Join(object.Path, "SKILL.md")
		body, err := shared.ReadOwnedSkillMetadata(ctx, env.NativeConfig, env.Snapshots, object.Path, object.ManagedDigest)
		if err != nil {
			return nil, fmt.Errorf("hermes owned Skill contents unavailable: %w", err)
		}
		skill, err := env.ParseSkill(ctx, object.LogicalName, body)
		if err != nil || skill.Name != object.LogicalName {
			return nil, fmt.Errorf("hermes owned Skill identity differs")
		}
		result[object.LogicalName] = hermesSkillMetadata{path: path, digest: hermesSkillTextDigest(body)}
	}
	return result, nil
}

// Pinned Hermes reads utf-8-sig text with universal newlines. The catalog's
// captured text must match the text of the sealed, valid UTF-8 owned document.
func hermesSkillTextDigest(body []byte) string {
	text := strings.TrimPrefix(string(body), "\ufeff")
	text = strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n")
	return fmt.Sprintf("%x", sha256.Sum256([]byte(text)))
}

func hermesSkillPackageReceipt(request domain.ActivationRequest) error {
	plan, delivery := request.Plan, request.Delivery
	if delivery.ArtifactDigest == "" || delivery.ClientID != domain.ClientHermes || delivery.ActivePath != plan.ActivePath ||
		delivery.OwnedBase != plan.TargetRoot || request.DeclaredName != plan.DeclaredName {
		return fmt.Errorf("hermes Skill package authority differs")
	}
	expected := domain.NativeObjectOwnership{ObjectID: "package:hermes:" + plan.PhysicalArtifactID,
		Kind: "managed_package_directory", LogicalName: plan.DeclaredName, Path: plan.ActivePath,
		ManagedDigest: delivery.ArtifactDigest, ProtectionClass: "managed"}
	packages := 0
	for _, object := range delivery.NativeObjects {
		if object.Kind == "managed_package_directory" {
			if !reflect.DeepEqual(object, expected) {
				return fmt.Errorf("hermes Skill package receipt differs")
			}
			packages++
		}
	}
	if packages != 1 {
		return fmt.Errorf("hermes Skill package receipt missing or duplicated")
	}
	return nil
}
