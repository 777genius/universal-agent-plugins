package planner

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestPlannerNegotiatesPartialSupportWithoutRejectingSupportedComponents(t *testing.T) {
	t.Parallel()
	planner := Planner{ManagedRoot: t.TempDir()}
	envelope := testEnvelope()
	client := detectedClient(domain.ClientCodex, filepath.Join(t.TempDir(), ".codex"))
	plan, err := planner.Plan(context.Background(), envelope, client, domain.ScopeUser, "demo-0123456789ab")
	if err != nil {
		t.Fatal(err)
	}
	if plan.Status != domain.PlanManualActivationRequired || plan.PackageMode != domain.PackageProjection {
		t.Fatalf("plan = %+v", plan)
	}
	if supportOf(plan, domain.ComponentSkill, "docs") != domain.SupportProjected {
		t.Fatal("skill was not projected")
	}
	if supportOf(plan, domain.ComponentExtension, "cursor") != domain.SupportUnsupported {
		t.Fatal("unsupported extension was not isolated")
	}
	if plan.Authentication != domain.AuthenticationNotChecked {
		t.Fatalf("authentication = %s", plan.Authentication)
	}
}

func TestPlannerAuthenticationRequiresAffirmativePerClientCatalogEvidence(t *testing.T) {
	t.Parallel()
	client := detectedClient(domain.ClientCursor, filepath.Join(t.TempDir(), ".cursor"))
	for name, test := range map[string]struct {
		evidence *domain.CatalogEvidence
		want     domain.AuthenticationState
	}{
		"external_without_catalog": {want: domain.AuthenticationNotChecked},
		"catalog_required": {evidence: &domain.CatalogEvidence{Compatibility: map[string]domain.CatalogCompatibility{
			"cursor": {Package: "native", Verification: "tested", Authentication: domain.AuthenticationRequirementRequired},
		}}, want: domain.AuthenticationPending},
		"catalog_not_required": {evidence: &domain.CatalogEvidence{Compatibility: map[string]domain.CatalogCompatibility{
			"cursor": {Package: "native", Verification: "tested", Authentication: domain.AuthenticationRequirementNotRequired},
		}}, want: domain.AuthenticationNotRequired},
	} {
		name, test := name, test
		t.Run(name, func(t *testing.T) {
			envelope := domain.PackageEnvelope{Manifest: domain.PluginManifest{Name: "metadata-only"}, CatalogEvidence: test.evidence}
			plan, err := (Planner{ManagedRoot: t.TempDir()}).Plan(context.Background(), envelope, client, domain.ScopeUser, "demo-0123456789ab")
			if err != nil {
				t.Fatal(err)
			}
			if plan.Authentication != test.want {
				t.Fatalf("authentication = %s, want %s", plan.Authentication, test.want)
			}
		})
	}
}

func TestPlannerFailsClosedWhenPinnedCatalogEvidenceOmitsSelectedClient(t *testing.T) {
	t.Parallel()
	envelope := testEnvelope()
	envelope.CatalogEvidence = &domain.CatalogEvidence{Compatibility: map[string]domain.CatalogCompatibility{
		"codex": {Package: "projected", Verification: "tested", Authentication: domain.AuthenticationRequirementNotRequired},
	}}
	plan, err := (Planner{ManagedRoot: t.TempDir()}).Plan(
		context.Background(), envelope,
		detectedClient(domain.ClientCursor, filepath.Join(t.TempDir(), ".cursor")),
		domain.ScopeUser, "demo-0123456789ab",
	)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Status != domain.PlanUnsupported || plan.Activation != domain.ActivationFailed || !contains(plan.Warnings, "client_compatibility_not_catalog_verified") {
		t.Fatalf("plan = %+v", plan)
	}
}

