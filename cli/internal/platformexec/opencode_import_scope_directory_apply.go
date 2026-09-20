package platformexec

func importOpenCodeWorkspaceDirectory(state *opencodeImportedState, spec openCodeWorkspaceDirectoryImport) error {
	artifacts, err := importDirectoryArtifacts(spec.source, spec.dstRoot, spec.keep)
	if err != nil {
		return err
	}
	state.addArtifacts(artifacts...)
	if len(artifacts) > 0 {
		state.hasInput = true
	}
	return nil
}
