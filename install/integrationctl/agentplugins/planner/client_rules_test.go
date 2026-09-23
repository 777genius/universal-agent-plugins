package planner

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

// TestClaudeWithoutItsCLIIsRejectedBeforeATargetIsResolved covers the one
// precondition in the registry. The rejection has to happen before the target
// is resolved, so an unusable plan never hands a caller paths it could act on.
func TestClaudeWithoutItsCLIIsRejectedBeforeATargetIsResolved(t *testing.T) {
	t.Parallel()
	client := detectedClient(domain.ClientClaude, filepath.Join(t.TempDir(), ".claude"))
	plan, err := testPlanner(Planner{ManagedRoot: t.TempDir()}).Plan(context.Background(), testEnvelope(), client, domain.ScopeUser, "demo-0123456789ab")
	if err != nil {
		t.Fatal(err)
	}
	if plan.Status != domain.PlanUnsupported || plan.Activation != domain.ActivationFailed {
		t.Fatalf("Claude without its CLI = %s/%s", plan.Status, plan.Activation)
	}
	if !contains(plan.Warnings, "trusted_claude_cli_required") {
		t.Fatalf("warnings = %v", plan.Warnings)
	}
	if !contains(plan.UserActions, "install Claude Code CLI so agentplugins can verify the exact @skills-dir identity") {
		t.Fatalf("user actions = %v", plan.UserActions)
	}
	if plan.TargetRoot != "" || plan.ActivePath != "" || len(plan.Components) != 0 {
		t.Fatalf("rejected plan carries a resolved target: %+v", plan)
	}

	client.ExecutablePath = filepath.Join(t.TempDir(), "claude")
	ready, err := testPlanner(Planner{ManagedRoot: t.TempDir()}).Plan(context.Background(), testEnvelope(), client, domain.ScopeUser, "demo-0123456789ab")
	if err != nil {
		t.Fatal(err)
	}
	if ready.Status == domain.PlanUnsupported || contains(ready.Warnings, "trusted_claude_cli_required") {
		t.Fatalf("Claude with its CLI = %+v", ready)
	}
}

// TestCopilotWithoutItsCLIIsNotPromoted covers the negative half of the backend
// promotion: without the CLI that performs the install, the plan stays what the
// generic pipeline made it and the user is told what is missing.
func TestCopilotWithoutItsCLIIsNotPromoted(t *testing.T) {
	t.Parallel()
	client := detectedClient(domain.ClientCopilot, filepath.Join(t.TempDir(), ".copilot"))
	plan, err := testPlanner(Planner{ManagedRoot: t.TempDir()}).Plan(context.Background(), testEnvelope(), client, domain.ScopeUser, "demo-0123456789ab")
	if err != nil {
		t.Fatal(err)
	}
	if plan.Status == domain.PlanReady || plan.Activation == domain.ActivationPrepared {
		t.Fatalf("promoted without the Copilot CLI: %s/%s", plan.Status, plan.Activation)
	}
	if !contains(plan.UserActions, "GitHub Copilot CLI is required for automatic activation") {
		t.Fatalf("user actions = %v", plan.UserActions)
	}

	client.ExecutablePath = filepath.Join(t.TempDir(), "copilot")
	promoted, err := testPlanner(Planner{ManagedRoot: t.TempDir()}).Plan(context.Background(), testEnvelope(), client, domain.ScopeUser, "demo-0123456789ab")
	if err != nil {
		t.Fatal(err)
	}
	if promoted.Status != domain.PlanReady || promoted.Activation != domain.ActivationPrepared {
		t.Fatalf("Copilot with its CLI = %s/%s", promoted.Status, promoted.Activation)
	}
	if !contains(promoted.UserActions, "agentplugins will install and verify the plugin through GitHub Copilot CLI automatically") {
		t.Fatalf("user actions = %v", promoted.UserActions)
	}
}

