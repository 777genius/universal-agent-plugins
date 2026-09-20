package platformexec

import (
	"path/filepath"
	"slices"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

func (geminiAdapter) ManagedPaths(root string, graph pluginmodel.PackageGraph, state pluginmodel.TargetState) ([]string, error) {
	seen := initialGeminiManagedPathSet(graph, state)
	selected, ok, err := resolveGeminiManagedContextSelection(root, graph, state)
	if err != nil {
		return sortedKeys(seen), err
	}
	return buildGeminiManagedPaths(seen, state, selected, ok), nil
}

func initialGeminiManagedPathSet(graph pluginmodel.PackageGraph, state pluginmodel.TargetState) map[string]struct{} {
	seen := map[string]struct{}{}
	if geminiUsesGeneratedHooks(graph, state) {
		seen[geminiGeneratedHooksPath()] = struct{}{}
	}
	return seen
}

func resolveGeminiManagedContextSelection(root string, graph pluginmodel.PackageGraph, state pluginmodel.TargetState) (geminiContextSelection, bool, error) {
	meta, err := loadGeminiRenderMeta(root, state)
	if err != nil {
		return geminiContextSelection{}, false, err
	}
	return selectGeminiPrimaryContext(graph, state, meta)
}

func buildGeminiManagedPaths(seen map[string]struct{}, state pluginmodel.TargetState, selected geminiContextSelection, ok bool) []string {
	if !ok {
		return sortedKeys(seen)
	}
	addGeminiManagedContextPaths(seen, state, selected)
	return sortedGeminiManagedPaths(seen)
}

func addGeminiManagedContextPaths(seen map[string]struct{}, state pluginmodel.TargetState, selected geminiContextSelection) {
	seen[selected.ArtifactName] = struct{}{}
	for _, rel := range state.ComponentPaths("contexts") {
		if rel == selected.SourcePath {
			continue
		}
		seen[geminiExtraContextArtifactPath(rel)] = struct{}{}
	}
}

func sortedGeminiManagedPaths(seen map[string]struct{}) []string {
	var out []string
	for path := range seen {
		out = append(out, path)
	}
	slices.Sort(out)
	return out
}

func geminiGeneratedHooksPath() string {
	return filepath.ToSlash(filepath.Join("hooks", "hooks.json"))
}

func geminiUsesGeneratedHooks(graph pluginmodel.PackageGraph, state pluginmodel.TargetState) bool {
	if graph.Launcher == nil || strings.TrimSpace(graph.Launcher.Entrypoint) == "" {
		return false
	}
	if len(state.ComponentPaths("hooks")) > 0 {
		return false
	}
	return slices.Equal(graph.Manifest.Targets, []string{"gemini"})
}

func geminiManifestManagedPaths() []string {
	return []string{
		"name",
		"version",
		"description",
		"mcpServers",
		"contextFileName",
		"excludeTools",
		"settings",
		"themes",
		"plan.directory",
	}
}
