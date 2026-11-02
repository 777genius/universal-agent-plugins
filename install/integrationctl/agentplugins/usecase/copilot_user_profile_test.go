package usecase

import (
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestNewCopilotUserBindingRetainsSelectedProfile(t *testing.T) {
	root := t.TempDir()
	input := AddInput{Client: domain.DetectedClient{ClientID: domain.ClientCopilot, ConfigRoot: root}, Scope: domain.ScopeUser}
	plan := domain.DeliveryPlan{ClientID: domain.ClientCopilot, Scope: domain.ScopeUser, ActivePath: filepath.Join(t.TempDir(), "private-artifact")}
	binding := stagedClientBinding(domain.ClientBinding{}, false, input, plan, "new-binding", "now")
	if binding.NativeProfileRoot != root {
		t.Fatalf("new Copilot binding lost captured profile: got %q want %q", binding.NativeProfileRoot, root)
	}
	for _, prior := range []domain.ClientBinding{
		{},
		{ClientID: "copilot", ClientBindingID: "legacy"},
		{ClientID: "copilot", ClientBindingID: "recorded", NativeProfileRoot: t.TempDir()},
		{ClientID: "vscode", ClientBindingID: "sibling"},
		{ClientID: "vscode", ClientBindingID: "sibling-recorded", NativeProfileRoot: t.TempDir()},
	} {
		got := stagedClientBinding(prior, true, input, plan, "new-binding", "now")
		if got.NativeProfileRoot != prior.NativeProfileRoot || (prior.ClientID != "" && got.ClientID != prior.ClientID) {
			t.Fatalf("existing binding adopted requested Copilot profile: prior=%+v got=%+v", prior, got)
		}
	}
	input.Client.ClientID = domain.ClientVSCode
	if got := stagedClientBinding(domain.ClientBinding{}, false, input, plan, "new-binding", "now"); got.NativeProfileRoot != "" {
		t.Fatal("VS Code inherited Copilot profile recording")
	}

	input.Client.ClientID, input.Scope = domain.ClientCopilot, domain.ScopeProject
	plan.Scope, plan.NativeRegistryRoot = domain.ScopeProject, t.TempDir()
	if got := stagedClientBinding(domain.ClientBinding{}, false, input, plan, "project-binding", "now"); got.NativeProfileRoot != plan.NativeRegistryRoot {
		t.Fatalf("project binding borrowed user profile: got %q want %q", got.NativeProfileRoot, plan.NativeRegistryRoot)
	}
}
