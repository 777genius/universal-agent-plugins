package chatgpt

import (
	"context"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestProjectChatGPTDoesNotAdvertiseNativeFilesystemScope(t *testing.T) {
	registry, err := clients.NewRegistry(New())
	if err != nil {
		t.Fatal(err)
	}
	if !clients.SupportsScope(registry, domain.ClientChatGPT, domain.ScopeUser) || clients.SupportsScope(registry, domain.ClientChatGPT, domain.ScopeProject) {
		t.Fatal("ChatGPT scope qualification changed its user or project boundary")
	}
	if _, ok := clients.As[clients.ProjectTargetLayout](registry, domain.ClientChatGPT); ok {
		t.Fatal("ChatGPT exposed a fake project artifact installation layout")
	}
	if _, ok := clients.As[clients.NativeRegistryLayout](registry, domain.ClientChatGPT); ok {
		t.Fatal("ChatGPT exposed an unobservable project native registry")
	}
}

func TestProjectChatGPTDirectLifecycleRefusesPreparationAndAttestation(t *testing.T) {
	adapter := New()
	for _, intent := range []domain.InstallIntent{domain.InstallIntentAutomatic, domain.InstallIntentPrepare} {
		for _, target := range []domain.TargetContext{{Scope: domain.ScopeProject}, {Scope: domain.ScopeUser, ProjectRoot: t.TempDir()}} {
			request := domain.ActivationRequest{Client: domain.DetectedClient{ClientID: domain.ClientChatGPT},
				Plan: domain.DeliveryPlan{ClientID: domain.ClientChatGPT, Scope: domain.ScopeProject, TargetContext: target,
					InstallIntent: intent, PersonalChatGPTPreparation: true},
				Delivery: domain.StagedDelivery{ClientID: domain.ClientChatGPT}, ActivationComplete: true}
			if _, err := adapter.Activate(context.Background(), clients.Env{}, request); err == nil || !strings.Contains(err.Error(), "does not support project scope") {
				t.Fatalf("project direct activation/attestation = %v", err)
			}
			// Target context alone cannot bypass the guard through an inconsistent
			// legacy plan scope or a project root with a user scope label.
			request.Plan.Scope = domain.ScopeUser
			if err := adapter.PreflightActivation(clients.Env{}, request); err == nil {
				t.Fatal("project context bypassed direct activation preflight")
			}
			remove := domain.DeactivationRequest{TargetContext: target, Confirmed: true, ExternalUninstalled: true}
			if outcome, err := adapter.Deactivate(context.Background(), clients.Env{}, remove); err == nil || outcome.ExternalRemovalComplete {
				t.Fatalf("project external uninstall attested unsupported removal: %+v, %v", outcome, err)
			}
		}
	}
}

func TestProjectChatGPTLeafPreservesExistingUserManualPreparation(t *testing.T) {
	for _, personal := range []bool{false, true} {
		intent, activation := domain.InstallIntentAutomatic, domain.ActivationManual
		if personal {
			intent, activation = domain.InstallIntentPrepare, domain.ActivationPrepared
		}
		request := domain.ActivationRequest{Client: domain.DetectedClient{ClientID: domain.ClientChatGPT},
			Plan: domain.DeliveryPlan{ClientID: domain.ClientChatGPT, Scope: domain.ScopeUser, InstallIntent: intent,
				PersonalChatGPTPreparation: personal, Verification: domain.VerificationPackageValid},
			Delivery: domain.StagedDelivery{ClientID: domain.ClientChatGPT, ActivePath: t.TempDir()}, DeclaredName: "user-preparation"}
		outcome, err := New().Activate(context.Background(), clients.Env{}, request)
		if err != nil || outcome.Activation != activation || outcome.ActivationAttested || outcome.Verification == domain.VerificationInstalled {
			t.Fatalf("user manual/prepared boundary changed: %+v, %v", outcome, err)
		}
	}
}
