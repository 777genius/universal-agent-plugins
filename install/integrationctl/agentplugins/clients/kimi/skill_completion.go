package kimi

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"slices"
	"time"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

const kimiSkillCompletionAction = "Kimi Skills are installed. Retry installation to finish checking that Kimi can find them in this project. Kimi may initialize its local server and workspace metadata. No project trust, session, model or MCP activation is requested."

type kimiSkillMetadata struct{ path, description, digest string }

func kimiPureSkillPlan(plan domain.DeliveryPlan) bool {
	if plan.ClientID != domain.ClientKimi || plan.Scope != domain.ScopeProject || plan.TargetContext.Scope != domain.ScopeProject || plan.InstallIntent != domain.InstallIntentAutomatic || plan.Policy != domain.PolicyAllowed || len(plan.Components) == 0 {
		return false
	}
	seen := map[string]bool{}
	for _, component := range plan.Components {
		key := shared.FoldJSONKey(component.Name)
		if component.Kind != domain.ComponentSkill || component.Support != domain.SupportNative || component.Source != "" || component.SourceName != "" || !domain.ValidSkillName(component.Name) || seen[key] {
			return false
		}
		seen[key] = true
	}
	return true
}

func kimiSkillVersion(client domain.DetectedClient) bool {
	return client.Status == domain.DetectionDetected && currentKimi(client) == nil && (client.Version == "2.3.2" || client.Version == "2.5.2")
}

func completeKimiSkills(ctx context.Context, env clients.Env, request domain.ActivationRequest, outcome domain.ActivationOutcome) (domain.ActivationOutcome, error) {
	if !kimiSkillCompletionEligible(request.Plan, outcome) {
		return outcome, nil
	}
	outcome.UserActions = slices.DeleteFunc(outcome.UserActions, func(action string) bool { return action == KimiProjectAction })
	outcome.UserActions = shared.AppendUnique(outcome.UserActions, kimiSkillCompletionAction)
	// Even a GET-only Kimi server boot can migrate workspace/session metadata.
	// Read-only verification never starts that process. Confirmed completion
	// re-enters through the existing PreparedSetupCompleter path instead.
	if request.VerifyOnly || !kimiSkillVersion(request.Client) || request.ActivationComplete || request.ConfirmPluginEnable || env.NativeHTTP == nil || env.Snapshots == nil || env.ParseSkill == nil {
		return outcome, nil
	}
	before, command, err := kimiSkillProfile(ctx, env, request)
	if err != nil {
		if errors.Is(err, errKimiAdditionalDirectories) {
			outcome.UserActions = shared.AppendUnique(outcome.UserActions, "Review Kimi's project local.toml directory scope before native Skill verification; its additional-directory selection is not qualified")
		}
		return outcome, nil
	}
	expected, err := kimiOwnedSkills(ctx, env, request)
	if err != nil {
		return outcome, err
	}
	bounded, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	effect, issuedToken, probeErr := observeKimiSkills(bounded, env, request, command, expected)
	if effect != domain.NativeEffectUnchanged {
		outcome.NativeEffect = effect
	}
	if domain.IsUnclosedClientProbe(probeErr) {
		outcome.NativeEffect = domain.NativeEffectUncertain
		return outcome, probeErr
	}
	if ctx.Err() != nil {
		return outcome, ctx.Err()
	}
	if effect == domain.NativeEffectUncertain {
		return outcome, probeErr
	}
	return finishKimiSkills(ctx, env, request, outcome, before, issuedToken, expected, probeErr)
}

func kimiSkillCompletionEligible(plan domain.DeliveryPlan, outcome domain.ActivationOutcome) bool {
	return kimiPureSkillPlan(plan) && outcome.Activation == domain.ActivationPrepared && outcome.Verification == domain.VerificationInstalled && outcome.Policy == domain.PolicyAllowed && !outcome.ActivationAttested && !outcome.AuthenticationAttested && outcome.NativeEffect != domain.NativeEffectUncertain
}

