package planner

import (
	"context"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestSignedChatGPTAppBindingAuthorizesPreparationWithoutClaimingRemoteVerification(t *testing.T) {
	t.Parallel()
	envelope := domain.PackageEnvelope{
		Manifest: domain.PluginManifest{Name: "remote-mcp"},
		MCP: domain.MCPComponent{Present: true, Enabled: true, Servers: map[string]domain.MCPServer{
			"docs": {Name: "docs", Type: "streamable-http"},
		}},
		App: domain.AppComponent{Present: true, Declared: true, Enabled: true, Bindings: map[string]domain.AppBinding{
			"docs": {Alias: "docs", ID: "asdk_app_docs_123"},
		}},
		CatalogEvidence: &domain.CatalogEvidence{SchemaVersion: 1, Compatibility: map[string]domain.CatalogCompatibility{
			"chatgpt": {Package: "projected", Verification: "tested", Authentication: domain.AuthenticationRequirementRequired,
				AppBinding: &domain.CatalogAppBinding{AppKey: "docs", ID: "asdk_app_docs_123", MCPServer: "docs", MCPURL: "https://example.test/mcp"}},
		}},
	}
	plan, err := testPlanner(Planner{ManagedRoot: t.TempDir()}).Plan(context.Background(), envelope, domain.DetectedClient{ClientID: domain.ClientChatGPT}, domain.ScopeUser, "remote-mcp-0123456789ab")
	if err != nil {
		t.Fatal(err)
	}
	if !plan.LocalPreparationAuthorized || plan.Status != domain.PlanManualActivationRequired || plan.Activation != domain.ActivationManual || plan.Authentication != domain.AuthenticationPending || plan.Verification == domain.VerificationInstalled {
		t.Fatalf("ChatGPT preparation boundary = %+v", plan)
	}

	envelope.CatalogEvidence.Compatibility["chatgpt"] = domain.CatalogCompatibility{Package: "projected", Verification: "tested", Authentication: domain.AuthenticationRequirementRequired}
	unsignedPlan, err := testPlanner(Planner{ManagedRoot: t.TempDir()}).Plan(context.Background(), envelope, domain.DetectedClient{ClientID: domain.ClientChatGPT}, domain.ScopeUser, "remote-mcp-0123456789ab")
	if err != nil {
		t.Fatal(err)
	}
	if unsignedPlan.LocalPreparationAuthorized {
		t.Fatalf("compatibility without an explicit signed app binding authorized preparation: %+v", unsignedPlan)
	}
}