func TestKiroMCPPlanRequiresCurrentCLIForAutomaticActivation(t *testing.T) {
	t.Parallel()
	client := detectedClient(domain.ClientKiro, filepath.Join(t.TempDir(), ".kiro"))
	planner := testPlanner(Planner{ManagedRoot: t.TempDir()})
	manual, err := planner.Plan(context.Background(), testEnvelope(), client, domain.ScopeUser, "demo-0123456789ab")
	if err != nil {
		t.Fatal(err)
	}
	if manual.Status != domain.PlanManualActivationRequired || manual.Activation != domain.ActivationManual {
		t.Fatalf("Kiro MCP without CLI claimed automatic activation: %s/%s", manual.Status, manual.Activation)
	}
	if contains(manual.UserActions, "agentplugins will install and verify the package's global Kiro skills and MCP servers automatically") {
		t.Fatalf("manual Kiro plan promised automatic installation: %v", manual.UserActions)
	}

	client.ExecutablePath = filepath.Join(t.TempDir(), "kiro-cli")
	automatic, err := planner.Plan(context.Background(), testEnvelope(), client, domain.ScopeUser, "demo-0123456789ab")
	if err != nil {
		t.Fatal(err)
	}
	if automatic.Status != domain.PlanReady || automatic.Activation != domain.ActivationPrepared {
		t.Fatalf("Kiro MCP with CLI was not promoted: %s/%s", automatic.Status, automatic.Activation)
	}
}

const fixtureContext7AppID = "asdk_app_0123456789abcdef0123456789abcdef"

// TestPersonalChatGPTMappingReplacesCatalogEvidenceOrIsRejected covers the only
// authorizer in the registry: a user's own registration receipt stands in for
// pinned catalog evidence, and a receipt that does not describe the projection
// in hand is an error rather than a quietly unauthorized plan.
func TestPersonalChatGPTMappingReplacesCatalogEvidenceOrIsRejected(t *testing.T) {
	t.Parallel()
	envelope := context7Envelope(fixtureContext7AppID)
	plan, err := testPlanner(Planner{ManagedRoot: t.TempDir()}).Plan(context.Background(), envelope, domain.DetectedClient{ClientID: domain.ClientChatGPT}, domain.ScopeUser, "context7-0123456789ab")
	if err != nil {
		t.Fatal(err)
	}
	if !plan.LocalPreparationAuthorized || !plan.PersonalChatGPTPreparation {
		t.Fatalf("validated receipt did not authorize preparation: %+v", plan)
	}
	if plan.Authentication != domain.AuthenticationNotRequired {
		t.Fatalf("authentication = %s", plan.Authentication)
	}
	if !contains(plan.Warnings, "personal_registration_requires_account_install") {
		t.Fatalf("warnings = %v", plan.Warnings)
	}

	// The receipt names an app the projection does not carry.
	mismatched := context7Envelope(fixtureContext7AppID)
	mismatched.App.Bindings["context7"] = domain.AppBinding{Alias: "context7", ID: "asdk_app_ffffffffffffffffffffffffffffffff"}
	if _, err := testPlanner(Planner{ManagedRoot: t.TempDir()}).Plan(context.Background(), mismatched, domain.DetectedClient{ClientID: domain.ClientChatGPT}, domain.ScopeUser, "context7-0123456789ab"); err == nil {
		t.Fatal("a receipt that does not match the projection was accepted")
	}

	// The receipt is not the canonical Context7 package.
	foreign := context7Envelope(fixtureContext7AppID)
	foreign.Source.Repository = "someone/else"
	if _, err := testPlanner(Planner{ManagedRoot: t.TempDir()}).Plan(context.Background(), foreign, domain.DetectedClient{ClientID: domain.ClientChatGPT}, domain.ScopeUser, "context7-0123456789ab"); err == nil {
		t.Fatal("a receipt for a different package was accepted")
	}
}

func context7Envelope(appID string) domain.PackageEnvelope {
	return domain.PackageEnvelope{
		Manifest: domain.PluginManifest{Name: "context7"},
		Source:   domain.SourceIdentity{Repository: "upstash/context7", PackageSubpath: "plugins/agent-plugins/context7"},
		MCP: domain.MCPComponent{Present: true, Enabled: true, Servers: map[string]domain.MCPServer{
			"context7": {Name: "context7", Type: "streamable-http", Decoded: map[string]any{"url": domain.Context7OAuthURL}},
		}},
		App: domain.AppComponent{Present: true, Declared: true, Enabled: true, Bindings: map[string]domain.AppBinding{
			"context7": {Alias: "context7", ID: appID},
		}},
		LocalChatGPTMapping: &domain.ChatGPTLocalMapping{
			ProductID: "context7", Repository: "upstash/context7", PackagePath: "plugins/agent-plugins/context7",
			Server: "context7", URL: domain.Context7ChatGPTURL, AppID: appID,
		},
	}
}
