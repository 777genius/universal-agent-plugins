package antigravity

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestAntigravityPreparedNativeLifecycleDoesNotClaimActive(t *testing.T) {
	project := t.TempDir()
	nativeRoot := filepath.Join(project, ".agents")
	pluginRoot := filepath.Join(nativeRoot, "plugins")
	active := filepath.Join(pluginRoot, "test-plugin")
	if err := os.MkdirAll(active, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := shared.WriteJSON(filepath.Join(active, ".antigravity-plugin", "plugin.json"),
		map[string]any{"$schema": "https://antigravity.google/schemas/v1/plugin.json", "name": "test-plugin"}); err != nil {
		t.Fatal(err)
	}
	plan := domain.DeliveryPlan{
		ClientID: domain.ClientAntigravity, Scope: domain.ScopeProject,
		TargetContext: domain.TargetContext{Scope: domain.ScopeProject, ProjectRoot: project},
		NativeRegistryRoot: nativeRoot, TargetRoot: pluginRoot, ActivePath: active,
	}
	req := domain.ActivationRequest{
		Client: domain.DetectedClient{ClientID: domain.ClientAntigravity},
		DeclaredName: "test-plugin", Plan: plan,
		Delivery: domain.StagedDelivery{ClientID: domain.ClientAntigravity, ActivePath: active},
	}
	outcome, err := New().Activate(context.Background(), clients.Env{}, req)
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Activation != domain.ActivationPrepared || outcome.Verification != domain.VerificationInstalled ||
		outcome.ActivationAttested {
		t.Fatalf("installed files are not verified runtime activation: %+v", outcome)
	}
	req.ActivationComplete = true
	if _, err := New().Activate(context.Background(), clients.Env{}, req); err == nil {
		t.Fatal("installer must reject operator-only Active attestation")
	}
	req.ActivationComplete = false
	req.Plan.NativeRegistryRoot = filepath.Join(project, "different", ".agents")
	if _, err := New().Activate(context.Background(), clients.Env{}, req); err == nil {
		t.Fatal("project authority mismatch must not fall back to user")
	}
}

func TestAntigravityRemovalUsesSharedOwnedDirectoryOnly(t *testing.T) {
	root := t.TempDir()
	sentinel := filepath.Join(root, "foreign-plugin")
	if err := os.WriteFile(sentinel, []byte("do not modify"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, confirmed := range []bool{false, true} {
		outcome, err := New().Deactivate(context.Background(), clients.Env{},
			domain.DeactivationRequest{Confirmed: confirmed})
		if err != nil {
			t.Fatal(err)
		}
		if !outcome.ArtifactRemovalAllowed {
			t.Fatalf("generic ownership transaction should retain removal authority: %+v", outcome)
		}
	}
	body, err := os.ReadFile(sentinel)
	if err != nil || string(body) != "do not modify" {
		t.Fatalf("foreign plugin was affected: %q %v", body, err)
	}
}

func TestAntigravityGroupPreflightHasNoStagedDeliveryYet(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	profile := filepath.Join(root, "user", ".gemini", "config")
	project := filepath.Join(root, "project")
	nativeRoot := filepath.Join(project, ".agents")
	pluginRoot := filepath.Join(nativeRoot, "plugins")
	active := filepath.Join(pluginRoot, "managed")
	req := domain.ActivationRequest{
		Client: domain.DetectedClient{ClientID: domain.ClientAntigravity, ConfigRoot: profile},
		Plan: domain.DeliveryPlan{ClientID: domain.ClientAntigravity, Scope: domain.ScopeProject,
			NativeRegistryRoot: nativeRoot, TargetRoot: pluginRoot, ActivePath: active,
			TargetContext: domain.TargetContext{Scope: domain.ScopeProject, ProjectRoot: project, UserConfigRoot: profile}},
	}
	if err := New().PreflightActivation(clients.Env{}, req); err != nil {
		t.Fatalf("pre-staging group preflight refused valid selected authority: %v", err)
	}
	req.Delivery = domain.StagedDelivery{ClientID: domain.ClientAntigravity, ActivePath: filepath.Join(pluginRoot, "foreign")}
	if err := New().PreflightActivation(clients.Env{}, req); err == nil {
		t.Fatal("foreign staged delivery path accepted")
	}
	req.Delivery = domain.StagedDelivery{ClientID: domain.ClientHermes, ActivePath: active}
	if err := New().PreflightActivation(clients.Env{}, req); err == nil {
		t.Fatal("staged delivery for another client accepted")
	}
}
