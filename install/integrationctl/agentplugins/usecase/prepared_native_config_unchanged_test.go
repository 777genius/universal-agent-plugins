package usecase

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestPreparedNativeConfigNoChangeRequiresCompleteUnchangedOwnership(t *testing.T) {
	for _, name := range []string{"exact", "incomplete projection", "missing ownership", "delivery mismatch", "outcome mismatch", "committed effect", "unknown effect", "pending attempt", "pending intent", "activation completion", "auth completion", "activation attestation", "auth attestation", "user scope", "wrong client", "wrong active path", "unverified", "blocked policy", "changed auth", "no verifier"} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			active := filepath.Join(root, ".uap", "managed", "fixture")
			digest := "sha256:" + strings.Repeat("1", 64)
			objects := []domain.NativeObjectOwnership{
				{Kind: "managed_package_directory", ObjectID: "package:codex:fixture", Path: active, ManagedDigest: digest},
				{Kind: "codex_project_skill_directory", ObjectID: "skill:docs", Path: filepath.Join(root, ".agents", "skills", "docs"), ManagedDigest: digest},
			}
			input := AddInput{Scope: domain.ScopeProject, Client: domain.DetectedClient{ClientID: domain.ClientCodex}}
			plan := domain.DeliveryPlan{ClientID: domain.ClientCodex, Scope: domain.ScopeProject, ActivePath: active}
			binding := domain.ClientBinding{ClientID: "codex", Scope: "project", TargetLocator: active, Materialization: domain.MaterializationMaterialized, Activation: domain.ActivationPrepared, Verification: domain.VerificationInstalled, Authentication: domain.AuthenticationNotChecked, Policy: domain.PolicyAllowed, NativeObjects: append([]domain.NativeObjectOwnership(nil), objects...)}
			delivery := domain.StagedDelivery{ClientID: domain.ClientCodex, ActivePath: active, ArtifactDigest: digest, NativeObjects: append([]domain.NativeObjectOwnership(nil), objects...)}
			outcome := lifecycleOutcome(binding)
			outcome.NativeEffect, outcome.NativeObjects = domain.NativeEffectUnchanged, append([]domain.NativeObjectOwnership(nil), objects...)
			actor := &resumeObservationActivator{available: true}
			service := Service{Activator: actor}
			complete := true
			switch name {
			case "incomplete projection":
				complete = false
			case "missing ownership":
				binding.NativeObjects, delivery.NativeObjects, outcome.NativeObjects = nil, nil, nil
			case "delivery mismatch":
				delivery.NativeObjects[1].Path += "-foreign"
			case "outcome mismatch":
				outcome.NativeObjects[1].Path += "-foreign"
			case "committed effect":
				outcome.NativeEffect = domain.NativeEffectCommitted
			case "unknown effect":
				outcome.NativeEffect = ""
			case "pending attempt":
				binding.NativeActivationAttempt = "unresolved"
			case "pending intent":
				binding.PendingNativeIntent = &domain.PendingNativeIntent{}
			case "activation completion":
				input.ActivationComplete = true
			case "auth completion":
				input.AuthComplete = true
			case "activation attestation":
				outcome.ActivationAttested = true
			case "auth attestation":
				outcome.AuthenticationAttested = true
			case "user scope":
				input.Scope, plan.Scope, binding.Scope = domain.ScopeUser, domain.ScopeUser, "user"
			case "wrong client":
				binding.ClientID = "opencode"
			case "wrong active path":
				plan.ActivePath += "-foreign"
			case "unverified":
				outcome.Verification = domain.VerificationPackageValid
			case "blocked policy":
				binding.Policy, outcome.Policy = domain.PolicyBlocked, domain.PolicyBlocked
			case "changed auth":
				outcome.Authentication = domain.AuthenticationComplete
			case "no verifier":
				actor.available = false
			}
			if got := service.verifiedPreparedObservationUnchanged(input, plan, binding, delivery, outcome, complete); got != (name == "exact") {
				t.Fatalf("contradictory or missing proof classified as unchanged=%t", got)
			}
			if binding.Activation != domain.ActivationPrepared || binding.LifecycleConverged() {
				t.Fatal("operation classification promoted Prepared lifecycle")
			}
		})
	}
}
