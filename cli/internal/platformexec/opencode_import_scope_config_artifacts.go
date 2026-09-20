package platformexec

import "github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"

type openCodeConfigImportArtifacts struct {
	config         importedOpenCodeConfig
	commandWarns   []pluginmodel.Warning
	agentWarns     []pluginmodel.Warning
	commandPayload []pluginmodel.Artifact
	agentPayload   []pluginmodel.Artifact
}

func resolveOpenCodeConfigImportArtifacts(importedConfig importedOpenCodeConfig, configDisplayPath string) (openCodeConfigImportArtifacts, error) {
	commandArtifacts, remainingCommands, commandWarnings, err := importedOpenCodeInlineCommandArtifacts(importedConfig.Commands, configDisplayPath)
	if err != nil {
		return openCodeConfigImportArtifacts{}, err
	}
	agentArtifacts, remainingAgents, agentWarnings, err := importedOpenCodeInlineAgentArtifacts(importedConfig.Agents, configDisplayPath)
	if err != nil {
		return openCodeConfigImportArtifacts{}, err
	}
	mergeRemainingOpenCodeConfigFields(&importedConfig, remainingCommands, remainingAgents)
	return openCodeConfigImportArtifacts{
		config:         importedConfig,
		commandWarns:   commandWarnings,
		agentWarns:     agentWarnings,
		commandPayload: commandArtifacts,
		agentPayload:   agentArtifacts,
	}, nil
}

func applyOpenCodeConfigImportArtifacts(state *opencodeImportedState, result openCodeConfigImportArtifacts) {
	state.warnings = append(state.warnings, result.commandWarns...)
	state.warnings = append(state.warnings, result.agentWarns...)
	state.addArtifacts(result.commandPayload...)
	state.addArtifacts(result.agentPayload...)
	state.mergeConfig(result.config)
}

func mergeRemainingOpenCodeConfigFields(importedConfig *importedOpenCodeConfig, remainingCommands, remainingAgents map[string]any) {
	if len(remainingCommands) > 0 {
		ensureOpenCodeConfigExtra(importedConfig)["command"] = remainingCommands
	}
	if len(remainingAgents) > 0 {
		ensureOpenCodeConfigExtra(importedConfig)["agent"] = remainingAgents
	}
}

func ensureOpenCodeConfigExtra(importedConfig *importedOpenCodeConfig) map[string]any {
	if importedConfig.Extra == nil {
		importedConfig.Extra = map[string]any{}
	}
	return importedConfig.Extra
}
