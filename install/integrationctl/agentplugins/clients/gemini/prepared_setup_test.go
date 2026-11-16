package gemini

import (
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestGeminiPreparedSetupRequiresExactSupportedSkillProject(t *testing.T) {
	project := t.TempDir()
	root := filepath.Join(t.TempDir(), ".gemini")
	request := domain.ActivationRequest{Client: domain.DetectedClient{ConfigRoot: root, ExecutablePath: "/native/gemini"}, Plan: domain.DeliveryPlan{Scope: domain.ScopeProject, InstallIntent: domain.InstallIntentAutomatic, NativeRegistryRoot: filepath.Join(project, ".gemini"), TargetContext: domain.TargetContext{Scope: domain.ScopeProject, ProjectRoot: project}, Components: []domain.ComponentDecision{{Kind: domain.ComponentSkill, Name: "audit", Support: domain.SupportNative}}}}
	outcome := domain.ActivationOutcome{Activation: domain.ActivationPrepared, Verification: domain.VerificationInstalled, Policy: domain.PolicyAllowed}
	adapter := New()
	if !adapter.CanCompletePreparedSetup(request, outcome) {
		t.Fatal("supported exact skill project refused")
	}
	request.Plan.NativeRegistryRoot = filepath.Join(project, "foreign")
	if adapter.CanCompletePreparedSetup(request, outcome) {
		t.Fatal("foreign authority qualified")
	}
	request.Plan.NativeRegistryRoot = filepath.Join(project, ".gemini")
	request.Plan.Components = append(request.Plan.Components, domain.ComponentDecision{Kind: domain.ComponentMCPServer, Name: "unknown-transport", Support: domain.SupportNative})
	if adapter.CanCompletePreparedSetup(request, outcome) {
		t.Fatal("unknown MCP transport qualified by skill classifier")
	}
}
