package platformexec

import "github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"

func renderGeminiArtifacts(root string, graph pluginmodel.PackageGraph, state pluginmodel.TargetState, meta geminiPackageMeta) ([]pluginmodel.Artifact, error) {
	if err := validateGeminiRenderReady(root, graph, state, meta); err != nil {
		return nil, err
	}
	manifest, artifacts, err := renderGeminiManifest(root, graph, state, meta)
	if err != nil {
		return nil, err
	}
	return appendGeminiManifestArtifact(artifacts, manifest)
}

func appendGeminiManifestArtifact(artifacts []pluginmodel.Artifact, manifest map[string]any) ([]pluginmodel.Artifact, error) {
	manifestJSON, err := marshalJSON(manifest)
	if err != nil {
		return nil, err
	}
	return append(artifacts, pluginmodel.Artifact{RelPath: "gemini-extension.json", Content: manifestJSON}), nil
}
