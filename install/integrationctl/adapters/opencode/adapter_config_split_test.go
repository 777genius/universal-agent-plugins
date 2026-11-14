package opencode

import (
	"strings"
	"testing"

	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
)

func TestExistingPluginRefsRejectsInvalidTuplePluginRef(t *testing.T) {
	t.Parallel()

	_, err := existingPluginRefs([]any{
		[]any{"@acme/plugin", "not-an-object"},
	})
	if err == nil {
		t.Fatal("expected invalid tuple plugin ref error")
	}
	if !strings.Contains(err.Error(), "tuple plugin ref options must be an object") {
		t.Fatalf("error = %v", err)
	}
}

func TestEnsureConfigMutationCompatibleRejectsForeignPluginConflict(t *testing.T) {
	t.Parallel()

	err := ensureConfigMutationCompatible(configPatchState{
		currentPlugins: map[string]pluginRef{
			"review": {Name: "review", Options: map[string]any{"mode": "strict"}},
		},
	}, configMutation{
		PluginsSet: []pluginRef{{Name: "review", Options: map[string]any{"mode": "fast"}}},
	}, nil)
	if err == nil {
		t.Fatal("expected conflict")
	}
	if !strings.Contains(err.Error(), "OpenCode plugin ref conflict for review") {
		t.Fatalf("error = %v", err)
	}
}

func TestEnsureConfigMutationCompatibleAllowsOwnedMCPAliasUpdate(t *testing.T) {
	t.Parallel()

	err := ensureConfigMutationCompatible(configPatchState{
		currentMCP: map[string]any{
			"docs": map[string]any{"type": "remote"},
		},
	}, configMutation{
		MCPSet: map[string]any{
			"docs": map[string]any{"type": "local"},
		},
	}, &domain.TargetInstallation{
		OwnedNativeObjects: []domain.NativeObjectRef{
			{Kind: "opencode_mcp_server", Name: "docs"},
		},
	})
	if err != nil {
		t.Fatalf("error = %v", err)
	}
}
