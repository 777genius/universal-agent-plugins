package platformexec

import "github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"

func (opencodeAdapter) Generate(root string, graph pluginmodel.PackageGraph, state pluginmodel.TargetState) ([]pluginmodel.Artifact, error) {
	doc, err := buildOpenCodeConfig(root, graph, state)
	if err != nil {
		return nil, err
	}
	body, err := marshalJSON(doc)
	if err != nil {
		return nil, err
	}

	artifacts := []pluginmodel.Artifact{{
		RelPath: "opencode.json",
		Content: body,
	}}
	artifacts, err = appendOpenCodeArtifacts(root, graph, state, artifacts)
	if err != nil {
		return nil, err
	}
	return artifacts, nil
}

func (opencodeAdapter) ManagedPaths(root string, graph pluginmodel.PackageGraph, state pluginmodel.TargetState) ([]string, error) {
	return nil, nil
}
