package platformexec

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

type cursorWorkspaceAdapter struct{}

const removedCursorRulesFileName = "." + "cursor" + "rules"
const (
	cursorAgentsSectionStart = "<!-- plugin-kit-ai:cursor-agents:start -->"
	cursorAgentsSectionEnd   = "<!-- plugin-kit-ai:cursor-agents:end -->"
)

func CursorAgentsSectionStart() string { return cursorAgentsSectionStart }

func CursorAgentsSectionEnd() string { return cursorAgentsSectionEnd }

func (cursorWorkspaceAdapter) ID() string { return "cursor-workspace" }

func (cursorWorkspaceAdapter) DetectNative(root string) bool {
	if fileExists(filepath.Join(root, ".cursor", "mcp.json")) {
		return true
	}
	return len(discoverFiles(root, filepath.Join(".cursor", "rules"), nil)) > 0
}

func (cursorWorkspaceAdapter) RefineDiscovery(root string, state *pluginmodel.TargetState) error {
	for _, rel := range state.ComponentPaths("rules") {
		if strings.ToLower(filepath.Ext(rel)) != ".mdc" {
			return fmt.Errorf("unsupported Cursor rule file %s: use .mdc", rel)
		}
	}
	return nil
}

func (cursorWorkspaceAdapter) ManagedPaths(root string, graph pluginmodel.PackageGraph, state pluginmodel.TargetState) ([]string, error) {
	return []string{"AGENTS.md"}, nil
}
