package platformexec

import "github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"

func sortedImportedArtifacts(artifacts map[string]pluginmodel.Artifact) []pluginmodel.Artifact {
	out := make([]pluginmodel.Artifact, 0, len(artifacts))
	for _, rel := range sortedArtifactKeys(artifacts) {
		out = append(out, artifacts[rel])
	}
	return out
}