func finishKimiSkills(ctx context.Context, env clients.Env, request domain.ActivationRequest, outcome domain.ActivationOutcome, before kimiProfileSnapshot, issuedToken shared.NativeFileSnapshot, expected map[string]kimiSkillMetadata, probeErr error) (domain.ActivationOutcome, error) {
	owned := kimiOwnedSkillsMatch(ctx, env, request, expected)
	after, _, profileErr := kimiSkillProfile(ctx, env, request)
	if err := ctx.Err(); err != nil {
		return outcome, err
	}
	if profileErr != nil || !before.same(after) || issuedToken.Exists && !reflect.DeepEqual(issuedToken, after.token) {
		outcome.NativeEffect = domain.NativeEffectUncertain
		return outcome, fmt.Errorf("kimi protected profile or discovery policy changed during native observation")
	}
	if !owned {
		return outcome, fmt.Errorf("owned Kimi Skill contents changed during native observation")
	}
	if probeErr == nil {
		outcome.Activation = domain.ActivationActive
		if outcome.Authentication == domain.AuthenticationNotChecked {
			outcome.Authentication = shared.ObservedSkillInstallationAuthentication(request.Plan)
		}
		outcome.UserActions = slices.DeleteFunc(outcome.UserActions, func(action string) bool { return action == kimiSkillCompletionAction })
	} else {
		outcome.LocalActions = shared.AppendUnique(outcome.LocalActions, "Kimi native Skills discovery did not complete; retry the confirmed installation")
	}
	return outcome, nil
}

func kimiOwnedSkillsMatch(ctx context.Context, env clients.Env, request domain.ActivationRequest, expected map[string]kimiSkillMetadata) bool {
	current, err := kimiOwnedSkills(ctx, env, request)
	return err == nil && reflect.DeepEqual(expected, current)
}

func kimiOwnedSkills(ctx context.Context, env clients.Env, request domain.ActivationRequest) (map[string]kimiSkillMetadata, error) {
	plan, delivery := request.Plan, request.Delivery
	if delivery.ClientID != domain.ClientKimi || delivery.ActivePath != plan.ActivePath || delivery.OwnedBase != plan.TargetRoot || request.DeclaredName != plan.DeclaredName || delivery.ArtifactDigest == "" {
		return nil, fmt.Errorf("kimi Skill package authority differs")
	}
	objects, err := kimiOwnedSkillObjects(plan, delivery)
	if err != nil {
		return nil, err
	}
	layout := kimiComponents(plan.NativeRegistryRoot)
	if err := layout.Verify(objects, false, env.NativeConfig); err != nil {
		return nil, err
	}
	digest, err := env.Snapshots.Digest(ctx, delivery.ActivePath)
	if err != nil || digest != delivery.ArtifactDigest {
		return nil, fmt.Errorf("kimi managed package contents differ")
	}
	expected := map[string]kimiSkillMetadata{}
	for _, object := range objects {
		if object.Kind != kimiProjectSkillKind || object.ProtectionClass != "managed" || object.Path != filepath.Join(layout.SkillsRoot, object.LogicalName) || expected[object.LogicalName].path != "" || !slices.ContainsFunc(plan.Components, func(component domain.ComponentDecision) bool { return component.Name == object.LogicalName }) {
			return nil, fmt.Errorf("kimi native Skill ownership differs")
		}
		body, err := shared.ReadOwnedSkillMetadata(ctx, env.NativeConfig, env.Snapshots, object.Path, object.ManagedDigest)
		if err != nil {
			return nil, err
		}
		skill, err := env.ParseSkill(ctx, object.LogicalName, body)
		if err != nil || skill.Name != object.LogicalName {
			return nil, fmt.Errorf("kimi owned Skill metadata differs")
		}
		expected[object.LogicalName] = kimiSkillMetadata{path: filepath.Join(object.Path, "SKILL.md"), description: skill.Description, digest: fmt.Sprintf("%x", sha256.Sum256(body))}
	}
	return expected, nil
}

func kimiOwnedSkillObjects(plan domain.DeliveryPlan, delivery domain.StagedDelivery) ([]domain.NativeObjectOwnership, error) {
	packageReceipt := domain.NativeObjectOwnership{ObjectID: "package:kimi:" + plan.PhysicalArtifactID, Kind: "managed_package_directory", LogicalName: plan.DeclaredName, Path: plan.ActivePath, ManagedDigest: delivery.ArtifactDigest, ProtectionClass: "managed"}
	packages := 0
	for _, object := range delivery.NativeObjects {
		if object.Kind == "managed_package_directory" {
			if !reflect.DeepEqual(object, packageReceipt) {
				return nil, fmt.Errorf("kimi managed package receipt differs")
			}
			packages++
		}
	}
	objects, layout := kimiNativeObjects(delivery.NativeObjects), kimiComponents(plan.NativeRegistryRoot)
	if packages != 1 || len(objects) != len(plan.Components) || layout.ValidateObjects(objects) != nil {
		return nil, fmt.Errorf("kimi Skill receipt coverage differs")
	}
	return objects, nil
}