func TestPlannerKeepsManualVerificationGuidanceForUntrustedEvidence(t *testing.T) {
	t.Parallel()
	envelope := testEnvelope()
	envelope.CatalogEvidence = &domain.CatalogEvidence{Compatibility: map[string]domain.CatalogCompatibility{
		"cursor": {Package: "native", Verification: "not_tested", Authentication: domain.AuthenticationRequirementNotRequired,
			Evidence: []domain.DirectoryEvidence{{Level: "runtime", Outcome: "passed"}}},
	}}
	plan, err := (Planner{ManagedRoot: t.TempDir()}).Plan(context.Background(), envelope,
		detectedClient(domain.ClientCursor, filepath.Join(t.TempDir(), ".cursor")), domain.ScopeUser, "demo-0123456789ab")
	if err != nil {
		t.Fatal(err)
	}
	if !contains(plan.Warnings, "catalog_not_tested") || !contains(plan.UserActions, "verify the plugin in the selected client before relying on it") {
		t.Fatalf("planner suppressed verification guidance: warnings=%v actions=%v", plan.Warnings, plan.UserActions)
	}
}

func TestPlannerSuppressesManualVerificationOnlyForTrustedRuntimePass(t *testing.T) {
	t.Parallel()
	client := detectedClient(domain.ClientCursor, filepath.Join(t.TempDir(), ".cursor"))
	trusted := func(level string) domain.DirectoryEvidence {
		artifact := domain.DirectoryEvidenceArtifact{Repository: "owner/evidence", Revision: strings.Repeat("a", 40)}
		evidence := domain.DirectoryEvidence{Level: level, Outcome: "passed", Client: domain.ClientCursor, Artifact: artifact}
		evidence.Trust = &domain.DirectoryEvidenceTrust{Kind: "reviewed_external"}
		return evidence
	}
	for _, level := range []string{"materialization", "discovery", "oauth", "runtime"} {
		t.Run(level, func(t *testing.T) {
			envelope := testEnvelope()
			envelope.CatalogEvidence = &domain.CatalogEvidence{Compatibility: map[string]domain.CatalogCompatibility{
				"cursor": {Package: "native", Verification: "tested", Authentication: domain.AuthenticationRequirementNotRequired, Evidence: []domain.DirectoryEvidence{trusted(level)}},
			}}
			plan, err := (Planner{ManagedRoot: t.TempDir()}).Plan(context.Background(), envelope, client, domain.ScopeUser, "demo-0123456789ab")
			if err != nil {
				t.Fatal(err)
			}
			hasAction := contains(plan.UserActions, "verify the plugin in the selected client before relying on it")
			if hasAction != (level != "runtime") {
				t.Fatalf("level %s manual runtime verification action = %t, actions=%v", level, hasAction, plan.UserActions)
			}
		})
	}
}

func TestPlannerCarriesNonFatalLoaderDiagnosticsToPlan(t *testing.T) {
	t.Parallel()
	envelope := testEnvelope()
	envelope.Diagnostics = []domain.Diagnostic{{Severity: domain.SeverityWarning, Boundary: domain.BoundaryPlugin, Code: "plugin_unknown_field", Message: "future field preserved"}}
	plan, err := (Planner{ManagedRoot: t.TempDir()}).Plan(context.Background(), envelope, detectedClient(domain.ClientCursor, filepath.Join(t.TempDir(), ".cursor")), domain.ScopeUser, "demo-0123456789ab")
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Diagnostics) != 1 || plan.Diagnostics[0].Message != "future field preserved" || !contains(plan.Warnings, "plugin_unknown_field") {
		t.Fatalf("plan diagnostics = %+v warnings=%v", plan.Diagnostics, plan.Warnings)
	}
}

func TestPlannerUsesNativeCursorTargetAndDoesNotExposeItInJSON(t *testing.T) {
	t.Parallel()
	config := filepath.Join(t.TempDir(), ".cursor")
	planner := Planner{ManagedRoot: t.TempDir()}
	plan, err := planner.Plan(context.Background(), testEnvelope(), detectedClient(domain.ClientCursor, config), domain.ScopeUser, "demo-0123456789ab")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(config, "plugins", "local", "demo-0123456789ab")
	if plan.ActivePath != want || plan.Status != domain.PlanManualActivationRequired || plan.PackageMode != domain.PackageNative {
		t.Fatalf("plan = %+v, want active %s", plan, want)
	}
	if plan.DeclaredName != "demo" || plan.NativeRegistryRoot != config {
		t.Fatalf("native identity inputs = name %q root %q", plan.DeclaredName, plan.NativeRegistryRoot)
	}
	body, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), config) {
		t.Fatalf("public JSON leaked path: %s", body)
	}
}

