package platformexec

func importOpenCodeScope(state *opencodeImportedState, cfg opencodeScopeConfig) error {
	configImport, err := resolveOpenCodeScopeConfigImport(cfg)
	if err != nil {
		return err
	}
	if err := applyOpenCodeScopeConfigImport(state, configImport); err != nil {
		return err
	}
	if err := importOpenCodeWorkspaceArtifacts(state, cfg); err != nil {
		return err
	}
	return nil
}
