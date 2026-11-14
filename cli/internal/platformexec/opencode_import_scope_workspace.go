package platformexec

func importOpenCodeWorkspaceArtifacts(state *opencodeImportedState, cfg opencodeScopeConfig) error {
	if err := importOpenCodeWorkspaceDirectoryArtifacts(state, cfg); err != nil {
		return err
	}
	return importOpenCodeWorkspaceFileArtifacts(state, cfg)
}
