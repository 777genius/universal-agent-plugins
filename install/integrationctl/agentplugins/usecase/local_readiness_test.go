package usecase

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
)

// Red: the historical vscode ClientID does not require managed stdio, so Local
// MCP could pass preflight without its helper. The real stager refuses it; native
// only selection must avoid both the helper and the authored missing runtime.
func TestLocalReadinessRequiresHelperOnlyForSelectedMCP(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "bin"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "bin", "TEST-runtime"), []byte("TEST fixture not executed"), 0700); err != nil {
		t.Fatal(err)
	}
	enabled := true
	facts := domain.LocalDeliveryFacts{ProfileRoot: root, SettingsPath: filepath.Join(root, "settings.json"), ProfileIdentity: "TEST-profile", SettingsIdentity: "TEST-settings", Tuple: domain.LocalQualifiedTuple{VSCodeVersion: "TEST-code", CopilotVersion: "TEST-copilot", TargetOS: "linux", TargetShell: "bash", QualificationID: "TEST-contract"}, NativeStop: true, CanonicalDigest: "sha256:" + strings.Repeat("a", 64), Registration: domain.OwnedProfileEntry{ObjectID: "entry", Selector: filepath.Join(root, "installed"), DesiredValue: &enabled}}
	envelope := domain.PackageEnvelope{SnapshotRoot: root, TreeDigest: facts.CanonicalDigest, MCP: domain.MCPComponent{Servers: map[string]domain.MCPServer{"notify": {Type: "stdio", Decoded: map[string]any{"command": "./bin/TEST-runtime"}}}}}
	plan := domain.DeliveryPlan{ClientID: domain.ClientVSCode, ActivePath: facts.Registration.Selector, Components: []domain.ComponentDecision{{Kind: domain.ComponentMCPServer, Name: "notify", Support: domain.SupportUnsupported}}}
	var err error
	plan.SelectedDelivery, err = domain.NewLocalDelivery(facts)
	if err != nil {
		t.Fatal(err)
	}
	service := Service{Stager: providers.Stager{}}
	// The unselected source MCP cannot trigger launcher/readiness effects.
	// Native Stop still receives its own existing owned PLUGIN_DATA locator.
	if err := service.preflightComponents(envelope, &plan, false); err != nil || !packageNeedsPluginData(envelope, plan) {
		t.Fatalf("native-only inherited manual requirements: %v", err)
	}
	facts.MCPServers = []string{"notify"}
	plan.Components = []domain.ComponentDecision{{Kind: domain.ComponentMCPServer, Name: "notify", Support: domain.SupportNative}}
	plan.SelectedDelivery, err = domain.NewLocalDelivery(facts)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.preflightComponents(envelope, &plan, false); err == nil || len(plan.Diagnostics) != 1 || plan.Diagnostics[0].Code != "managed_stdio_helper_unavailable" {
		t.Fatalf("Local MCP skipped real launcher preflight: %+v %v", plan.Diagnostics, err)
	}
}
