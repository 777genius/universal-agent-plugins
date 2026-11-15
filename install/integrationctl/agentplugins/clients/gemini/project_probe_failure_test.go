package gemini

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/adapters/nativeconfig"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients/shared"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	legacyports "github.com/777genius/plugin-kit-ai/install/integrationctl/ports"
)

type geminiFailingProbe struct{ result legacyports.CommandResult }

func (r geminiFailingProbe) Run(context.Context, legacyports.Command) (legacyports.CommandResult, error) {
	return r.result, context.DeadlineExceeded
}

func geminiProbeFixture(t *testing.T) (domain.ActivationRequest, nativeconfig.Kernel) {
	t.Helper()
	project, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	user, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(project, ".gemini")
	skill := filepath.Join(root, "skills", "audit")
	if err := os.MkdirAll(skill, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skill, "SKILL.md"), []byte("---\nname: audit\ndescription: audit marker\n---\nLocal marker.\n"), 0600); err != nil {
		t.Fatal(err)
	}
	digest, err := shared.DigestSkillDirectory(skill)
	if err != nil {
		t.Fatal(err)
	}
	object := domain.NativeObjectOwnership{ObjectID: GeminiSkillObjectKind + ":audit", Kind: GeminiSkillObjectKind, LogicalName: "audit", Path: skill, ManagedDigest: digest, ProtectionClass: "managed"}
	request := domain.ActivationRequest{VerifyOnly: true, Client: domain.DetectedClient{ClientID: domain.ClientGemini, ConfigRoot: filepath.Join(user, ".gemini"), ExecutablePath: "/native/gemini"}, Plan: domain.DeliveryPlan{Scope: domain.ScopeProject, NativeRegistryRoot: root, TargetContext: domain.TargetContext{Scope: domain.ScopeProject, ProjectRoot: project}, Components: []domain.ComponentDecision{{Kind: domain.ComponentSkill, Name: "audit", Support: domain.SupportNative}}}, Delivery: domain.StagedDelivery{NativeObjects: []domain.NativeObjectOwnership{object}}}
	return request, nativeconfig.New()
}

func TestGeminiCompletionPropagatesUnclosedProbeMetadata(t *testing.T) {
	for _, test := range []struct {
		name   string
		result legacyports.CommandResult
		unsafe bool
	}{{"unknown", legacyports.CommandResult{}, true}, {"closed", legacyports.CommandResult{CleanupComplete: true}, false}, {"not-started", legacyports.CommandResult{NoChildStarted: true}, false}} {
		t.Run(test.name, func(t *testing.T) {
			request, kernel := geminiProbeFixture(t)
			outcome, err := completeGeminiProject(context.Background(), clients.Env{NativeConfig: kernel, Runner: geminiFailingProbe{test.result}}, request, domain.ActivationOutcome{Activation: domain.ActivationPrepared, Verification: domain.VerificationInstalled, NativeEffect: domain.NativeEffectUnchanged})
			if domain.IsUnclosedClientProbe(err) != test.unsafe {
				t.Fatalf("lost cleanup metadata: %v", err)
			}
			if test.unsafe && outcome.NativeEffect != domain.NativeEffectUncertain {
				t.Fatal("unclosed process did not retain uncertain effect")
			}
			if !test.unsafe && err != nil {
				t.Fatalf("closed unavailable probe should remain prepared: %v", err)
			}
		})
	}
	request, kernel := geminiProbeFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := completeGeminiProject(ctx, clients.Env{NativeConfig: kernel}, request, domain.ActivationOutcome{Verification: domain.VerificationInstalled})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("parent cancellation lost: %v", err)
	}
}
