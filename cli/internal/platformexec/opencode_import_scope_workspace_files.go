package platformexec

func importOpenCodeWorkspaceFileArtifacts(state *opencodeImportedState, cfg opencodeScopeConfig) error {
	return importOpenCodePackageJSON(state, cfg)
}