func TestPlannerPromotesVSCodeToReadyWhenCopilotIsDetected(t *testing.T) {
	t.Parallel()
	client := detectedClient(domain.ClientVSCode, filepath.Join(t.TempDir(), "Code", "User"))
	withoutBridge := Planner{ManagedRoot: t.TempDir()}
	manual, err := withoutBridge.Plan(context.Background(), testEnvelope(), client, domain.ScopeUser, "demo-0123456789ab")
	if err != nil {
		t.Fatal(err)
	}
	if manual.Status != domain.PlanManualActivationRequired || manual.PackageMode != domain.PackagePrepared {
		t.Fatalf("manual plan = %+v", manual)
	}
	withBridge := Planner{
		ManagedRoot: t.TempDir(),
		Detected: map[domain.ClientID]domain.DetectedClient{
			domain.ClientCopilot: {
				ClientID: domain.ClientCopilot, Status: domain.DetectionDetected, ConfigRoot: "/test/home/.copilot", ExecutablePath: "/test/bin/copilot",
			},
		},
	}
	bridged, err := withBridge.Plan(context.Background(), testEnvelope(), client, domain.ScopeUser, "demo-0123456789ab")
	if err != nil {
		t.Fatal(err)
	}
	if bridged.Status != domain.PlanReady || bridged.PackageMode != domain.PackagePrepared || bridged.Activation != domain.ActivationPrepared {
		t.Fatalf("bridged plan = %+v", bridged)
	}
	if bridged.NativeRegistryExecutable != "/test/bin/copilot" || bridged.NativeRegistryRoot != "/test/home/.copilot" {
		t.Fatalf("shared native registry = executable %q root %q", bridged.NativeRegistryExecutable, bridged.NativeRegistryRoot)
	}
}

func TestPlannerPromotesDetectedCopilotExecutableToReady(t *testing.T) {
	t.Parallel()
	client := detectedClient(domain.ClientCopilot, filepath.Join(t.TempDir(), ".copilot"))
	client.ExecutablePath = "/test/bin/copilot"
	plan, err := (Planner{ManagedRoot: t.TempDir()}).Plan(
		context.Background(), testEnvelope(), client, domain.ScopeUser, "demo-0123456789ab",
	)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Status != domain.PlanReady || plan.Activation != domain.ActivationPrepared {
		t.Fatalf("plan = %+v", plan)
	}
}

func TestDetectedPhysicalClientUsesRealSharedSurfaceIdentity(t *testing.T) {
	t.Parallel()
	vscode := detectedClient(domain.ClientVSCode, filepath.Join(t.TempDir(), "Code", "User"))
	detected := map[domain.ClientID]domain.DetectedClient{domain.ClientVSCode: vscode}

	client, ok := DetectedPhysicalClient(domain.ClientCopilot, detected)
	if !ok || client.ClientID != domain.ClientVSCode || client.ConfigRoot != vscode.ConfigRoot {
		t.Fatalf("shared physical client = %+v, %v", client, ok)
	}
	if _, ok := DetectedPhysicalClient(domain.ClientCursor, detected); ok {
		t.Fatal("unrelated detected client was accepted for Cursor binding")
	}
}

func TestPlannerFailsClosedForUndetectedClientAndUnsupportedScope(t *testing.T) {
	t.Parallel()
	planner := Planner{ManagedRoot: t.TempDir()}
	client := domain.DetectedClient{ClientID: domain.ClientCursor, Status: domain.DetectionNotDetected}
	plan, err := planner.Plan(context.Background(), testEnvelope(), client, domain.ScopeUser, "demo-0123456789ab")
	if err != nil {
		t.Fatal(err)
	}
	if plan.Status != domain.PlanUnsupported || !contains(plan.Warnings, "client_not_detected") {
		t.Fatalf("undetected plan = %+v", plan)
	}
	client = detectedClient(domain.ClientCursor, filepath.Join(t.TempDir(), ".cursor"))
	plan, err = planner.Plan(context.Background(), testEnvelope(), client, domain.ScopeProject, "demo-0123456789ab")
	if err != nil {
		t.Fatal(err)
	}
	if plan.Status != domain.PlanUnsupported || !contains(plan.Warnings, "scope_not_supported") {
		t.Fatalf("project plan = %+v", plan)
	}
}

