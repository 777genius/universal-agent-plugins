package platformexec

import "github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"

func (codexPackageAdapter) Generate(root string, graph pluginmodel.PackageGraph, state pluginmodel.TargetState) ([]pluginmodel.Artifact, error) {
	return generateCodexPackageArtifacts(root, graph, state)
}

func (codexRuntimeAdapter) Generate(root string, graph pluginmodel.PackageGraph, state pluginmodel.TargetState) ([]pluginmodel.Artifact, error) {
	return generateCodexRuntimeArtifacts(root, graph, state)
}
