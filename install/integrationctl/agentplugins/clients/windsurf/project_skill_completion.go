package windsurf

import (
	"context"
	"fmt"
	"path/filepath"
	"reflect"
	"slices"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/ports"
)

var _ clients.PreparedSetupCompleter = (*Adapter)(nil)

// Membership permits a fresh read-only verification of a saved projection.
// Devin owns no acting trust, permission or cloud-policy setup repair here.
func (*Adapter) CanCompletePreparedSetup(domain.ActivationRequest, domain.ActivationOutcome) bool {
	return false
}

type devinSkillMetadata struct {
	path, description string
}

type devinSkillContext struct {
	layout      shared.ProjectComponents
	cwd, action string
}

func devinSkillContextFor(request domain.ActivationRequest) (devinSkillContext, error) {
	plan := request.Plan
	if plan.Scope == domain.ScopeProject {
		root, err := devinProjectPlanRoot(request.Client, plan)
		return devinSkillContext{layout: devinComponents(root), cwd: plan.TargetContext.ProjectRoot, action: DevinProjectAction}, err
	}
	root, err := devinGlobalPlanRoot(request.Client, plan)
	if err == nil {
		err = validateDevinGlobalProjectionRoot(plan.ActivePath, plan)
	}
	// A Global query retains original HOME/XDG authority but has no selected
	// project. The volume root avoids inheriting a repository around that HOME.
	cwd := filepath.VolumeName(root) + string(filepath.Separator)
	return devinSkillContext{layout: devinGlobalComponents(root, plan.ActivePath), cwd: cwd, action: DevinGlobalAction}, err
}

func devinSkillContextKnown(env clients.Env, scope domain.InstallScope, cwd string) bool {
	if scope == domain.ScopeProject {
		return true
	}
	if filepath.Dir(cwd) != cwd {
		return false
	}
	// Pinned Devin also imports these project sources without a Git marker.
	// Check only the neutral volume root, never normal user-level imports.
	for _, name := range []string{".git", ".jj", ".devin", ".agents", ".windsurf", ".claude", ".cursor", ".github", ".zed", "opencode.json", ".mcp.json"} {
		file, err := env.NativeConfig.ReadExactFileBounded(filepath.Join(cwd, name), 0)
		if err != nil || file.Exists {
			return false
		}
	}
	return true
}

func devinPureObservedSkills(request domain.ActivationRequest) bool {
	plan := request.Plan
	if requireCurrentDevin(request.Client) != nil || request.BackendExecutable == "" ||
		request.BackendExecutable != request.Client.ExecutablePath || request.ActivationComplete || request.ConfirmPluginEnable ||
		plan.Status != domain.PlanReady || plan.PackageMode != domain.PackagePrepared ||
		plan.InstallIntent != domain.InstallIntentAutomatic || plan.Policy != domain.PolicyAllowed || len(plan.Components) == 0 {
		return false
	}
	if _, err := devinSkillContextFor(request); err != nil {
		return false
	}
	seen := map[string]bool{}
	for _, component := range plan.Components {
		key := shared.FoldJSONKey(component.Name)
		if component.Kind != domain.ComponentSkill || component.Support != domain.SupportPrepared ||
			component.Source != "" || component.SourceName != "" || !domain.ValidSkillName(component.Name) || seen[key] {
			return false
		}
		seen[key] = true
	}
	return true
}

func completeDevinSkills(ctx context.Context, env clients.Env, request domain.ActivationRequest, outcome domain.ActivationOutcome) (domain.ActivationOutcome, error) {
	if !devinSkillObservationAllowed(env, request, outcome) {
		return outcome, nil
	}
	selected, err := devinSkillContextFor(request)
	if err != nil {
		return outcome, err
	}
	global, err := requireSelectedDevinGlobal(request.Client)
	if err != nil || !devinSkillContextKnown(env, request.Plan.Scope, selected.cwd) {
		return outcome, nil
	}
	before, eligible, err := devinSkillPolicyFiles(env, request.Plan.NativeRegistryRoot, global)
	if err != nil || !eligible {
		return outcome, err
	}
	if request.VerifyOnly && !devinSkillPolicyVersioned(before[filepath.Join(global, "config.json")]) {
		return outcome, nil
	}
	skills, digest, err := devinOwnedSkills(ctx, env, request, selected.layout)
	if err != nil {
		return outcome, err
	}
	bounded, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	result, probeErr := shared.RunNativeRegistry(bounded, env.Runner, ports.Command{
		Argv: []string{request.BackendExecutable, "skills", "list", "--json"},
		Dir:  selected.cwd, StdoutLimitBytes: 1 << 20,
	})
	if domain.IsUnclosedClientProbe(probeErr) {
		outcome.NativeEffect = domain.NativeEffectUncertain
		return outcome, probeErr
	}
	if ctx.Err() != nil {
		return outcome, ctx.Err()
	}
	afterSkills, afterDigest, err := devinOwnedSkills(ctx, env, request, selected.layout)
	if err != nil || digest != afterDigest || !reflect.DeepEqual(skills, afterSkills) {
		return outcome, fmt.Errorf("owned devin Skill bytes changed during native observation")
	}
	if err := verifyDevinSkillPolicy(env, request.Plan, selected.cwd, global, before); err != nil {
		return outcome, err
	}
	if err := ctx.Err(); err != nil {
		return outcome, err
	}
	if !devinSkillProbeMatches(result, probeErr, skills) {
		return outcome, nil
	}
	outcome.Activation = domain.ActivationActive
	outcome.UserActions = slices.DeleteFunc(outcome.UserActions, func(action string) bool { return action == selected.action })
	if outcome.Authentication == domain.AuthenticationNotChecked &&
		shared.ObservedPreparedSkillInstallationAuthentication(request.Plan) == domain.AuthenticationNotRequired {
		outcome.Authentication = domain.AuthenticationNotRequired
	}
	return outcome, nil
}