func TestChatGPTRemoteTargetSupportsSkillsWithoutDesktopDetection(t *testing.T) {
	t.Parallel()
	envelope := domain.PackageEnvelope{
		Manifest: domain.PluginManifest{Name: "skills-only"},
		Skills:   map[string]domain.Skill{"docs": {Name: "docs"}},
	}
	client := domain.DetectedClient{ClientID: domain.ClientChatGPT, Status: domain.DetectionNotDetected}
	plan, err := (Planner{ManagedRoot: t.TempDir()}).Plan(context.Background(), envelope, client, domain.ScopeUser, "skills-only-0123456789ab")
	if err != nil {
		t.Fatal(err)
	}
	if plan.Status != domain.PlanManualActivationRequired || supportOf(plan, domain.ComponentSkill, "docs") != domain.SupportProjected || contains(plan.Warnings, "client_not_detected") {
		t.Fatalf("ChatGPT skills plan = %+v", plan)
	}
	if !containsText(plan.UserActions, "skills-only") || containsText(plan.UserActions, ".app.json") || containsText(plan.UserActions, "registered app") {
		t.Fatalf("ChatGPT skills actions = %+v", plan.UserActions)
	}
}

func TestOnlyChatGPTProjectsRegisteredAppBindings(t *testing.T) {
	t.Parallel()
	for _, client := range []domain.ClientID{domain.ClientCodex, domain.ClientChatGPT, domain.ClientCursor, domain.ClientCopilot, domain.ClientVSCode, domain.ClientKiro} {
		capabilities, ok := Capabilities(client)
		if !ok {
			t.Fatalf("missing capabilities for %s", client)
		}
		want := domain.SupportUnsupported
		if client == domain.ClientChatGPT {
			want = domain.SupportProjected
		}
		if capabilities.AppSupport != want {
			t.Fatalf("%s app support = %q, want %q", client, capabilities.AppSupport, want)
		}
	}
}

func TestPlannerReadsNewClientCapabilitiesFromRegistry(t *testing.T) {
	t.Parallel()
	claude, ok := Capabilities(domain.ClientClaude)
	if !ok || claude.PackageMode != domain.PackageProjection || claude.ActivationMode != domain.ActivationAutomatic || claude.SkillSupport != domain.SupportProjected || claude.MCPTransports["stdio"] != domain.SupportProjected {
		t.Fatalf("Claude capabilities = %+v", claude)
	}
	for _, client := range []domain.ClientID{domain.ClientOpenCode, domain.ClientWindsurf} {
		capabilities, ok := Capabilities(client)
		if !ok || capabilities.PackageMode != domain.PackagePrepared || capabilities.SkillSupport != domain.SupportPrepared || capabilities.MCPTransports["stdio"] != domain.SupportPrepared {
			t.Fatalf("%s capabilities = %+v", client, capabilities)
		}
	}
	cline, ok := Capabilities(domain.ClientCline)
	if !ok || cline.PackageMode != domain.PackageNative || cline.ActivationMode != domain.ActivationAutomatic || cline.SkillSupport != domain.SupportNative || cline.MCPTransports["stdio"] != domain.SupportNative {
		t.Fatalf("Cline capabilities = %+v", cline)
	}
	gemini, ok := Capabilities(domain.ClientGemini)
	if !ok || gemini.PackageMode != domain.PackageNative || gemini.SkillSupport != domain.SupportNative || gemini.MCPTransports["stdio"] != domain.SupportNative {
		t.Fatalf("Gemini capabilities = %+v", gemini)
	}
}

