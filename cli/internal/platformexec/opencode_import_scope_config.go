package platformexec

func importOpenCodeConfigArtifacts(state *opencodeImportedState, importedConfig importedOpenCodeConfig, configDisplayPath string) error {
	result, err := resolveOpenCodeConfigImportArtifacts(importedConfig, configDisplayPath)
	if err != nil {
		return err
	}
	applyOpenCodeConfigImportArtifacts(state, result)
	return nil
}
