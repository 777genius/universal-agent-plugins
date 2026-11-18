package grok

import (
	"context"
	"fmt"
	"path/filepath"
	"reflect"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/pathpolicy"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/conformance"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/packagesnapshot"
)

type projectSkillProof struct {
	name, root, digest string
	authentication     domain.AuthenticationState
	skills             map[string]projectSkillMetadata
}

type projectSkillMetadata struct {
	path, description string
}

func projectSkillsVerifiable(client domain.DetectedClient, plan domain.DeliveryPlan, executable string) bool {
	// The ordinary CLI does not probe a version ahead of planning. A blank
	// detected version is not authority: the bound native inspect result below
	// must still prove exact 1.0.70. Reject any known different version.
	if (client.Version != "" && client.Version != "1.0.70") || executable == "" || executable != client.ExecutablePath ||
		plan.NativeRegistryExecutable != executable || plan.Status != domain.PlanReady ||
		plan.PackageMode != domain.PackageNative || plan.InstallIntent != domain.InstallIntentAutomatic ||
		plan.Policy != domain.PolicyAllowed || len(plan.Components) == 0 {
		return false
	}
	if _, err := grokPlanRoot(client, plan); err != nil {
		return false
	}
	return projectSkillComponents(plan.Components)
}

func projectSkillComponents(components []domain.ComponentDecision) bool {
	seen := map[string]bool{}
	for _, component := range components {
		key := shared.FoldJSONKey(component.Name)
		if component.Kind != domain.ComponentSkill || component.Support != domain.SupportNative ||
			component.Source != "" || component.SourceName != "" || pathpolicy.ValidateLeafID(component.Name) != nil || seen[key] {
			return false
		}
		seen[key] = true
	}
	return true
}

// Hold package authority and metadata before inspect. Neither mutable native
// output nor a caller's later plan mutation can replace this owned byte proof.
func captureProjectSkills(ctx context.Context, env clients.Env, request domain.ActivationRequest) *projectSkillProof {
	if request.ActivationComplete || request.ConfirmPluginEnable ||
		!projectSkillsVerifiable(request.Client, request.Plan, request.BackendExecutable) || !projectSkillPackageOwned(request) {
		return nil
	}
	proof := &projectSkillProof{name: request.Plan.DeclaredName, root: request.Plan.ActivePath,
		digest: request.Delivery.ArtifactDigest, authentication: shared.ObservedSkillInstallationAuthentication(request.Plan),
		skills: map[string]projectSkillMetadata{}}
	if digest, err := (packagesnapshot.Builder{}).Digest(ctx, proof.root); err != nil || digest != proof.digest {
		return nil
	}
	for _, component := range request.Plan.Components {
		path := filepath.Join(proof.root, "skills", component.Name, "SKILL.md")
		file, err := env.NativeConfig.ReadExactFileBounded(path, 1<<20)
		if err != nil || !file.Exists {
			return nil
		}
		skill, err := conformance.ParseSkill(ctx, component.Name, file.Body)
		if err != nil || skill.Name != component.Name {
			return nil
		}
		proof.skills[component.Name] = projectSkillMetadata{path: path, description: skill.Description}
	}
	return proof
}

func projectSkillPackageOwned(request domain.ActivationRequest) bool {
	delivery, plan := request.Delivery, request.Plan
	if delivery.ArtifactDigest == "" || delivery.ClientID != domain.ClientGrok || delivery.ActivePath != plan.ActivePath ||
		delivery.OwnedBase != plan.TargetRoot || request.DeclaredName != plan.DeclaredName {
		return false
	}
	expected := domain.NativeObjectOwnership{ObjectID: "package:grok:" + plan.PhysicalArtifactID,
		Kind: "managed_package_directory", LogicalName: plan.DeclaredName, Path: plan.ActivePath,
		ManagedDigest: delivery.ArtifactDigest, ProtectionClass: "managed"}
	packages := 0
	for _, object := range delivery.NativeObjects {
		if object.Kind == enabledMemberKind {
			continue
		}
		if !reflect.DeepEqual(object, expected) {
			return false
		}
		packages++
	}
	_, err := enabledObjectForProfile(delivery.NativeObjects, plan.ActivePath, plan.DeclaredName, request.Client.ConfigRoot)
	return packages == 1 && err == nil
}

func completeProjectSkills(ctx context.Context, proof *projectSkillProof, native projectInspection, outcome domain.ActivationOutcome) (domain.ActivationOutcome, error) {
	if proof == nil || outcome.Policy != domain.PolicyAllowed || outcome.ActivationAttested || outcome.AuthenticationAttested ||
		outcome.NativeEffect == domain.NativeEffectUncertain {
		return outcome, nil
	}
	if !projectSkillBytesUnchanged(ctx, proof) {
		return outcome, fmt.Errorf("owned Grok project Skill package changed during native inspection")
	}
	if !projectSkillsObserved(native, proof) {
		return outcome, nil
	}
	outcome.Activation, outcome.Verification = domain.ActivationActive, domain.VerificationInstalled
	outcome.UserActions = nil
	if outcome.Authentication == domain.AuthenticationNotChecked && proof.authentication == domain.AuthenticationNotRequired {
		outcome.Authentication = domain.AuthenticationNotRequired
	}
	return outcome, nil
}

func projectSkillBytesUnchanged(ctx context.Context, proof *projectSkillProof) bool {
	// This is the receipt's complete package digest, including every selected
	// Skill resource and projected metadata, with the builder's finite limits.
	digest, err := (packagesnapshot.Builder{}).Digest(ctx, proof.root)
	return err == nil && digest == proof.digest
}
