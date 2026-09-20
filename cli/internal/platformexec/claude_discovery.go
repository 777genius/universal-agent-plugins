package platformexec

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

func (claudeAdapter) DetectNative(root string) bool {
	return fileExists(filepath.Join(root, ".claude-plugin", "plugin.json")) ||
		fileExists(filepath.Join(root, "hooks", "hooks.json")) ||
		fileExists(filepath.Join(root, "settings.json")) ||
		fileExists(filepath.Join(root, ".lsp.json")) ||
		fileExists(filepath.Join(root, "commands")) ||
		fileExists(filepath.Join(root, "agents"))
}

func (claudeAdapter) RefineDiscovery(root string, state *pluginmodel.TargetState) error {
	if rel := state.DocPath("package_metadata"); strings.TrimSpace(rel) != "" {
		if _, ok, err := readYAMLDoc[claudePackageMeta](root, rel); err != nil {
			return fmt.Errorf("parse %s: %w", rel, err)
		} else if !ok {
			return nil
		}
	}
	for _, doc := range []struct {
		kind  string
		label string
	}{
		{kind: "settings", label: "Claude settings"},
		{kind: "lsp", label: "Claude LSP"},
		{kind: "user_config", label: "Claude userConfig"},
	} {
		if _, _, _, err := loadClaudeJSONDoc(root, state.DocPath(doc.kind), doc.label); err != nil {
			return err
		}
	}
	return nil
}

func (claudeAdapter) ManagedPaths(root string, graph pluginmodel.PackageGraph, state pluginmodel.TargetState) ([]string, error) {
	if claudeUsesGeneratedHooks(graph, state) || fileExists(filepath.Join(root, "hooks", "hooks.json")) {
		return []string{filepath.ToSlash(filepath.Join("hooks", "hooks.json"))}, nil
	}
	return nil, nil
}
