package usecase

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/cursorhooks"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/providers"
)

// A selected Stop needs its owned locator without inheriting unselected MCP requirements.
func TestCursorStopOnlyRequiresOwnedDataWithoutMCP(t *testing.T) {
	for _, unsupportedMCP := range []bool{false, true} {
		root := t.TempDir()
		spec := cursorhooks.HookSpec{Executable: filepath.Join(root, "TEST-observer"), Selector: filepath.Join(root, "TEST-data", "TEST-binding")}
		result, err := cursorhooks.Plan(cursorhooks.Request{Operation: cursorhooks.Install, Specs: []cursorhooks.HookSpec{spec}, Shell: cursorhooks.DarwinUserShell20260918, ExecutableVerified: true})
		if err != nil {
			t.Fatal(err)
		}
		r := result.Receipt
		canonical := "sha256:" + strings.Repeat("a", 64)
		selected, err := domain.NewCursorDelivery(domain.CursorDeliveryFacts{ProfileRoot: root, HooksPath: filepath.Join(root, "hooks.json"), ProfileIdentity: root, CursorVersion: "2026.09.18-9a7762b", TargetOS: "darwin", TargetArch: "arm64", QualificationID: "TEST-stop-only", Executable: spec.Executable, Selector: spec.Selector, Shell: string(r.Shell), ObjectID: "TEST-stop", EntryDigest: r.EntryDigest, CanonicalDigest: canonical, PlannedReceipt: domain.CursorHookReceipt{Version: r.Version, Event: r.Event, Executable: spec.Executable, Selector: spec.Selector, Shell: string(r.Shell), EntryDigest: r.EntryDigest, RemainderDigest: r.RemainderDigest}, OriginalRawDigest: "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"})
		if err != nil {
			t.Fatal(err)
		}
		plan := domain.DeliveryPlan{ClientID: domain.ClientCursor, Scope: domain.ScopeUser, NativeRegistryRoot: root, ActivePath: filepath.Join(root, "TEST-projection"), SelectedDelivery: selected, Components: []domain.ComponentDecision{{Kind: domain.ComponentSkill, Name: "TEST-skill", Support: domain.SupportNative}}}
		envelope := domain.PackageEnvelope{TreeDigest: canonical, MCP: domain.MCPComponent{Servers: map[string]domain.MCPServer{}}}
		if unsupportedMCP {
			envelope.MCP.Servers["TEST-manual"] = domain.MCPServer{Type: "stdio", Decoded: map[string]any{"command": "./TEST-missing-runtime"}}
			plan.Components = append(plan.Components, domain.ComponentDecision{Kind: domain.ComponentMCPServer, Name: "TEST-manual", Support: domain.SupportUnsupported})
		}
		service := Service{Stager: providers.Stager{}}
		if err := service.preflightComponents(envelope, &plan, false); err != nil {
			t.Fatalf("unselected MCP forced runtime: %v", err)
		}
		if !packageNeedsPluginData(envelope, plan) {
			t.Fatalf("Stop-only lacks owned locator, unsupportedMCP=%t", unsupportedMCP)
		}
		plan.SelectedDelivery = domain.SelectedDelivery{}
		if packageNeedsPluginData(envelope, plan) {
			t.Fatal("unselected unsupported MCP granted owned data")
		}
	}
}
