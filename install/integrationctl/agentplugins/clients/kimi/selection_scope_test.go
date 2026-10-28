package kimi

import (
	"path/filepath"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/clients"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/agentplugins/domain"
)

func TestManagedMCPSelectionHasExplicitScopeAndKeepsUserAPI(t *testing.T) {
	adapter := New()
	user := clients.SelectionLayout{File: filepath.Join(".kimi-plugin", "plugin.json"), Nested: true, ServersMemberOptional: true}
	project := clients.SelectionLayout{File: "mcp.json", Nested: true}
	if adapter.ManagedMCPSelection() != user {
		t.Fatal("legacy user selection policy changed")
	}
	for scope, want := range map[domain.InstallScope]clients.SelectionLayout{domain.ScopeUser: user, domain.ScopeProject: project} {
		got, err := adapter.ManagedMCPSelectionForScope(scope)
		if err != nil || got != want {
			t.Fatalf("scope %q layout=%+v err=%v", scope, got, err)
		}
	}
	for _, scope := range []domain.InstallScope{"", "unknown", "PROJECT"} {
		if _, err := adapter.ManagedMCPSelectionForScope(scope); err == nil {
			t.Fatalf("unrecognized scope %q accepted", scope)
		}
	}
}
