package platformexec

import "github.com/777genius/plugin-kit-ai/cli/internal/pluginmodel"

type opencodeImportedState struct {
	plugins         []opencodePluginRef
	pluginsProvided bool
	mcp             map[string]any
	defaultAgent    string
	defaultAgentSet bool
	instructions    []string
	instructionsSet bool
	permission      any
	permissionSet   bool
	extra           map[string]any
	artifacts       map[string]pluginmodel.Artifact
	warnings        []pluginmodel.Warning
	hasInput        bool
}

type opencodeImportSource struct {
	dir       string
	display   string
	warnOnUse bool
	warnPath  string
	warnMsg   string
}

type opencodeScopeConfig struct {
	root              string
	displayConfigRoot string
	workspaceRoot     string
	workspaceDisplay  string
}

func importOpenCodePackage(root string, seed ImportSeed) (ImportResult, error) {
	state := initializeOpenCodeImportState()
	if err := runOpenCodeImportScopes(&state, root, seed); err != nil {
		return ImportResult{}, err
	}
	return finalizeOpenCodeImport(state, seed)
}
