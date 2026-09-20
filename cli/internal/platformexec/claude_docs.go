package platformexec

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

func claudeUsesGeneratedHooks(graph pluginmodel.PackageGraph, state pluginmodel.TargetState) bool {
	if graph.Launcher == nil || strings.TrimSpace(graph.Launcher.Entrypoint) == "" {
		return false
	}
	return len(state.ComponentPaths("hooks")) == 0
}

func claudeHooksRequireLauncher(graph pluginmodel.PackageGraph, state pluginmodel.TargetState) bool {
	if len(state.ComponentPaths("hooks")) > 0 {
		return true
	}
	return graph.Launcher != nil
}

func claudePackageOnlyMode(graph pluginmodel.PackageGraph, state pluginmodel.TargetState) bool {
	return graph.Launcher == nil && len(state.ComponentPaths("hooks")) == 0 && claudeHasPackageOnlySurface(graph, state)
}

func claudeHasPackageOnlySurface(graph pluginmodel.PackageGraph, state pluginmodel.TargetState) bool {
	if len(graph.Portable.Paths("skills")) > 0 || graph.Portable.MCP != nil {
		return true
	}
	for _, kind := range []string{"settings", "lsp", "user_config", "manifest_extra"} {
		if strings.TrimSpace(state.DocPath(kind)) != "" {
			return true
		}
	}
	for _, kind := range []string{"commands", "agents"} {
		if len(state.ComponentPaths(kind)) > 0 {
			return true
		}
	}
	return false
}

func claudePrimaryHookPath(state pluginmodel.TargetState) string {
	hookPaths := state.ComponentPaths("hooks")
	if len(hookPaths) == 0 {
		return ""
	}
	return hookPaths[0]
}

func claudeManifestManagedPaths() []string {
	return []string{
		"name",
		"version",
		"description",
		"skills",
		"agents",
		"commands",
		"hooks",
		"mcpServers",
		"lspServers",
		"settings",
		"userConfig",
	}
}

func loadClaudeJSONDoc(root, rel, label string) (map[string]any, []byte, bool, error) {
	if strings.TrimSpace(rel) == "" {
		return nil, nil, false, nil
	}
	body, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		return nil, nil, false, fmt.Errorf("%s %s is not readable: %w", label, rel, err)
	}
	var doc map[string]any
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, nil, true, fmt.Errorf("%s %s is invalid JSON: %w", label, rel, err)
	}
	if doc == nil {
		doc = map[string]any{}
	}
	return doc, body, true, nil
}
