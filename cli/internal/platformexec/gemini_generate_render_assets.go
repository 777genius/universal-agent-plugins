package platformexec

import "github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"

func mergeGeminiManifestAssets(root string, state pluginmodel.TargetState, manifest map[string]any) error {
	if err := mergeGeminiManifestSettings(root, state, manifest); err != nil {
		return err
	}
	return mergeGeminiManifestThemes(root, state, manifest)
}

func mergeGeminiManifestContexts(root string, graph pluginmodel.PackageGraph, state pluginmodel.TargetState, meta geminiPackageMeta, manifest map[string]any) ([]pluginmodel.Artifact, error) {
	contextName, artifacts, ok, err := buildGeminiContextArtifacts(root, graph, state, meta)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, nil
	}
	manifest["contextFileName"] = contextName
	return artifacts, nil
}

func mergeGeminiManifestSettings(root string, state pluginmodel.TargetState, manifest map[string]any) error {
	settings, err := loadGeminiSettings(root, state.ComponentPaths("settings"))
	if err != nil {
		return err
	}
	mergeGeminiManifestAssetSection(manifest, "settings", settings)
	return nil
}

func mergeGeminiManifestThemes(root string, state pluginmodel.TargetState, manifest map[string]any) error {
	themes, err := loadGeminiThemes(root, state.ComponentPaths("themes"))
	if err != nil {
		return err
	}
	mergeGeminiManifestAssetSection(manifest, "themes", themes)
	return nil
}

func mergeGeminiManifestAssetSection(manifest map[string]any, key string, values []map[string]any) {
	if len(values) == 0 {
		return
	}
	manifest[key] = values
}

func buildGeminiContextArtifacts(root string, graph pluginmodel.PackageGraph, state pluginmodel.TargetState, meta geminiPackageMeta) (string, []pluginmodel.Artifact, bool, error) {
	contextName, contextArtifact, extraContexts, ok, err := geminiContextArtifacts(root, graph, state, meta)
	if err != nil || !ok {
		return "", nil, ok, err
	}
	return contextName, append([]pluginmodel.Artifact{contextArtifact}, extraContexts...), true, nil
}
