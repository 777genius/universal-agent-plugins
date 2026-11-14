package platformexec

func importOpenCodeWorkspaceDirectoryArtifacts(state *opencodeImportedState, cfg opencodeScopeConfig) error {
	for _, step := range openCodeWorkspaceDirectorySteps(cfg) {
		if err := step(state); err != nil {
			return err
		}
	}
	return nil
}

func openCodeWorkspaceDirectorySteps(cfg opencodeScopeConfig) []func(*opencodeImportedState) error {
	return []func(*opencodeImportedState) error{
		func(state *opencodeImportedState) error { return importOpenCodeThemeArtifacts(state, cfg) },
		func(state *opencodeImportedState) error { return importOpenCodeToolArtifactsIntoState(state, cfg) },
		func(state *opencodeImportedState) error { return importOpenCodeCommandDirectory(state, cfg) },
		func(state *opencodeImportedState) error { return importOpenCodeAgentDirectory(state, cfg) },
		func(state *opencodeImportedState) error { return importOpenCodeSkillDirectory(state, cfg) },
		func(state *opencodeImportedState) error { return importOpenCodePluginDirectory(state, cfg) },
	}
}
