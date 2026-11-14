package platformexec

import "github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"

func (s *opencodeImportedState) addArtifacts(artifacts ...pluginmodel.Artifact) {
	for _, artifact := range artifacts {
		s.artifacts[artifact.RelPath] = artifact
	}
}

func (s *opencodeImportedState) mergeConfig(config importedOpenCodeConfig) {
	if config.PluginsProvided {
		s.pluginsProvided = true
		s.plugins = append([]opencodePluginRef(nil), config.Plugins...)
	}
	if config.MCPProvided {
		if s.mcp == nil {
			s.mcp = map[string]any{}
		}
		mergeOpenCodeObject(s.mcp, config.MCP)
	}
	if len(config.Extra) > 0 {
		if s.extra == nil {
			s.extra = map[string]any{}
		}
		mergeOpenCodeObject(s.extra, config.Extra)
	}
	if config.DefaultAgentSet {
		s.defaultAgent = config.DefaultAgent
		s.defaultAgentSet = true
	}
	if config.InstructionsSet {
		s.instructions = append([]string(nil), config.Instructions...)
		s.instructionsSet = true
	}
	if config.PermissionSet {
		s.permission = config.Permission
		s.permissionSet = true
	}
}