func verifyDevinSkillPolicy(env clients.Env, plan domain.DeliveryPlan, cwd, global string, before map[string]shared.NativeFileSnapshot) error {
	after, eligible, err := devinSkillPolicyFiles(env, plan.NativeRegistryRoot, global)
	if err != nil || !eligible || !devinSkillContextKnown(env, plan.Scope, cwd) || !devinSkillPolicyStable(before, after, global) {
		return fmt.Errorf("devin Skill discovery policy changed during native observation")
	}
	return nil
}

func devinSkillProbeMatches(result ports.CommandResult, probeErr error, skills map[string]devinSkillMetadata) bool {
	return probeErr == nil && result.ExitCode == 0 && len(result.Stderr) == 0 && devinProjectSkillsObserved(string(result.Stdout), skills)
}

func devinOwnedSkills(ctx context.Context, env clients.Env, request domain.ActivationRequest, layout shared.ProjectComponents) (map[string]devinSkillMetadata, string, error) {
	plan, delivery := request.Plan, request.Delivery
	if err := devinSkillPackageReceipt(request); err != nil {
		return nil, "", err
	}

	objects := devinNativeObjects(delivery.NativeObjects)
	if len(objects) != len(plan.Components) {
		return nil, "", fmt.Errorf("devin Project Skill receipt coverage differs")
	}
	if err := layout.Verify(objects, false, env.NativeConfig); err != nil {
		return nil, "", err
	}
	digest, err := env.Snapshots.Digest(ctx, plan.ActivePath)
	if err != nil || digest != delivery.ArtifactDigest {
		return nil, "", fmt.Errorf("devin Project Skill package digest differs")
	}
	skills := map[string]devinSkillMetadata{}
	for _, component := range plan.Components {
		metadata, err := devinOwnedSkillMetadata(ctx, env, layout, component.Name, objects)
		if err != nil {
			return nil, "", err
		}
		skills[component.Name] = metadata
	}
	return skills, digest, nil
}

func devinSkillObservationAllowed(env clients.Env, request domain.ActivationRequest, outcome domain.ActivationOutcome) bool {
	return env.ParseSkill != nil && env.Snapshots != nil && env.NativeConfig.RequireBoundedRead() == nil && devinPureObservedSkills(request) && env.Runner != nil && outcome.Verification == domain.VerificationInstalled && outcome.Policy == domain.PolicyAllowed && !outcome.ActivationAttested && !outcome.AuthenticationAttested && outcome.NativeEffect != domain.NativeEffectUncertain
}

func devinSkillPackageReceipt(request domain.ActivationRequest) error {
	plan, delivery := request.Plan, request.Delivery
	if delivery.ArtifactDigest == "" || delivery.ClientID != domain.ClientWindsurf || delivery.ActivePath != plan.ActivePath ||
		delivery.OwnedBase != plan.TargetRoot || request.DeclaredName != plan.DeclaredName {
		return fmt.Errorf("devin Project Skill package authority differs")
	}
	expected := domain.NativeObjectOwnership{ObjectID: "package:windsurf:" + plan.PhysicalArtifactID,
		Kind: "managed_package_directory", LogicalName: plan.DeclaredName, Path: plan.ActivePath,
		ManagedDigest: delivery.ArtifactDigest, ProtectionClass: "managed"}
	packages := 0
	for _, object := range delivery.NativeObjects {
		if object.Kind == "managed_package_directory" {
			if !reflect.DeepEqual(object, expected) {
				return fmt.Errorf("devin Project Skill package receipt differs")
			}
			packages++
		}
	}
	if packages != 1 {
		return fmt.Errorf("devin Project Skill package receipt is missing or duplicated")
	}
	return nil
}

func devinOwnedSkillMetadata(ctx context.Context, env clients.Env, layout shared.ProjectComponents, name string, objects []domain.NativeObjectOwnership) (devinSkillMetadata, error) {
	metadata := devinSkillMetadata{}
	matches := 0
	for _, object := range objects {
		if object.Kind != layout.SkillKind || object.LogicalName != name ||
			object.Path != filepath.Join(layout.SkillsRoot, name) ||
			object.ManagedDigest == "" || object.ProtectionClass != "managed" {
			continue
		}
		body, err := shared.ReadOwnedSkillMetadata(ctx, env.NativeConfig, env.Snapshots, object.Path, object.ManagedDigest)
		if err != nil {
			return devinSkillMetadata{}, fmt.Errorf("devin Project Skill source unavailable")
		}
		skill, err := env.ParseSkill(ctx, name, body)
		if err != nil || skill.Name != name {
			return devinSkillMetadata{}, fmt.Errorf("devin Project Skill metadata differs")
		}
		metadata = devinSkillMetadata{path: object.Path, description: skill.Description}
		matches++
	}
	if matches != 1 {
		return devinSkillMetadata{}, fmt.Errorf("devin Project Skill receipt is missing or duplicated")
	}
	return metadata, nil
}