func TestChatGPTMCPFailsClosedWithoutValidAppBinding(t *testing.T) {
	t.Parallel()
	envelope := domain.PackageEnvelope{
		Manifest: domain.PluginManifest{Name: "remote-mcp"},
		MCP: domain.MCPComponent{Present: true, Enabled: true, Servers: map[string]domain.MCPServer{
			"docs": {Name: "docs", Type: "streamable-http"},
		}},
	}
	plan, err := (Planner{ManagedRoot: t.TempDir()}).Plan(context.Background(), envelope, domain.DetectedClient{ClientID: domain.ClientChatGPT}, domain.ScopeUser, "remote-mcp-0123456789ab")
	if err != nil {
		t.Fatal(err)
	}
	if plan.Status != domain.PlanUnsupported || !contains(plan.Warnings, "chatgpt_app_binding_required") {
		t.Fatalf("ChatGPT missing app plan = %+v", plan)
	}
}

func TestChatGPTMCPFailsClosedWhenAppAliasDoesNotMapServer(t *testing.T) {
	t.Parallel()
	envelope := domain.PackageEnvelope{
		Manifest: domain.PluginManifest{Name: "remote-mcp"},
		MCP: domain.MCPComponent{Present: true, Enabled: true, Servers: map[string]domain.MCPServer{
			"docs": {Name: "docs", Type: "streamable-http"},
		}},
		App: domain.AppComponent{Present: true, Declared: true, Enabled: true, Bindings: map[string]domain.AppBinding{
			"different": {Alias: "different", ID: "plugin_asdk_app_different_123"},
		}},
	}
	plan, err := (Planner{ManagedRoot: t.TempDir()}).Plan(context.Background(), envelope, domain.DetectedClient{ClientID: domain.ClientChatGPT}, domain.ScopeUser, "remote-mcp-0123456789ab")
	if err != nil {
		t.Fatal(err)
	}
	if plan.Status != domain.PlanUnsupported || !contains(plan.Warnings, "chatgpt_app_binding_required") || supportOf(plan, domain.ComponentMCPServer, "docs") != domain.SupportUnsupported || !containsText(plan.UserActions, "docs") {
		t.Fatalf("ChatGPT mismatched app plan = %+v", plan)
	}
}

func TestChatGPTProjectsMCPThroughRegisteredAppBinding(t *testing.T) {
	t.Parallel()
	envelope := domain.PackageEnvelope{
		Manifest: domain.PluginManifest{Name: "remote-mcp"},
		MCP: domain.MCPComponent{Present: true, Enabled: true, Servers: map[string]domain.MCPServer{
			"docs": {Name: "docs", Type: "streamable-http"},
		}},
		App: domain.AppComponent{Present: true, Declared: true, Enabled: true, Bindings: map[string]domain.AppBinding{
			"docs": {Alias: "docs", ID: "asdk_app_docs_123"},
		}},
	}
	plan, err := (Planner{ManagedRoot: t.TempDir()}).Plan(context.Background(), envelope, domain.DetectedClient{ClientID: domain.ClientChatGPT}, domain.ScopeUser, "remote-mcp-0123456789ab")
	if err != nil {
		t.Fatal(err)
	}
	if plan.Status != domain.PlanManualActivationRequired || supportOf(plan, domain.ComponentMCPServer, "docs") != domain.SupportProjected || supportOf(plan, domain.ComponentApp, "docs") != domain.SupportProjected {
		t.Fatalf("ChatGPT app plan = %+v", plan)
	}
}

