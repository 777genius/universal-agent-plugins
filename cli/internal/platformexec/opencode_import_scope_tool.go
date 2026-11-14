package platformexec

import "github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"

type openCodeToolArtifactsImport struct {
	artifacts []pluginmodel.Artifact
	warnings  []pluginmodel.Warning
}

func resolveOpenCodeToolArtifactsImport(cfg opencodeScopeConfig) (openCodeToolArtifactsImport, error) {
	toolArtifacts, toolWarnings, err := importOpenCodeToolArtifacts(cfg.workspaceRoot, cfg.workspaceDisplay)
	if err != nil {
		return openCodeToolArtifactsImport{}, err
	}
	return openCodeToolArtifactsImport{
		artifacts: toolArtifacts,
		warnings:  toolWarnings,
	}, nil
}

func applyOpenCodeToolArtifactsImport(state *opencodeImportedState, result openCodeToolArtifactsImport) {
	state.addArtifacts(result.artifacts...)
	state.warnings = append(state.warnings, result.warnings...)
	if len(result.artifacts) > 0 {
		state.hasInput = true
	}
}
