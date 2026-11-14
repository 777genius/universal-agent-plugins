package platformexec

import (
	"fmt"
	"strings"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

func (geminiAdapter) Generate(root string, graph pluginmodel.PackageGraph, state pluginmodel.TargetState) ([]pluginmodel.Artifact, error) {
	entrypoint, err := geminiRenderEntrypoint(graph)
	if err != nil {
		return nil, err
	}
	meta, err := loadGeminiRenderMeta(root, state)
	if err != nil {
		return nil, err
	}
	artifacts, err := renderGeminiArtifacts(root, graph, state, meta)
	if err != nil {
		return nil, err
	}
	return appendGeminiGeneratedArtifacts(root, graph, state, entrypoint, artifacts)
}

func geminiRenderEntrypoint(graph pluginmodel.PackageGraph) (string, error) {
	if graph.Launcher == nil {
		return "", nil
	}
	entrypoint := strings.TrimSpace(graph.Launcher.Entrypoint)
	if entrypoint == "" {
		return "", fmt.Errorf("invalid %s: entrypoint required", pluginmodel.LauncherFileName)
	}
	return entrypoint, nil
}