func TestPlannerRejectsMetadataOnlyProjectionWhenAllDeclaredComponentsAreInvalid(t *testing.T) {
	t.Parallel()
	for name, diagnostic := range map[string]domain.Diagnostic{
		"malformed_only_mcp": {
			Severity: domain.SeverityError, Boundary: domain.BoundaryMCP,
			Code: "mcp_malformed", Message: "malformed MCP document",
		},
		"all_invalid_skills": {
			Severity: domain.SeverityError, Boundary: domain.BoundarySkill,
			Code: "skill_invalid", Message: "all skills are invalid",
		},
	} {
		name, diagnostic := name, diagnostic
		t.Run(name, func(t *testing.T) {
			envelope := domain.PackageEnvelope{
				Manifest:    domain.PluginManifest{Name: "broken"},
				Diagnostics: []domain.Diagnostic{diagnostic},
			}
			plan, err := (Planner{ManagedRoot: t.TempDir()}).Plan(
				context.Background(), envelope,
				detectedClient(domain.ClientCursor, filepath.Join(t.TempDir(), ".cursor")),
				domain.ScopeUser, "broken-0123456789ab",
			)
			if err != nil {
				t.Fatal(err)
			}
			if plan.Status != domain.PlanUnsupported || plan.Activation != domain.ActivationFailed || !contains(plan.Warnings, "no_valid_components") {
				t.Fatalf("plan = %+v", plan)
			}
		})
	}
}

func TestPlannerDoesNotAppendActivationOrAuthenticationGuidanceWhenUnsupported(t *testing.T) {
	t.Parallel()
	tests := map[string]domain.PackageEnvelope{
		"catalog_omits_client": func() domain.PackageEnvelope {
			envelope := testEnvelope()
			envelope.CatalogEvidence = &domain.CatalogEvidence{Compatibility: map[string]domain.CatalogCompatibility{
				"codex": {Package: "projected", Verification: "tested", Authentication: domain.AuthenticationRequirementNotRequired},
			}}
			return envelope
		}(),
		"catalog_package_mismatch": func() domain.PackageEnvelope {
			envelope := testEnvelope()
			envelope.CatalogEvidence = &domain.CatalogEvidence{Compatibility: map[string]domain.CatalogCompatibility{
				"cursor": {Package: "projected", Verification: "schema_only", Authentication: domain.AuthenticationRequirementRequired},
			}}
			return envelope
		}(),
		"invalid_components": {
			Manifest: domain.PluginManifest{Name: "broken"},
			Diagnostics: []domain.Diagnostic{{
				Severity: domain.SeverityError, Boundary: domain.BoundarySkill,
				Code: "skill_invalid", Message: "all skills are invalid",
			}},
		},
	}
	for name, envelope := range tests {
		name, envelope := name, envelope
		t.Run(name, func(t *testing.T) {
			plan, err := (Planner{ManagedRoot: t.TempDir()}).Plan(
				context.Background(), envelope,
				detectedClient(domain.ClientCursor, filepath.Join(t.TempDir(), ".cursor")),
				domain.ScopeUser, "broken-0123456789ab",
			)
			if err != nil {
				t.Fatal(err)
			}
			if plan.Status != domain.PlanUnsupported {
				t.Fatalf("plan = %+v", plan)
			}
			for _, action := range plan.UserActions {
				if strings.Contains(action, "authentication") || strings.Contains(action, "reload Cursor") || strings.Contains(action, "verify the plugin") {
					t.Fatalf("unsupported plan appended lifecycle guidance %q: %+v", action, plan)
				}
			}
		})
	}
}

func testEnvelope() domain.PackageEnvelope {
	return domain.PackageEnvelope{
		Manifest: domain.PluginManifest{
			Name:       "demo",
			Extensions: map[string]json.RawMessage{"cursor": json.RawMessage(`{"enabled":true}`)},
		},
		MCP: domain.MCPComponent{
			Present: true, Enabled: true,
			Servers: map[string]domain.MCPServer{
				"remote": {Name: "remote", Type: "streamable-http"},
			},
		},
		Skills: map[string]domain.Skill{"docs": {Name: "docs"}},
	}
}

func detectedClient(id domain.ClientID, config string) domain.DetectedClient {
	return domain.DetectedClient{ClientID: id, Status: domain.DetectionDetected, ConfigRoot: config}
}

func supportOf(plan domain.DeliveryPlan, kind domain.ComponentKind, name string) domain.SupportLevel {
	for _, component := range plan.Components {
		if component.Kind == kind && component.Name == name {
			return component.Support
		}
	}
	return ""
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func containsText(values []string, fragment string) bool {
	for _, value := range values {
		if strings.Contains(value, fragment) {
			return true
		}
	}
	return false
}
