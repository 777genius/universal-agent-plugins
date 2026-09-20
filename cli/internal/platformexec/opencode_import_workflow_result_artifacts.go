package platformexec

import (
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

func buildOpenCodeBaseImportArtifacts(state opencodeImportedState) []pluginmodel.Artifact {
	return []pluginmodel.Artifact{{
		RelPath: filepath.Join(pluginmodel.SourceDirName, "targets", "opencode", "package.yaml"),
		Content: mustYAML(opencodePackageMeta{Plugins: append([]opencodePluginRef(nil), state.plugins...)}),
	}}
}

func appendOpenCodeImportedArtifacts(artifacts []pluginmodel.Artifact, state opencodeImportedState) []pluginmodel.Artifact {
	for _, rel := range sortedArtifactKeys(state.artifacts) {
		artifacts = append(artifacts, state.artifacts[rel])
	}
	return artifacts
}
