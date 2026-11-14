package platformexec

import "github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"

func importDirectoryArtifacts(source opencodeImportSource, dstRoot string, keep func(string) bool) ([]pluginmodel.Artifact, error) {
	artifacts, _, err := importDirectoryArtifactsWithWarnings([]opencodeImportSource{source}, dstRoot, keep)
	return artifacts, err
}

func importDirectoryArtifactsWithWarnings(sources []opencodeImportSource, dstRoot string, keep func(string) bool) ([]pluginmodel.Artifact, []pluginmodel.Warning, error) {
	artifacts := map[string]pluginmodel.Artifact{}
	warnings, err := appendImportedDirectorySources(artifacts, sources, dstRoot, keep)
	if err != nil {
		return nil, nil, err
	}
	return sortedImportedArtifacts(artifacts), warnings, nil
}
