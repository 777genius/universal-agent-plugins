package platformexec

func importOpenCodeThemeArtifacts(state *opencodeImportedState, cfg opencodeScopeConfig) error {
	return importOpenCodeWorkspaceDirectory(state, openCodeThemeDirectoryImport(cfg))
}

func importOpenCodeToolArtifactsIntoState(state *opencodeImportedState, cfg opencodeScopeConfig) error {
	result, err := resolveOpenCodeToolArtifactsImport(cfg)
	if err != nil {
		return err
	}
	applyOpenCodeToolArtifactsImport(state, result)
	return nil
}

func importOpenCodeCommandDirectory(state *opencodeImportedState, cfg opencodeScopeConfig) error {
	return importOpenCodeWorkspaceDirectory(state, openCodeCommandDirectoryImport(cfg))
}

func importOpenCodeAgentDirectory(state *opencodeImportedState, cfg opencodeScopeConfig) error {
	return importOpenCodeWorkspaceDirectory(state, openCodeAgentDirectoryImport(cfg))
}

func importOpenCodeSkillDirectory(state *opencodeImportedState, cfg opencodeScopeConfig) error {
	return importOpenCodeWorkspaceDirectory(state, openCodeSkillDirectoryImport(cfg))
}

func importOpenCodePluginDirectory(state *opencodeImportedState, cfg opencodeScopeConfig) error {
	return importOpenCodeWorkspaceDirectory(state, openCodePluginDirectoryImport(cfg))
}

func importOpenCodePackageJSON(state *opencodeImportedState, cfg opencodeScopeConfig) error {
	artifact, ok, err := resolveOpenCodePackageJSONArtifact(cfg)
	if err != nil {
		return err
	}
	if ok {
		state.addArtifacts(artifact)
		state.hasInput = true
	}
	return nil
}
