package platformexec

import (
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

func renderGeminiManifest(root string, graph pluginmodel.PackageGraph, state pluginmodel.TargetState, meta geminiPackageMeta) (map[string]any, []pluginmodel.Artifact, error) {
	manifest := buildGeminiManifestBase(graph)
	if err := mergeGeminiManifestPortableSections(manifest, graph); err != nil {
		return nil, nil, err
	}
	mergeGeminiManifestMeta(manifest, meta)
	contextArtifacts, err := mergeGeminiManifestRuntimeSections(root, graph, state, meta, manifest)
	if err != nil {
		return nil, nil, err
	}
	if err := mergeGeminiManifestNativeDocs(root, state, manifest); err != nil {
		return nil, nil, err
	}
	return manifest, contextArtifacts, nil
}

func buildGeminiManifestBase(graph pluginmodel.PackageGraph) map[string]any {
	return map[string]any{
		"name":        graph.Manifest.Name,
		"version":     graph.Manifest.Version,
		"description": graph.Manifest.Description,
	}
}

func mergeGeminiManifestPortableSections(manifest map[string]any, graph pluginmodel.PackageGraph) error {
	return mergeGeminiManifestPortableMCP(manifest, graph)
}

func mergeGeminiManifestPortableMCP(manifest map[string]any, graph pluginmodel.PackageGraph) error {
	if graph.Portable.MCP == nil {
		return nil
	}
	projected, err := renderPortableMCPForTarget(graph.Portable.MCP, "gemini")
	if err != nil {
		return err
	}
	manifest["mcpServers"] = projected
	return nil
}

func mergeGeminiManifestMeta(manifest map[string]any, meta geminiPackageMeta) {
	mergeGeminiManifestExcludeTools(manifest, meta)
	mergeGeminiManifestMigratedTo(manifest, meta)
	mergeGeminiManifestPlan(manifest, meta)
}

func mergeGeminiManifestExcludeTools(manifest map[string]any, meta geminiPackageMeta) {
	if len(meta.ExcludeTools) == 0 {
		return
	}
	manifest["excludeTools"] = append([]string(nil), normalizeGeminiExcludeTools(meta.ExcludeTools)...)
}

func mergeGeminiManifestMigratedTo(manifest map[string]any, meta geminiPackageMeta) {
	if strings.TrimSpace(meta.MigratedTo) == "" {
		return
	}
	manifest["migratedTo"] = meta.MigratedTo
}

func mergeGeminiManifestPlan(manifest map[string]any, meta geminiPackageMeta) {
	if strings.TrimSpace(meta.PlanDirectory) == "" {
		return
	}
	manifest["plan"] = map[string]any{"directory": meta.PlanDirectory}
}

func mergeGeminiManifestRuntimeSections(root string, graph pluginmodel.PackageGraph, state pluginmodel.TargetState, meta geminiPackageMeta, manifest map[string]any) ([]pluginmodel.Artifact, error) {
	if err := mergeGeminiManifestAssets(root, state, manifest); err != nil {
		return nil, err
	}
	return mergeGeminiManifestContexts(root, graph, state, meta, manifest)
}

func mergeGeminiManifestNativeDocs(root string, state pluginmodel.TargetState, manifest map[string]any) error {
	return mergeGeminiManifestExtraDoc(root, state, manifest)
}

func mergeGeminiManifestExtraDoc(root string, state pluginmodel.TargetState, manifest map[string]any) error {
	extra, err := loadNativeExtraDoc(root, state, "manifest_extra", pluginmodel.NativeDocFormatJSON)
	if err != nil {
		return err
	}
	return pluginmodel.MergeNativeExtraObject(manifest, extra, "gemini manifest.extra.json", geminiManifestManagedPaths())
}
