package opencode

import (
	"context"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	fsadapter "github.com/777genius/plugin-kit-ai/install/integrationctl/adapters/fs"
	"github.com/777genius/plugin-kit-ai/install/integrationctl/domain"
)

func TestLoadSourceMaterialRejectsEmptyInstructionPath(t *testing.T) {
	t.Parallel()

	sourceRoot := t.TempDir()
	mustWriteOpenCodeFile(t, filepath.Join(sourceRoot, "plugin", "targets", "opencode", "instructions.yaml"), "- AGENTS.md\n- \"\"\n")

	adapter := Adapter{FS: fsadapter.OS{}, ProjectRoot: t.TempDir(), UserHome: t.TempDir()}
	_, err := adapter.loadSourceMaterial(context.Background(), sourceRoot, "project", adapter.ProjectRoot)
	if err == nil {
		t.Fatal("expected empty instruction path error")
	}
	if !strings.Contains(err.Error(), "OpenCode instructions must contain only non-empty paths") {
		t.Fatalf("error = %v", err)
	}
}

func TestBuildUpdateMutation_RemovesStaleOwnedKeys(t *testing.T) {
	t.Parallel()

	mutation := buildUpdateMutation(sourceMaterial{
		WholeFields: map[string]any{
			"$schema": "https://opencode.ai/config.json",
			"plugin":  []any{"fresh"},
		},
		Plugins: []pluginRef{{Name: "fresh"}},
		MCP:     map[string]any{"review": map[string]any{"type": "local"}},
	}, domain.TargetInstallation{
		OwnedNativeObjects: []domain.NativeObjectRef{
			{Kind: "opencode_config_key", Name: "instructions"},
			{Kind: "opencode_config_key", Name: "$schema"},
			{Kind: "opencode_plugin_ref", Name: "stale"},
			{Kind: "opencode_mcp_server", Name: "legacy"},
		},
	})

	if !slices.Equal(mutation.WholeRemove, []string{"$schema", "instructions"}) {
		t.Fatalf("WholeRemove = %v", mutation.WholeRemove)
	}
	if !slices.Equal(mutation.PluginsRemove, []string{"stale"}) {
		t.Fatalf("PluginsRemove = %v", mutation.PluginsRemove)
	}
	if !slices.Equal(mutation.MCPRemove, []string{"legacy"}) {
		t.Fatalf("MCPRemove = %v", mutation.MCPRemove)
	}
}
