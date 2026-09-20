package platformexec

import (
	"path/filepath"

	"github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"
)

func appendOpenCodeOptionalImportArtifacts(artifacts []pluginmodel.Artifact, state opencodeImportedState) ([]pluginmodel.Artifact, error) {
	if len(state.mcp) > 0 {
		artifact, err := importedPortableMCPArtifact("opencode", state.mcp)
		if err != nil {
			return nil, err
		}
		artifacts = append(artifacts, artifact)
	}
	if state.defaultAgentSet {
		artifacts = append(artifacts, pluginmodel.Artifact{
			RelPath: filepath.Join(pluginmodel.SourceDirName, "targets", "opencode", "default_agent.txt"),
			Content: append([]byte(state.defaultAgent), '\n'),
		})
	}
	if state.instructionsSet {
		artifacts = append(artifacts, pluginmodel.Artifact{
			RelPath: filepath.Join(pluginmodel.SourceDirName, "targets", "opencode", "instructions.yaml"),
			Content: mustYAML(state.instructions),
		})
	}
	if state.permissionSet {
		body, err := marshalJSON(state.permission)
		if err != nil {
			return nil, err
		}
		artifacts = append(artifacts, pluginmodel.Artifact{
			RelPath: filepath.Join(pluginmodel.SourceDirName, "targets", "opencode", "permission.json"),
			Content: body,
		})
	}
	if len(state.extra) > 0 {
		artifacts = append(artifacts, pluginmodel.Artifact{
			RelPath: filepath.Join(pluginmodel.SourceDirName, "targets", "opencode", "config.extra.json"),
			Content: mustJSON(state.extra),
		})
	}
	return artifacts, nil
}
